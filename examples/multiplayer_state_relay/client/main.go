package main

import (
	"context"
	"fmt"
	"math"
	"os"
	"os/signal"
	"syscall"
	"time"

	"telesect/examples/multiplayer_state_relay/shared"
	"telesect/internal/transport"
	"telesect/pkg/protocol"
	// Note: You will eventually import your Telesect client dialer here
)

func main() {
	fmt.Println("🎮 Starting Telesect Mock Game Client...")

	// 1. Setup graceful shutdown context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	// 2. Initialize our simulated player data
	player := &shared.PlayerState{
		PlayerID: 99,
		X:        0.0,
		Y:        1.0,
		Z:        0.0,
	}

	// 3. DIAL THE LIVE TELESECT SERVER
	fmt.Println("[Client] Dialing Telesect Server at 127.0.0.1:7777...")
	conn, err := transport.Dial("127.0.0.1:7777")
	if err != nil {
		fmt.Printf("Fatal: Could not connect to server: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	// Create our strict 60Hz ticker
	ticker := time.NewTicker(16666 * time.Microsecond)
	defer ticker.Stop()

	// 4. Run the simulation loop
	go func() {
		var angle float32 = 0.0

		for {
			select {
			case <-ctx.Done():
				fmt.Println("[Client] Simulation loop stopped.")
				return
			case <-ticker.C:
				// Simulate walking in a circle
				angle += 0.05
				player.X = float32(math.Cos(float64(angle))) * 10.0
				player.Z = float32(math.Sin(float64(angle))) * 10.0

				// Allocate a discrete 16-byte frame buffer for this specific network packet
				frameBuffer := make([]byte, 16)
				player.MarshalTo(frameBuffer)

				// Wrap the buffer in a Telesect network frame (0x10 = Game State update)
				packet := &protocol.Packet{
					Type:   0x10,
					Length: uint32(len(frameBuffer)),
					Value:  frameBuffer,
				}

				// Push the packet directly into Telesect's thread-safe queue
				select {
				case conn.SendChan <- packet:
					// Queued for transport successfully
					fmt.Printf("[🫵 Client] Sent 16-Byte Spatial Frame | X: %.2f Z: %.2f\n", player.X, player.Z)
				default:
					// If the outbound queue fills up, drop the frame to handle backpressure gracefully
					fmt.Println("[Client Warning] Outbound queue saturated, dropping frame.")
				}
			}
		}
	}()

	// Block here until CTRL+C
	<-sigCh
	fmt.Println("\n[Client] Shutting down gracefully...")
	cancel()
	time.Sleep(100 * time.Millisecond)
}
