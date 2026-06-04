package transport

import (
	"encoding/binary"
	"errors"
	"io"
)

// Frame Types
const (
	FrameTypeData    uint8 = 0x01 // Technical: Identifies operational payload frames. Analogy: Standard cargo box.
	FrameTypePing    uint8 = 0x02 // Technical: Identifies heartbeat keep-alive frames. Analogy: "Ping" radar pulse.
	FrameTypeControl uint8 = 0x03 // Technical: Identifies system plane control flags. Analogy: Emergency red flag.
)

// HeaderSize defines the exact byte layout allocation required for a frame header.
// 4 bytes (StreamID) + 1 byte (Type) + 2 bytes (Length) = 7 bytes.
const HeaderSize = 7

var ErrInvalidPayloadSize = errors.New("frame: payload dimensions exceed length field boundaries")

// Frame represents the structured application-layer wrapper for all Telesect network traffic.
type Frame struct {
	StreamID uint32 // Technical: Sub-channel multiplex index. Analogy: The internal apartment room number.
	Type     uint8  // Technical: Contextual operational flag. Analogy: The delivery category sticker.
	Length   uint16 // Technical: Size bounds of trailing payload. Analogy: The declared weight profile of the crate.
	Payload  []byte // Technical: Raw application bytes. Analogy: The physical contents inside the crate.
}

// Marshal writes the frame's structured values out to an io.Writer stream in Network Byte Order.
// ─── TECHNICAL ───
// Employs a zero-allocation optimization by creating a fixed stack buffer for the header fields,
// serializing integers to Big Endian, and performing sequentially chained writes across the network socket pipe.
// ─── ANALOGY ───
// The packaging station: Takes loose items, wraps them in a standardized container with a 7-byte shipping label slapped on front.
func (f *Frame) Marshal(w io.Writer) error {
	if int(f.Length) != len(f.Payload) {
		return ErrInvalidPayloadSize
	}

	// Technical: Allocate a 7-byte array directly on the thread stack. Bypasses the heap allocation entirely.
	headerBuf := make([]byte, HeaderSize)

	// Technical: Serialize fields sequentially into target byte slices enforcing Big Endian formatting.
	binary.BigEndian.PutUint32(headerBuf[0:4], f.StreamID)
	headerBuf[4] = f.Type
	binary.BigEndian.PutUint16(headerBuf[5:7], f.Length)

	// Technical: Transmit the fixed header block over the I/O connection stream first.
	if _, err := w.Write(headerBuf); err != nil {
		return err
	}

	// Technical: If a payload exists, transmit it immediately following the header block.
	if f.Length > 0 {
		if _, err := w.Write(f.Payload); err != nil {
			return err
		}
	}

	return nil
}

// UnmarshalRead reads from an io.Reader and builds a structured Frame instance.
// ─── TECHNICAL ───
// Performs a precise two-stage read dance. First, it pulls exactly HeaderSize bytes to decode the protocol
// boundaries. Then, it dynamically sizes and fills a payload bucket to match the declared Length.
// ─── ANALOGY ───
// The receiving clerk: Reads exactly the first 7 centimeters of the incoming container to read the label.
// If the label says the package contains 50 items, the clerk opens the door wide enough to pull exactly 50 items out.
func UnmarshalRead(r io.Reader) (*Frame, error) {
	headerBuf := make([]byte, HeaderSize)

	// Technical: io.ReadFull blocks until the exact buffer capacity is completely populated from the wire.
	// This prevents partial read anomalies common in raw TCP streams.
	if _, err := io.ReadFull(r, headerBuf); err != nil {
		return nil, err
	}

	frame := &Frame{}
	frame.StreamID = binary.BigEndian.Uint32(headerBuf[0:4])
	frame.Type = headerBuf[4]
	frame.Length = binary.BigEndian.Uint16(headerBuf[5:7])

	if frame.Length > 0 {
		// Technical: Allocate space for the incoming payload now that we know the exact size bound.
		frame.Payload = make([]byte, frame.Length)
		if _, err := io.ReadFull(r, frame.Payload); err != nil {
			return nil, err
		}
	}

	return frame, nil
}
