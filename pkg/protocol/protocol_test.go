package protocol

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"
)

// TestPacket_MarshalUnmarshal_HappyPath verifies a perfect data cycle:
// packing structured data into stream bytes and parsing it back flawlessly.
func TestPacket_MarshalUnmarshal_HappyPath(t *testing.T) {
	payload := []byte("Telesect-Agritech-Payload-0x02")
	packet, err := NewPacket(0x02, payload)
	if err != nil {
		t.Fatalf("Failed to create valid packet: %v", err)
	}

	// Use a bytes.Buffer to simulate our low-overhead TCP pipe
	var stream bytes.Buffer

	// Serialize into the stream
	if err := packet.Marshal(&stream); err != nil {
		t.Fatalf("Failed to marshal packet: %v", err)
	}

	// Deserialize out of the stream
	parsedPacket, err := UnmarshalPacket(&stream)
	if err != nil {
		t.Fatalf("Failed to unmarshal packet: %v", err)
	}

	// Assert boundaries and internal invariants
	if parsedPacket.Type != 0x02 {
		t.Errorf("Expected type 0x02, got 0x%02x", parsedPacket.Type)
	}
	if parsedPacket.Length != uint32(len(payload)) {
		t.Errorf("Expected length %d, got %d", len(payload), parsedPacket.Length)
	}
	if !bytes.Equal(parsedPacket.Value, payload) {
		t.Errorf("Payload mismatch. Expected %s, got %s", payload, parsedPacket.Value)
	}
}

// TestUnmarshalPacket_OOMDefense guarantees that if a malicious payload size
// is injected into the stream header, our engine catches it BEFORE allocating memory.
func TestUnmarshalPacket_OOMDefense(t *testing.T) {
	var maliciousStream bytes.Buffer

	// Construct a rogue 5-byte header manually
	// Type: 0x00, Length: 400 Megabytes (419,430,400 bytes) -> well over our 16MB ceiling
	maliciousStream.WriteByte(0x00)

	hugeLengthBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(hugeLengthBytes, 400*1024*1024)
	maliciousStream.Write(hugeLengthBytes)

	// Attempt to unmarshal the malicious stream frame
	_, err := UnmarshalPacket(&maliciousStream)

	// Verify the engine intercepts the execution vector cleanly
	if !errors.Is(err, ErrPayloadTooLarge) {
		t.Errorf("Security flaw: Expected ErrPayloadTooLarge, got %v", err)
	}
}

// TestUnmarshalPacket_TruncatedStream verifies that if the stream cuts off unexpectedly
// mid-payload, Telesect surfaces a clean integrity error rather than stalling.
func TestUnmarshalPacket_TruncatedStream(t *testing.T) {
	var corruptedStream bytes.Buffer

	// Header states Type 0x01, Payload Length 10 bytes
	corruptedStream.WriteByte(0x01)
	lengthBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBytes, 10)
	corruptedStream.Write(lengthBytes)

	// MALICIOUS OR BROKEN DELAY: Only send 3 bytes of data instead of the promised 10
	corruptedStream.Write([]byte{0xA, 0xB, 0xC})

	_, err := UnmarshalPacket(&corruptedStream)

	if !errors.Is(err, ErrInvalidPacket) {
		t.Errorf("Expected ErrInvalidPacket for truncated wire stream, got %v", err)
	}
}
