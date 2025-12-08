package constants

// Platform service ports - centralized port definitions for consistency
const (
	// Core Infrastructure
	NomadPort = 4646
	VaultPort = 8200

	// Ingress & Routing
	TraefikHTTPPort      = 4200
	TraefikDashboardPort = 4201
	TraefikMetricsPort   = 8082

	// Messaging & Cache
	NATSClientPort  = 4222
	NATSClusterPort = 6222
	NATSMonitorPort = 8222
	DragonflyPort   = 6379

	// Observability
	PrometheusPort = 9090
	LokiPort       = 3100
	GrafanaPort    = 3000
	PromtailPort   = 9080

	// Data Services (Phase 9)
	PostgresPort      = 5432
	RustFSAPIPort     = 9000
	RustFSConsolePort = 9001
)
