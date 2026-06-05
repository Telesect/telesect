package protocol

import (
	"encoding/binary"
	"errors"
	"io"
	"sync"
)

const (
	// HeaderSize represents the exact fixed byte length of the protocol header
	// Type (1 Byte) + Length (4 Bytes) = 5 Bytes
	HeaderSize = 5

	// MaxPayloadSize enforces an aggressive 16MB ceiling on incoming frames.
	// This directly prevents OOM panics caused by malicious or corrupted length values.
	MaxPayloadSize = 16 * 1024 * 1024 // 16 MB
)

// ─── INTERNAL ENGINE PROTOCOL IDENTIFIERS (Reserved 0x00 - 0x0F) ───
const (
	// TypeWindowUpdate is a reserved INTERNAL engine frame for backpressure flow control.
	// End-users will never see this packet; the transport layer intercepts it.
	TypeWindowUpdate byte = 0x05

	// Note: In the future, we might add TypePing = 0x06, TypeHandshake = 0x07, etc.
)

// ─── INTERNAL FLOW CONTROL STATES ───
const (
	// WindowStatePause tells the remote WriteLoop to freeze transmission
	WindowStatePause byte = 0x00

	// WindowStateResume tells the remote WriteLoop to unfreeze transmission
	WindowStateResume byte = 0x01
)

var (
	ErrPayloadTooLarge = errors.New("protocol boundary error: payload size exceeds maximum ceiling")
	ErrInvalidPacket   = errors.New("protocol integrity error: malformed packet read")
)

// ─── NEW: THE PACKET RECYCLER POOL ───
var packetPool = sync.Pool{
	New: func() any {
		return &Packet{
			// Pre-allocate a 1KB baseline capacity for the value slice to eliminate
			// initial allocations for standard agritech telemetry frames.
			Value: make([]byte, 0, 1024),
		}
	},
}

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

// Release zeroes out the packet properties and returns the object to the pool.
// CRITICAL: The application layer MUST call this once processing is completed.
func (p *Packet) Release() {
	p.Type = 0
	p.Length = 0
	// Truncate length to 0 but retain underlying memory capacity for re-use
	p.Value = p.Value[:0]
	packetPool.Put(p)
}

// Marshal writes the packet out using a pre-allocated header buffer to eliminate heap escapes.
// The provided 'buf' MUST be at least HeaderSize (5 bytes) in length.
func (p *Packet) Marshal(w io.Writer, buf []byte) error {
	if len(p.Value) > MaxPayloadSize {
		return ErrPayloadTooLarge
	}
	if len(buf) < HeaderSize {
		return ErrInvalidPacket
	}

	buf[0] = p.Type
	binary.BigEndian.PutUint32(buf[1:5], p.Length)

	// Write out the pre-allocated 5-byte header frame
	if _, err := w.Write(buf[:HeaderSize]); err != nil {
		return err
	}

	if p.Length > 0 {
		if _, err := w.Write(p.Value); err != nil {
			return err
		}
	}
	return nil
}

// UnmarshalPacket extracts a frame using a pre-allocated header buffer to bypass escape analysis.
// The provided 'buf' MUST be at least HeaderSize (5 bytes) in length.
func UnmarshalPacket(r io.Reader, buf []byte) (*Packet, error) {
	if len(buf) < HeaderSize {
		return nil, ErrInvalidPacket
	}

	// Read directly into the provided reuseable scratchpad
	if _, err := io.ReadFull(r, buf[:HeaderSize]); err != nil {
		return nil, err
	}

	pType := buf[0]
	length := binary.BigEndian.Uint32(buf[1:5])

	if length > MaxPayloadSize {
		return nil, ErrPayloadTooLarge
	}

	// ─── UPDATED: DRAW PACKET WRAPPER FROM POOL ───
	pkt := packetPool.Get().(*Packet)
	pkt.Type = pType
	pkt.Length = length

	// ─── UPDATED: REUSE OR GROW SLICE MEMORY WITHOUT ALLOCATING ───
	if length > 0 {
		if cap(pkt.Value) < int(length) {
			// Grow backing array if incoming payload breaks current boundary
			pkt.Value = make([]byte, length)
		} else {
			// Reslice existing memory instantly (0 allocs)
			pkt.Value = pkt.Value[:length]
		}

		if _, err := io.ReadFull(r, pkt.Value); err != nil {
			pkt.Release() // Instantly recycle if read fails
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				return nil, ErrInvalidPacket
			}
			return nil, err
		}
	}

	return pkt, nil
}
