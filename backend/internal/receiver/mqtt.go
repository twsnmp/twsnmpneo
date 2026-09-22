package receiver

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/parquet"
)

// MQTTConfig holds configuration for the embedded MQTT broker.
type MQTTConfig struct {
	Port         int
	LogStore     *parquet.Store
	MqttToSyslog bool
}

// MQTTServer is an embedded MQTT message broker for IoT sensor data.
type MQTTServer struct {
	port         int
	logStore     *parquet.Store
	mqttToSyslog bool
	listener     net.Listener
	mu           sync.Mutex
}

// NewMQTTServer creates an instance of the embedded MQTT broker.
func NewMQTTServer(cfg MQTTConfig) *MQTTServer {
	return &MQTTServer{
		port:         cfg.Port,
		logStore:     cfg.LogStore,
		mqttToSyslog: cfg.MqttToSyslog,
	}
}

// Start launches the MQTT broker TCP listener and blocks until ctx is canceled.
func (s *MQTTServer) Start(ctx context.Context) error {
	if s.port <= 0 {
		return nil
	}

	addr := fmt.Sprintf(":%d", s.port)
	slog.Info("Starting MQTT broker", "addr", addr)

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		slog.Warn("Failed to start MQTT broker", "addr", addr, "error", err)
		return fmt.Errorf("listen mqtt: %w", err)
	}
	s.mu.Lock()
	s.listener = ln
	s.mu.Unlock()
	defer ln.Close()

	slog.Info("Started MQTT broker", "addr", addr)

	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			continue
		}
		go s.handleClient(ctx, conn)
	}
}

func (s *MQTTServer) handleClient(ctx context.Context, conn net.Conn) {
	defer conn.Close()

	srcIP, _, _ := net.SplitHostPort(conn.RemoteAddr().String())
	if srcIP == "" {
		srcIP = conn.RemoteAddr().String()
	}

	reader := bufio.NewReader(conn)
	for {
		if ctx.Err() != nil {
			return
		}

		// Read Fixed Header: Packet Type & Flags
		headerByte, err := reader.ReadByte()
		if err != nil {
			return
		}

		packetType := headerByte >> 4
		flags := headerByte & 0x0F

		// Read Remaining Length
		remLen, err := readRemainingLength(reader)
		if err != nil {
			return
		}

		// Read Remaining Bytes
		payload := make([]byte, remLen)
		if _, err := io.ReadFull(reader, payload); err != nil {
			return
		}

		switch packetType {
		case 1: // CONNECT
			// Respond with CONNACK: Return Code 0 (Connection Accepted)
			connack := []byte{0x20, 0x02, 0x00, 0x00}
			_, _ = conn.Write(connack)

		case 3: // PUBLISH
			// Decode Topic Name
			if len(payload) >= 2 {
				topicLen := int(binary.BigEndian.Uint16(payload[0:2]))
				if len(payload) >= 2+topicLen {
					topic := string(payload[2 : 2+topicLen])
					offset := 2 + topicLen

					qos := (flags >> 1) & 0x03
					var packetID uint16
					if qos > 0 && len(payload) >= offset+2 {
						packetID = binary.BigEndian.Uint16(payload[offset : offset+2])
						offset += 2
					}

					msgContent := string(payload[offset:])
					logText := fmt.Sprintf("[%s] %s", topic, msgContent)

					if s.logStore != nil {
						_ = s.logStore.WriteLog(&parquet.ParquetLogRecord{
							Time: time.Now().UnixNano(),
							Type: "mqtt",
							Src:  srcIP,
							Log:  logText,
						})
					}

					// If QoS 1, send PUBACK
					if qos == 1 {
						puback := []byte{0x40, 0x02, byte(packetID >> 8), byte(packetID & 0xFF)}
						_, _ = conn.Write(puback)
					}
				}
			}

		case 8: // SUBSCRIBE
			if len(payload) >= 2 {
				packetID := binary.BigEndian.Uint16(payload[0:2])
				// Send SUBACK with success return code 0
				suback := []byte{0x90, 0x03, byte(packetID >> 8), byte(packetID & 0xFF), 0x00}
				_, _ = conn.Write(suback)
			}

		case 12: // PINGREQ
			// Send PINGRESP
			pingresp := []byte{0xD0, 0x00}
			_, _ = conn.Write(pingresp)

		case 14: // DISCONNECT
			return

		default:
			// Ignore other packet types
		}
	}
}

func readRemainingLength(r io.Reader) (int, error) {
	multiplier := 1
	value := 0
	buf := make([]byte, 1)
	for {
		if _, err := io.ReadFull(r, buf); err != nil {
			return 0, err
		}
		digit := buf[0]
		value += int(digit&127) * multiplier
		multiplier *= 128
		if (digit & 128) == 0 {
			break
		}
		if multiplier > 128*128*128 {
			return 0, fmt.Errorf("malformed remaining length")
		}
	}
	return value, nil
}
