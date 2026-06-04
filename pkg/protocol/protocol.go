package protocol

import (
	"encoding/binary"
	"errors"
	"io"
)

const (
	// HeaderSize represents the exact fixed byte length of the protocol header
	// Type (1 Byte) + Length (4 Bytes) = 5 Bytes
	HeaderSize = 5

	// MaxPayloadSize enforces an aggressive 16MB ceiling on incoming frames.
	// This directly prevents OOM panics caused by malicious or corrupted length values.
	MaxPayloadSize = 16 * 1024 * 1024 // 16 MB
)

var (
	ErrPayloadTooLarge = errors.New("protocol boundary error: payload size exceeds maximum ceiling")
	ErrInvalidPacket   = errors.New("protocol integrity error: malformed packet read")
)

// Packet represents the rigid Type-Length-Value contract for Telesect routing.
type Packet struct {
	Type   byte
	Length uint32
	Value  []byte
}

// NewPacket constructs a clean, memory-safe Telesect packet wrapper.
func NewPacket(pType byte, value []byte) (*Packet, error) {
	if len(value) > MaxPayloadSize {
		return nil, ErrPayloadTooLarge
	}
	return &Packet{
		Type:   pType,
		Length: uint32(len(value)),
		Value:  value,
	}, nil
}

// Marshal writes the packet out to an io.Writer according to the 5-byte big-endian specification.
func (p *Packet) Marshal(w io.Writer) error {
	if len(p.Value) > MaxPayloadSize {
		return ErrPayloadTooLarge
	}

	// Allocate a single contiguous buffer for the header to reduce allocation fragmentation
	header := make([]byte, HeaderSize)
	header[0] = p.Type

	// Encode length field as a 32-bit big-endian unsigned integer
	binary.BigEndian.PutUint32(header[1:5], p.Length)

	// Write out the 5-byte header frame
	if _, err := w.Write(header); err != nil {
		return err
	}

	// If a payload exists, write it out immediately following the boundary
	if p.Length > 0 {
		if _, err := w.Write(p.Value); err != nil {
			return err
		}
	}

	return nil
}

// UnmarshalPacket extracts a single Telesect packet frame from an io.Reader stream.
// It applies strict boundary validations on the incoming header to guarantee memory safety.
func UnmarshalPacket(r io.Reader) (*Packet, error) {
	// Allocate our fixed 5-byte stack bucket for header ingestion
	header := make([]byte, HeaderSize)

	// io.ReadFull guarantees we block until exactly 5 bytes are pulled,
	// or we catch an immediate EOF/network termination state.
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, err
	}

	// Extract data type routing identifier
	pType := header[0]

	// Extract length field utilizing optimized Big-Endian math
	length := binary.BigEndian.Uint32(header[1:5])

	// 🛡️ SECURITY GUARD RAIL: Evaluate length claim BEFORE memory allocation
	if length > MaxPayloadSize {
		return nil, ErrPayloadTooLarge
	}

	// Initialize our value buffer safely now that the boundary verification passed
	value := make([]byte, length)

	// If the payload specifies data, read exactly that many remaining bytes from the stream
	if length > 0 {
		if _, err := io.ReadFull(r, value); err != nil {
			// If the connection drops here, the frame was corrupted or truncated
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				return nil, ErrInvalidPacket
			}
			return nil, err
		}
	}

	return &Packet{
		Type:   pType,
		Length: length,
		Value:  value,
	}, nil
}
