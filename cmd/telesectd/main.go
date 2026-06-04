package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"telesect/internal/transport"
)

func main() {
	// 1. Initialize the Core Server Primitive
	srv := transport.NewServer("127.0.0.1:8080")

	// 2. Launch the Server on a Dedicated Thread Stack
	// Technical: We run srv.Start() in a goroutine because srv.listener.Accept() is a blocking loop.
	// This leaves the main thread free to manage the system intercept trap below.
	go func() {
		fmt.Println("[Engine Initialization] Launching Telesect core relay backbone on port 8080...")
		if err := srv.Start(); err != nil {
			fmt.Printf("[Fatal Engine Error] Core gateway failed to execute: %v\n", err)
			os.Exit(1)
		}
	}()

	// Allow the background server a few milliseconds to bind to the port cleanly
	time.Sleep(50 * time.Millisecond)

	// ─── MILESTONE 1.3: SYSTEM INTERRUPT TRAP ───
	// Technical: Create a buffered channel capable of holding 1 operating system signal token.
	sigChan := make(chan os.Signal, 1)

	// Technical: Register our channel with the OS runtime. We explicitly trap SIGINT (Ctrl+C)
	// and SIGTERM (the standard termination request sent by Kubernetes pods during a rollout).
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Technical: The main thread hits this line and drops into a deep sleep.
	// It consumes 0% CPU while waiting for the operating system to fire a signal into sigChan.
	capturedSignal := <-sigChan
	fmt.Printf("\n[System Interrupt] Caught termination signal (%v). Triggering graceful drain...\n", capturedSignal)

	// ─── THE GRACEFUL SHUTDOWN CHAIN ───
	fmt.Println("[Lifecycle] Commencing total resource reclamation...")

	// Technical: Call our verified Stop() method. This initiates the cascading teardown
	// we just built, unblocking all loops and clearing out the registry map.
	if err := srv.Stop(); err != nil {
		fmt.Printf("[Shutdown Warning] Resource reclamation encountered anomalies: %v\n", err)
	}

	fmt.Println("[Lifecycle] All network descriptors closed. Connection registry zeroed out.")
	fmt.Println("[Lifecycle] Telesect binary exiting cleanly. Safe journey, cargo.")
}
