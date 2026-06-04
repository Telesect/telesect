package transport

import (
	"bytes"
	"testing"
)

// TestFrame_MarshalUnmarshal validates that a data frame can be converted to
// binary and reconstructed flawlessly without altering integers or slicing values.
func TestFrame_MarshalUnmarshal(t *testing.T) {
	originalFrame := &Frame{
		StreamID: 1024,
		Type:     FrameTypeData,
		Length:   11,
		Payload:  []byte("hello world"),
	}

	// Technical: bytes.Buffer implements both io.Writer and io.Reader natively in-memory.
	// Analogy: A clean, localized conveyor loop used to pass packages back and forth on a test bench.
	var buf bytes.Buffer

	// Technical: Test encoding the concrete structure into raw binary bytes.
	if err := originalFrame.Marshal(&buf); err != nil {
		t.Fatalf("Failed to marshal frame structure: %v", err)
	}

	// Technical: Assert that the buffer holds exactly 7 bytes (header) + 11 bytes (payload) = 18 bytes total.
	expectedTotalSize := HeaderSize + len(originalFrame.Payload)
	if buf.Len() != expectedTotalSize {
		t.Errorf("Unexpected serialized buffer capacity. Got %d, expected %d", buf.Len(), expectedTotalSize)
	}

	// Technical: Decode the raw binary blocks back into an explicit Go structure pointer.
	decodedFrame, err := UnmarshalRead(&buf)
	if err != nil {
		t.Fatalf("Failed to unmarshal binary stream from buffer context: %v", err)
	}

	// Technical: Deeply compare all structural fields between original and output variables.
	if decodedFrame.StreamID != originalFrame.StreamID {
		t.Errorf("StreamID mismatch. Got %d, expected %d", decodedFrame.StreamID, originalFrame.StreamID)
	}
	if decodedFrame.Type != originalFrame.Type {
		t.Errorf("Frame type mismatch. Got %x, expected %x", decodedFrame.Type, originalFrame.Type)
	}
	if !bytes.Equal(decodedFrame.Payload, originalFrame.Payload) {
		t.Errorf("Payload content corruption. Got %s, expected %s", decodedFrame.Payload, originalFrame.Payload)
	}
}

// TestFrame_LengthMismatch ensures that the serialization loop throws a hard defensive
// error if someone configures a frame's length flag to lie about its actual payload size.
func TestFrame_LengthMismatch(t *testing.T) {
	liarFrame := &Frame{
		StreamID: 1,
		Type:     FrameTypeData,
		Length:   500,             // Claiming 500 bytes
		Payload:  []byte("short"), // Providing only 5 bytes
	}

	var buf bytes.Buffer
	if err := liarFrame.Marshal(&buf); err == nil {
		t.Error("Expected marshal to fail due to structural size discrepancies, but returned nil error")
	}
}
