package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"sg-emulator/internal/ca"
	"sg-emulator/internal/server"
	"sg-emulator/internal/transport/grpc"
	"sg-emulator/internal/transport/mcp"
	"sg-emulator/internal/transport/rest"
	"sg-emulator/internal/transport/tui"
)

func main() {
	useTUI := flag.Bool("tui", false, "Run with TUI interface")
	numRestApps := flag.Int("rest", 0, "Number of virtual app instances with REST transport")
	numGrpcApps := flag.Int("grpc", 0, "Number of virtual app instances with gRPC transport")
	mcpAddr := flag.String("mcp", "", "MCP server HTTP address (e.g., localhost:3000)")
	logLevel := flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	logFormat := flag.String("log-format", "text", "Log format (text, json)")
	logFile := flag.String("log-file", "", "Log file path (auto-set for TUI mode)")
	flag.Parse()

	// Setup logging based on mode
	if *useTUI {
		// When TUI is active, ALWAYS log to file to avoid conflicts
		if *logFile == "" {
			*logFile = "/tmp/sg-emulator.log"
		}
		setupFileLogging(*logFile, *logLevel, *logFormat)
		fmt.Fprintf(os.Stderr, "TUI mode: Logs written to %s\n", *logFile)
	} else {
		// Headless mode: log to stdout
		setupStdoutLogging(*logLevel, *logFormat)
	}

	slog.Info("Starting SG Emulator",
		slog.Group("config",
			"rest_apps", *numRestApps,
			"grpc_apps", *numGrpcApps,
			"mcp_enabled", *mcpAddr != "",
			"mcp_address", *mcpAddr,
			"tui_enabled", *useTUI,
			"log_level", *logLevel,
			"log_format", *logFormat,
		),
	)

	// Create context for MCP server
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create logger hierarchy
	rootLogger := slog.Default().With("app", "sg-emulator")
	serverLogger := rootLogger.With("component", "server")

	// Get current working directory for CA storage
	cwd, err := os.Getwd()
	if err != nil {
		slog.Error("Failed to get current directory", "error", err)
		os.Exit(1)
	}

	// Initialize Certificate Authority
	caLogger := rootLogger.With("component", "ca")
	certAuth, err := ca.New(cwd, caLogger)
	if err != nil {
		slog.Error("Failed to initialize Certificate Authority", "error", err)
		os.Exit(1)
	}
	slog.Info("Certificate Authority initialized")

	// Create and start the server with CA (runs in its own goroutine)
	srv := server.NewWithCA(serverLogger, certAuth)
	srv.Start()
	defer func() {
		slog.Info("Stopping server")
		srv.Stop()
	}()

	// Create REST virtual app instances
	if *numRestApps > 0 {
		slog.Info("Creating REST virtual app instances", "count", *numRestApps)
		for i := 0; i < *numRestApps; i++ {
			vapp, err := srv.CreateVirtualApp()
			if err != nil {
				slog.Error("Failed to create virtual app",
					"error", err,
					"index", i,
					"transport", "rest",
				)
				continue
			}

			restAddr := fmt.Sprintf("localhost:%d", 8080+i)
			restLogger := rootLogger.With("component", "rest", "index", i, "address", restAddr)
			vapp.AddTransport(rest.New(restAddr, vapp.Client(), restLogger))
			vapp.Start()

			slog.Info("Created REST virtual app",
				"id", vapp.ID(),
				"address", restAddr,
				"index", i,
			)
		}
	}

	// Create gRPC virtual app instances
	if *numGrpcApps > 0 {
		slog.Info("Creating gRPC virtual app instances", "count", *numGrpcApps)
		for i := 0; i < *numGrpcApps; i++ {
			vapp, err := srv.CreateVirtualApp()
			if err != nil {
				slog.Error("Failed to create virtual app",
					"error", err,
					"index", i,
					"transport", "grpc",
				)
				continue
			}

			grpcAddr := fmt.Sprintf("localhost:%d", 50051+i)
			grpcLogger := rootLogger.With("component", "grpc", "index", i, "address", grpcAddr)
			vapp.AddTransport(grpc.New(grpcAddr, vapp.Client(), grpcLogger))
			vapp.Start()

			slog.Info("Created gRPC virtual app",
				"id", vapp.ID(),
				"address", grpcAddr,
				"index", i,
			)
		}
	}

	// Start MCP server if address is provided
	if *mcpAddr != "" {
		slog.Info("Starting MCP server", "address", *mcpAddr)
		mcpLogger := rootLogger.With("component", "mcp", "address", *mcpAddr)
		client := server.NewClient(srv.RequestChannel(), mcpLogger.With("role", "mcp-client"))

		go func() {
			if err := mcp.RunHTTPServer(ctx, *mcpAddr, client, srv, mcpLogger); err != nil {
				slog.Error("MCP server error", "error", err, "address", *mcpAddr)
			}
		}()
	}

	if *useTUI {
		// Create TUI virtual app
		vapp, err := srv.CreateVirtualApp()
		if err != nil {
			slog.Error("Failed to create TUI virtual app", "error", err)
			os.Exit(1)
		}

		tuiLogger := rootLogger.With("component", "tui")
		client := server.NewClient(srv.RequestChannel(), tuiLogger.With("role", "tui-client"))
		vapp.AddTransport(tui.New(client, srv, tuiLogger))

		slog.Info("Starting TUI")
		vapp.Start()

		// Wait for TUI virtual app to finish (blocks until user exits TUI)
		// This keeps the REST servers and other transports running while TUI is active
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		slog.Info("Shutting down")
		vapp.Stop()
	} else {
		runHeadless(srv, rootLogger)
	}
}

func runHeadless(srv *server.Server, rootLogger *slog.Logger) {
	slog.Info("Running in headless mode", "hint", "Use -tui flag for interactive interface")

	// Create a client to check initial state
	client := server.NewClient(srv.RequestChannel(), rootLogger.With("component", "headless-client"))
	count, err := client.AccountCount(context.Background())
	if err != nil {
		slog.Warn("Failed to get initial account count", "error", err)
	} else {
		slog.Info("App initialized",
			"account_count", count,
			"virtual_app_count", srv.Registry().Count(),
		)
	}

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	slog.Info("Server ready", "hint", "Press Ctrl+C to exit")
	sig := <-sigChan
	slog.Info("Received shutdown signal", "signal", sig.String())
}

func setupStdoutLogging(logLevel, logFormat string) {
	setupLogging(os.Stdout, logLevel, logFormat)
}

func setupFileLogging(logFile, logLevel, logFormat string) {
	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open log file: %v\n", err)
		// Fall back to discard
		setupLogging(io.Discard, logLevel, logFormat)
		return
	}
	setupLogging(f, logLevel, logFormat)
}

func setupLogging(writer io.Writer, logLevel, logFormat string) {
	var level slog.Level
	switch logLevel {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: level == slog.LevelDebug,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.String("timestamp", a.Value.Time().Format("2006-01-02T15:04:05.000Z07:00"))
			}
			return a
		},
	}

	var handler slog.Handler
	if logFormat == "json" {
		handler = slog.NewJSONHandler(writer, opts)
	} else {
		handler = slog.NewTextHandler(writer, opts)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)
}
