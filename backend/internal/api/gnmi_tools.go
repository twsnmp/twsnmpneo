package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	gnmiapi "github.com/openconfig/gnmic/pkg/api"
	gnmitarget "github.com/openconfig/gnmic/pkg/api/target"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
)

var errGNMINodeNotFound = errors.New("node not found")

type gnmiModel struct {
	Name         string `json:"name"`
	Organization string `json:"organization"`
	Version      string `json:"version"`
}

type gnmiValue struct {
	Path  string `json:"Path"`
	Value string `json:"Value"`
	Index string `json:"Index,omitempty"`
}

func registerGNMIToolRoutes(apiGroup *echo.Group, store datastore.DataStore) {
	apiGroup.POST("/tools/gnmi/capabilities", func(c echo.Context) error {
		node, target, err := bindGNMITarget(c, store)
		if err != nil {
			return writeGNMIError(c, err)
		}
		ctx, cancel := context.WithTimeout(c.Request().Context(), 30*time.Second)
		defer cancel()
		client, err := newGNMIClient(ctx, node, target)
		if err != nil {
			return c.JSON(http.StatusBadGateway, map[string]string{"error": err.Error()})
		}
		defer client.Close()

		capabilities, err := client.Capabilities(ctx)
		if err != nil {
			return c.JSON(http.StatusBadGateway, map[string]string{"error": err.Error()})
		}
		models := make([]gnmiModel, 0, len(capabilities.GetSupportedModels()))
		for _, model := range capabilities.GetSupportedModels() {
			models = append(models, gnmiModel{
				Name: model.Name, Organization: model.Organization, Version: model.Version,
			})
		}
		encodings := make([]string, 0, len(capabilities.GetSupportedEncodings()))
		for _, encoding := range capabilities.GetSupportedEncodings() {
			encodings = append(encodings, encoding.String())
		}
		return c.JSON(http.StatusOK, map[string]any{
			"version":   capabilities.GetGNMIVersion(),
			"models":    models,
			"encodings": strings.Join(encodings, ", "),
		})
	})

	apiGroup.POST("/tools/gnmi/get", func(c echo.Context) error {
		var req struct {
			NodeID   string `json:"node_id"`
			Target   string `json:"target"`
			Path     string `json:"path"`
			Encoding string `json:"encoding"`
		}
		if err := c.Bind(&req); err != nil || req.NodeID == "" || strings.TrimSpace(req.Path) == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "node_id and path are required"})
		}
		node, err := store.GetNode(c.Request().Context(), req.NodeID)
		if err != nil || node == nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "node not found"})
		}
		target := strings.TrimSpace(req.Target)
		if target == "" {
			target = gnmiTarget(node)
		}
		ctx, cancel := context.WithTimeout(c.Request().Context(), 30*time.Second)
		defer cancel()
		client, err := newGNMIClient(ctx, node, target)
		if err != nil {
			return c.JSON(http.StatusBadGateway, map[string]string{"error": err.Error()})
		}
		defer client.Close()

		encoding := req.Encoding
		if encoding == "" {
			encoding = node.GNMIEncoding
		}
		if encoding == "" {
			encoding = "json_ietf"
		}
		getRequest, err := gnmiapi.NewGetRequest(gnmiapi.Path(req.Path), gnmiapi.Encoding(encoding))
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		response, err := client.Get(ctx, getRequest)
		if err != nil {
			return c.JSON(http.StatusBadGateway, map[string]string{"error": err.Error()})
		}
		results := make([]gnmiValue, 0)
		for _, notification := range response.GetNotification() {
			for _, update := range notification.GetUpdate() {
				var value any
				if raw := update.GetVal().GetJsonIetfVal(); len(raw) > 0 {
					if err := json.Unmarshal(raw, &value); err != nil {
						value = string(raw)
					}
				} else if raw := update.GetVal().GetJsonVal(); len(raw) > 0 {
					if err := json.Unmarshal(raw, &value); err != nil {
						value = string(raw)
					}
				} else {
					value = update.GetVal().String()
				}
				path := make([]string, 0, len(update.GetPath().GetElem()))
				for _, element := range update.GetPath().GetElem() {
					path = append(path, element.GetName())
				}
				results = append(results, flattenGNMIValue(value, "/"+strings.Join(path, "/"), "")...)
			}
		}
		return c.JSON(http.StatusOK, results)
	})
}

func bindGNMITarget(c echo.Context, store datastore.DataStore) (*datastore.NodeEnt, string, error) {
	var req struct {
		NodeID string `json:"node_id"`
		Target string `json:"target"`
	}
	if err := c.Bind(&req); err != nil || req.NodeID == "" {
		return nil, "", errors.New("node_id is required")
	}
	node, err := store.GetNode(c.Request().Context(), req.NodeID)
	if err != nil || node == nil {
		return nil, "", errGNMINodeNotFound
	}
	target := strings.TrimSpace(req.Target)
	if target == "" {
		target = gnmiTarget(node)
	}
	return node, target, nil
}

func writeGNMIError(c echo.Context, err error) error {
	if errors.Is(err, errGNMINodeNotFound) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
}

func gnmiTarget(node *datastore.NodeEnt) string {
	port := node.GNMIPort
	if port == "" {
		port = "57400"
	}
	return net.JoinHostPort(node.IP, port)
}

func newGNMIClient(ctx context.Context, node *datastore.NodeEnt, target string) (*gnmitarget.Target, error) {
	if node == nil {
		return nil, errGNMINodeNotFound
	}
	if strings.TrimSpace(node.IP) == "" {
		return nil, errors.New("node has no IP address")
	}
	client, err := gnmiapi.NewTarget(
		gnmiapi.Name(node.Name),
		gnmiapi.Address(target),
		gnmiapi.Username(node.GNMIUser),
		gnmiapi.Password(node.GNMIPassword),
		gnmiapi.SkipVerify(true),
	)
	if err != nil {
		return nil, err
	}
	if err := client.CreateGNMIClient(ctx); err != nil {
		_ = client.Close()
		return nil, err
	}
	return client, nil
}

func flattenGNMIValue(value any, path, index string) []gnmiValue {
	switch v := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		results := make([]gnmiValue, 0)
		for _, key := range keys {
			results = append(results, flattenGNMIValue(v[key], path+"/"+key, index)...)
		}
		return results
	case []any:
		results := make([]gnmiValue, 0)
		for i, item := range v {
			results = append(results, flattenGNMIValue(item, path, fmt.Sprintf("%d", i))...)
		}
		return results
	case nil:
		return []gnmiValue{{Path: path, Value: "null", Index: index}}
	default:
		return []gnmiValue{{Path: path, Value: fmt.Sprint(v), Index: index}}
	}
}
