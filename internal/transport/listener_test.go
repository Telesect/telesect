package transport

import (
	"net"
	"testing"
	"time"

	"telesect/pkg/protocol"
)

// TestServer_PacketIngress verifies that a packet sent from a raw TCP client
// is successfully parsed by the connection workers and dumped directly onto
// the central master InboundRouter switchboard channel.
func TestServer_PacketIngress(t *testing.T) {
	// 1. Initialize Server on an ephemeral port (:0 forces the OS to pick an open port)
	srv := NewServer("127.0.0.1:0")
	if err := srv.Start(); err != nil {
		t.Fatalf("Failed to start transport server: %v", err)
	}
	defer srv.Stop()

	// Capture the dynamically allocated address
	serverAddr := srv.listener.Addr().String()

	// 2. Dial into the active logistics hub as an external client socket
	clientConn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		t.Fatalf("Failed to dial test server at %s: %v", serverAddr, err)
	}
	defer clientConn.Close()

	// 3. Construct a production-grade 5-byte type-aware packet contract
	mockPacket := &protocol.Packet{
		Type:  0x02, // Transparent routing identifier (e.g., Agritech/Sensor hub vector)
		Value: []byte("hub_id:rootone_01,status:active,temp:24.5C"),
	}
	mockPacket.Length = uint32(len(mockPacket.Value))

	// ─── NEW: Declare a scratchpad for the test ───
	var scratch [protocol.HeaderSize]byte

	// 4. Serialize and transmit the packet directly over the network wire
	if err := mockPacket.Marshal(clientConn, scratch[:]); err != nil {
		t.Fatalf("Client failed to serialize and transmit packet: %v", err)
	}

	// 5. Enforce an asynchronous race-safe assertion window
	select {
	case receivedPacket := <-srv.InboundRouter:
		// Structural Integrity Assertions
		if receivedPacket.Type != mockPacket.Type {
			t.Errorf("Packet Type mismatch: expected 0x%02x, got 0x%02x", mockPacket.Type, receivedPacket.Type)
		}
		if receivedPacket.Length != mockPacket.Length {
			t.Errorf("Packet Length mismatch: expected %d, got %d", mockPacket.Length, receivedPacket.Length)
		}
		if string(receivedPacket.Value) != string(mockPacket.Value) {
			t.Errorf("Payload corruption detected: expected %q, got %q", string(mockPacket.Value), string(receivedPacket.Value))
		}

		// ─── NEW: CLEAN UP AND RECYCLE ───
		receivedPacket.Release()

	case <-time.After(2 * time.Second):
		t.Fatal("Timeout violation: Server connection workers failed to route packet to master switchboard within 2s")
	}
}

// TestServer_GracefulShutdownClearsRegistry verifies that the connection tracking
// clipboards are scrubbed cleanly and file descriptors are closed on terminal stop signals.
func TestServer_GracefulShutdownClearsRegistry(t *testing.T) {
	srv := NewServer("127.0.0.1:0")
	if err := srv.Start(); err != nil {
		t.Fatalf("Failed to start transport server: %v", err)
	}
	serverAddr := srv.listener.Addr().String()

	clientConn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		t.Fatalf("Failed to dial test server: %v", err)
	}
	defer clientConn.Close()

	// Give the concurrent acceptLoop goroutine a micro-window to execute registration
	time.Sleep(15 * time.Millisecond)

	// Verify client is accounted for in the directory map
	srv.mu.RLock()
	connectionCount := len(srv.connections)
	srv.mu.RUnlock()

	if connectionCount != 1 {
		t.Errorf("Expected connection directory count to be 1, found: %d", connectionCount)
	}

	// Fire the facility master breaker kill switch
	if err := srv.Stop(); err != nil {
		t.Fatalf("Server shutdown chain encountered an anomaly: %v", err)
	}

	// Verify all descriptors have been scrubbed from memory allocation tracking
	srv.mu.RLock()
	postShutdownCount := len(srv.connections)
	srv.mu.RUnlock()

	if postShutdownCount != 0 {
		t.Errorf("Memory leak warning: Registry map not zeroed out after shutdown, remaining: %d", postShutdownCount)
	}
}
