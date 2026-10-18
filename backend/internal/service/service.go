package service

// Service constants
const (
	ServiceName        = "twsnmpneo"
	ServiceDisplayName = "TWSNMP NEO Service"
	ServiceDescription = "TWSNMP NEO Network Management Service"
)

// InstallOptions holds options for installing the Windows service
type InstallOptions struct {
	AutoStart    bool     // If true, start automatically on system boot
	DelayedStart bool     // If true, start with delayed automatic start
	Arguments    []string // Extra CLI flags/arguments to pass to the service binary
}

// ServiceHandler defines the interface for running the application under the service manager
type ServiceHandler interface {
	Start() error
	Stop() error
}
