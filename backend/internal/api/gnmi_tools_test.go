package api

import (
	"reflect"
	"testing"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
)

func TestGNMITarget(t *testing.T) {
	tests := []struct {
		name string
		node datastore.NodeEnt
		want string
	}{
		{name: "default port", node: datastore.NodeEnt{IP: "192.0.2.1"}, want: "192.0.2.1:57400"},
		{name: "configured port", node: datastore.NodeEnt{IP: "192.0.2.1", GNMIPort: "50051"}, want: "192.0.2.1:50051"},
		{name: "IPv6", node: datastore.NodeEnt{IP: "2001:db8::1"}, want: "[2001:db8::1]:57400"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := gnmiTarget(&tt.node); got != tt.want {
				t.Errorf("gnmiTarget() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFlattenGNMIValue(t *testing.T) {
	got := flattenGNMIValue(map[string]any{
		"enabled": true,
		"ports":   []any{"up", "down"},
	}, "/interfaces", "")
	want := []gnmiValue{
		{Path: "/interfaces/enabled", Value: "true"},
		{Path: "/interfaces/ports", Value: "up", Index: "0"},
		{Path: "/interfaces/ports", Value: "down", Index: "1"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("flattenGNMIValue() = %#v, want %#v", got, want)
	}
}
