package transport

import (
	"net"
	"testing"

	"telesect/pkg/protocol"
)

// BenchmarkServer_Throughput measures the raw packet processing speed
// and memory footprint of the entire transport ingestion pipeline.
func BenchmarkServer_Throughput(b *testing.B) {
	// 1. Initialize Server on an ephemeral local port
	srv := NewServer("127.0.0.1:0")
	if err := srv.Start(); err != nil {
		b.Fatalf("Failed to start server for benchmark: %v", err)
	}
	defer srv.Stop()

	// 2. Spin up a hyper-fast background consumer to drain the InboundRouter channel.
	// This prevents the channel buffer (1000) from filling up and stalling the engine.
	go func() {
		for range srv.InboundRouter {
			for packet := range srv.InboundRouter {
				// ─── NEW: RECYCLE THE PACKET IMMEDIATELY ───
				packet.Release()
			}
		}
	}()

	// 3. Dial an external client into the active server link
	serverAddr := srv.listener.Addr().String()
	clientConn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		b.Fatalf("Failed to dial benchmark target: %v", err)
	}
	defer clientConn.Close()

	// 4. Pre-allocate a standard 100-byte agritech/telemetry sensor payload.
	// We allocate this outside the timer window to avoid benchmarking Go's string allocation mechanics.
	mockPayload := []byte("hub_id:rootone_cli_v1,sensor_zone:alpha,hydro_ph:6.2,water_temp:21.8C,status:nominal_throughput")
	packet := &protocol.Packet{
		Type:   0x02, // Agritech telemetry vector
		Length: uint32(len(mockPayload)),
		Value:  mockPayload,
	}

	// 5. Enable memory allocation reporting tracking
	b.ReportAllocs()

	// 6. Reset the timer to ignore all the setup and network dial overhead
	b.ResetTimer()

	// ─── NEW: Declare a scratchpad for the benchmark loop ───
	var scratch [protocol.HeaderSize]byte

	// 7. Execute the hot-path loop b.N times
	for i := 0; i < b.N; i++ {
		// Pass the scratchpad slice here
		if err := packet.Marshal(clientConn, scratch[:]); err != nil {
			b.Fatalf("Iteration %d failed to marshal packet to wire: %v", i, err)
		}
	}
}
