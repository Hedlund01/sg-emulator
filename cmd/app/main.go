package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"sg-emulator/internal/server"
	"sg-emulator/internal/transport/grpc"
	"sg-emulator/internal/transport/rest"
	"sg-emulator/internal/transport/tui"
)

func main() {
	useTUI := flag.Bool("tui", false, "Run with TUI interface")
	numRestApps := flag.Int("rest", 0, "Number of virtual app instances with REST transport")
	numGrpcApps := flag.Int("grpc", 0, "Number of virtual app instances with gRPC transport")
	flag.Parse()

	// Create and start the server (runs in its own goroutine)
	srv := server.New()
	srv.Start()
	defer srv.Stop()

	// Create REST virtual app instances
	if *numRestApps > 0 {
		fmt.Printf("Creating %d REST virtual app instances...\n", *numRestApps)
		for i := 0; i < *numRestApps; i++ {
			vapp, err := srv.CreateVirtualApp()
			if err != nil {
				fmt.Printf("  Error creating virtual app: %v\n", err)
				continue
			}

			restAddr := fmt.Sprintf("localhost:%d", 8080+i)
			vapp.AddTransport(rest.New(restAddr, vapp.Client()))
			vapp.Start()

			fmt.Printf("  Created virtual app (ID: %s)\n", vapp.ID())
			for tType, addr := range vapp.Addresses() {
				fmt.Printf("    - %s: %s\n", tType, addr)
			}
		}
	}

	// Create gRPC virtual app instances
	if *numGrpcApps > 0 {
		fmt.Printf("Creating %d gRPC virtual app instances...\n", *numGrpcApps)
		for i := 0; i < *numGrpcApps; i++ {
			vapp, err := srv.CreateVirtualApp()
			if err != nil {
				fmt.Printf("  Error creating virtual app: %v\n", err)
				continue
			}

			grpcAddr := fmt.Sprintf("localhost:%d", 50051+i)
			vapp.AddTransport(grpc.New(grpcAddr, vapp.Client()))
			vapp.Start()

			fmt.Printf("  Created virtual app (ID: %s)\n", vapp.ID())
			for tType, addr := range vapp.Addresses() {
				fmt.Printf("    - %s: %s\n", tType, addr)
			}
		}
	}

	if *useTUI {
		// Create TUI virtual app
		vapp, err := srv.CreateVirtualApp()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating TUI virtual app: %v\n", err)
			os.Exit(1)
		}

		client := server.NewClient(srv.RequestChannel())
		vapp.AddTransport(tui.New(client, srv))

		fmt.Println("Starting TUI...")
		vapp.Start()

		// Wait for TUI to finish (it will block)
		// The transport handles its own lifecycle
	} else {
		runHeadless(srv)
	}
}

func runHeadless(srv *server.Server) {
	fmt.Println("SG Emulator running in headless mode")
	fmt.Println("Use -tui flag to run with TUI interface")

	// Create a client to check initial state
	client := server.NewClient(srv.RequestChannel())
	count, err := client.AccountCount()
	if err != nil {
		fmt.Printf("App initialized (error getting count: %v)\n", err)
	} else {
		fmt.Printf("App initialized with %d accounts\n", count)
	}

	// Print registry info
	fmt.Printf("Registry has %d virtual apps\n", srv.Registry().Count())

	// This is where you would add gRPC/REST server startup

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println("\nPress Ctrl+C to exit...")
	<-sigChan
	fmt.Println("\nShutting down...")
}
