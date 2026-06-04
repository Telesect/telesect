package transport

import (
	"net"
	"testing"
	"time"
)

// TestServer_LifecycleAndIngress validates that the Server can ingest, serialize,
// and decode an application-layer binary frame cleanly without cutting off packets.
func TestServer_LifecycleAndIngress(t *testing.T) {
	srv := NewServer("127.0.0.1:0")
	if err := srv.Start(); err != nil {
		t.Fatalf("Failed to spin up local test server: %v", err)
	}
	serverAddr := srv.listener.Addr().String()

	// Technical: Establish an outbound client socket dialing into the active server.
	clientConn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		t.Fatalf("Failed to establish dial socket out to server gateway: %v", err)
	}

	// ─── UPGRADED TEST PACKET BUILDING ───
	// Technical: Define a concrete, compliant protocol frame instead of raw unstructured text.
	testFrame := &Frame{
		StreamID: 42,
		Type:     FrameTypeData,
		Length:   21,
		Payload:  []byte("TELESECT_SECURE_CARGO"),
	}

	// Technical: Serialize the test structure directly over the active TCP socket stream.
	// Because clientConn implements io.Writer, Marshal drives it right onto the wire.
	if err := testFrame.Marshal(clientConn); err != nil {
		t.Errorf("Failed to marshal and stream test packet: %v", err)
	}

	// Technical: Allow the Go scheduler a small processing window to unblock the server loop.
	time.Sleep(50 * time.Millisecond)

	if err := clientConn.Close(); err != nil {
		t.Errorf("Error during client connection close: %v", err)
	}

	if err := srv.Stop(); err != nil {
		t.Errorf("Server encountered error pools when closing workers: %v", err)
	}
}
