package receiver

import (
	"context"
	"sync"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/i18n"
)

// ReceiverStatus holds the runtime health of a protocol receiver server.
type ReceiverStatus struct {
	Status string `json:"status"` // "running", "error", "disabled", "stopped"
	Port   string `json:"port"`
	Error  string `json:"error,omitempty"`
	Range  string `json:"range,omitempty"`
}

var (
	receiverMu       sync.RWMutex
	receiverStatuses = make(map[string]ReceiverStatus)
)

// SetReceiverStatus sets runtime status of a receiver.
func SetReceiverStatus(name string, st ReceiverStatus) {
	receiverMu.Lock()
	defer receiverMu.Unlock()
	receiverStatuses[name] = st
}

// GetReceiverStatus gets runtime status of a receiver.
func GetReceiverStatus(name string) (ReceiverStatus, bool) {
	receiverMu.RLock()
	defer receiverMu.RUnlock()
	st, ok := receiverStatuses[name]
	return st, ok
}

// ClearReceiverStatus removes or resets status of a receiver.
func ClearReceiverStatus(name string) {
	receiverMu.Lock()
	defer receiverMu.Unlock()
	delete(receiverStatuses, name)
}

// ResetAllReceiverStatuses clears all receiver statuses (useful for testing).
func ResetAllReceiverStatuses() {
	receiverMu.Lock()
	defer receiverMu.Unlock()
	receiverStatuses = make(map[string]ReceiverStatus)
}

// logReceiverWarn logs a warning event in the datastore event log.
func logReceiverWarn(store datastore.DataStore, event string) {
	if store != nil {
		_ = store.AddEventLog(context.Background(), &datastore.EventLogEnt{
			Time:  time.Now().UnixNano(),
			Type:  "system",
			Level: "warn",
			Event: i18n.Trans(event),
		})
	}
}
