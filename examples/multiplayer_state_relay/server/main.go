package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"telesect/examples/multiplayer_state_relay/shared"
	"telesect/internal/transport"
	"telesect/pkg/protocol"
	// Note: You will need to import your Telesect server package here
	// e.g., "telesect/internal/transport"
)

// ---------------------------------------------------------
// 1. THE ZERO-ALLOCATION STATE MANAGER
// ---------------------------------------------------------

type WorldManager struct {
	state   map[uint32]shared.PlayerState
	updates chan shared.PlayerState
}

func NewWorldManager() *WorldManager {
	return &WorldManager{
		state:   make(map[uint32]shared.PlayerState),
		updates: make(chan shared.PlayerState, 1024),
	}
}

func (m *WorldManager) Run(ctx context.Context) {
	fmt.Println("[WorldManager] Central state registry online.")
	for {
		select {
		case <-ctx.Done():
			fmt.Println("[WorldManager] Tearing down state registry.")
			return
		case update := <-m.updates:
			m.state[update.PlayerID] = update
		}
	}
}

// ---------------------------------------------------------
// 2. THE NETWORK WORKER
// ---------------------------------------------------------

func stateWorker(ctx context.Context, workerID int, inbound <-chan *protocol.Packet, manager *WorldManager) {
	fmt.Printf("[Worker %d] Standing by for incoming game state updates...\n", workerID)
	for {
		select {
		case <-ctx.Done():
			fmt.Printf("[Worker %d] Shutting down.\n", workerID)
			return
		case packet, ok := <-inbound:
			if !ok {
				return
			}

			if packet.Type == 0x10 {
				var playerUpdate shared.PlayerState
				playerUpdate.Unmarshal(packet.Value)
				manager.updates <- playerUpdate

				fmt.Printf("[⚡ Worker %d] Processed Packet 0x10 | Player: %d | Position -> X: %.2f, Z: %.2f\n",
					workerID, playerUpdate.PlayerID, playerUpdate.X, playerUpdate.Z)
			}

			// Return the packet to Telesect's sync.Pool
			packet.Release()
		}
	}
}

// ---------------------------------------------------------
// 3. THE EXECUTABLE ENTRY POINT
// ---------------------------------------------------------

func main() {
	fmt.Println("🚀 Booting Telesect Multiplayer State Relay (Server)...")

	// 1. Set up our graceful shutdown context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Listen for CTRL+C (SIGINT) or Kubernetes eviction (SIGTERM)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	// 2. Boot the zero-allocation World Manager in the background
	manager := NewWorldManager()
	go manager.Run(ctx)

	// 3. INITIALIZE REAL TELESECT TRANSPORT SERVER
	fmt.Println("[Server] Binding Telesect transport to port 7777...")
	srv := transport.NewServer(":7777")

	// Start the server gates and background loops
	if err := srv.Start(); err != nil {
		fmt.Printf("Fatal server start error: %v\n", err)
		os.Exit(1)
	}
	defer srv.Stop() // Guarantees all sockets close cleanly on exit

	// 4. Boot our network worker to translate Telesect packets to Game State
	// We pass srv.InboundRouter directly into the worker!
	go stateWorker(ctx, 1, srv.InboundRouter, manager)

	// 5. Block the main thread until someone presses CTRL+C
	<-sigCh
	fmt.Println("\n[Server] Shutdown signal received. Cascading context cancellation...")

	// Press the kill-switch. This instantly stops the WorldManager and the stateWorker.
	cancel()
	time.Sleep(100 * time.Millisecond) // Give routines a split second to finish exit logging
	fmt.Println("[Server] Graceful shutdown complete.")
}
