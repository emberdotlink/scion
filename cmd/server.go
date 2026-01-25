package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ptone/scion-agent/pkg/config"
	"github.com/ptone/scion-agent/pkg/hub"
	"github.com/ptone/scion-agent/pkg/store"
	"github.com/ptone/scion-agent/pkg/store/sqlite"
	"github.com/spf13/cobra"
)

var (
	serverConfigPath string
	hubPort          int
	hubHost          string
	enableHub        bool
	dbURL            string
)

// serverCmd represents the server command
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Manage the Scion Hub API server",
	Long: `Commands for managing the Scion Hub API server.

The server provides:
- Hub API: Central registry for groves, agents, and templates
- Web Frontend: Browser-based UI (coming soon)
- Runtime Host API: Agent lifecycle management (coming soon)`,
}

// serverStartCmd represents the server start command
var serverStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the Scion Hub API server",
	Long: `Start the Scion Hub API server.

The Hub API provides central coordination for:
- Grove registration and management
- Agent lifecycle tracking
- Template registry
- Runtime host coordination

Configuration can be provided via:
- Config file (--config flag or ~/.scion/server.yaml)
- Environment variables (SCION_SERVER_* prefix)
- Command-line flags

Example:
  scion server start
  scion server start --port 9810 --host 0.0.0.0
  scion server start --config ./server.yaml`,
	RunE: runServerStart,
}

func runServerStart(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.LoadGlobalConfig(serverConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Override with command-line flags if specified
	if cmd.Flags().Changed("port") {
		cfg.Hub.Port = hubPort
	}
	if cmd.Flags().Changed("host") {
		cfg.Hub.Host = hubHost
	}
	if cmd.Flags().Changed("db") {
		cfg.Database.URL = dbURL
	}

	// Initialize store
	var s store.Store

	switch cfg.Database.Driver {
	case "sqlite":
		sqliteStore, err := sqlite.New(cfg.Database.URL)
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		s = sqliteStore
		defer s.Close()

		// Run migrations
		if err := s.Migrate(context.Background()); err != nil {
			return fmt.Errorf("failed to run migrations: %w", err)
		}
	default:
		return fmt.Errorf("unsupported database driver: %s", cfg.Database.Driver)
	}

	// Verify database connectivity
	if err := s.Ping(context.Background()); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	// Create Hub server configuration
	hubCfg := hub.ServerConfig{
		Port:               cfg.Hub.Port,
		Host:               cfg.Hub.Host,
		ReadTimeout:        cfg.Hub.ReadTimeout,
		WriteTimeout:       cfg.Hub.WriteTimeout,
		CORSEnabled:        cfg.Hub.CORSEnabled,
		CORSAllowedOrigins: cfg.Hub.CORSAllowedOrigins,
		CORSAllowedMethods: cfg.Hub.CORSAllowedMethods,
		CORSAllowedHeaders: cfg.Hub.CORSAllowedHeaders,
		CORSMaxAge:         cfg.Hub.CORSMaxAge,
	}

	// Create and start Hub server
	srv := hub.New(hubCfg, s)

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		log.Printf("Received signal %v, shutting down...", sig)
		cancel()
	}()

	log.Printf("Starting Hub API server on %s:%d", cfg.Hub.Host, cfg.Hub.Port)
	log.Printf("Database: %s (%s)", cfg.Database.Driver, cfg.Database.URL)

	return srv.Start(ctx)
}

func init() {
	rootCmd.AddCommand(serverCmd)
	serverCmd.AddCommand(serverStartCmd)

	// Server start flags
	serverStartCmd.Flags().StringVarP(&serverConfigPath, "config", "c", "", "Path to server configuration file")
	serverStartCmd.Flags().IntVar(&hubPort, "port", 9810, "Hub API port")
	serverStartCmd.Flags().StringVar(&hubHost, "host", "0.0.0.0", "Hub API host to bind")
	serverStartCmd.Flags().BoolVar(&enableHub, "enable-hub", true, "Enable the Hub API")
	serverStartCmd.Flags().StringVar(&dbURL, "db", "", "Database URL/path")
}
