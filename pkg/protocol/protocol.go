package protocol

import (
	"encoding/binary"
	"errors"
	"io"
	"sync"
)

const (
	// HeaderSize represents the exact fixed byte length of the protocol header.
	// It consists of Type (1 Byte) + Length (4 Bytes).
	HeaderSize = 5

	// MaxPayloadSize enforces an aggressive 16MB ceiling on incoming frames
	// to prevent Out-Of-Memory (OOM) panics caused by malicious length values.
	MaxPayloadSize = 16 * 1024 * 1024 // 16 MB
)

const (
	// TypeWindowUpdate is a reserved internal engine frame for backpressure flow control.
	// This packet type is intercepted at the transport layer and is invisible to end-users.
	TypeWindowUpdate byte = 0x05
)

const (
	// WindowStatePause signals the remote WriteLoop to freeze application data transmission.
	WindowStatePause byte = 0x00

	// WindowStateResume signals the remote WriteLoop to resume application data transmission.
	WindowStateResume byte = 0x01
)

var (
	// ErrPayloadTooLarge is returned when an incoming or outbound payload exceeds MaxPayloadSize.
	ErrPayloadTooLarge = errors.New("protocol boundary error: payload size exceeds maximum ceiling")

	// ErrInvalidPacket is returned when read data is truncated or violates wire integrity.
	ErrInvalidPacket = errors.New("protocol integrity error: malformed packet read")
)

// packetPool manages reusable Packet instances to minimize heap allocation under high throughput.
var packetPool = sync.Pool{
	New: func() any {
		return &Packet{
			Value: make([]byte, 0, 1024),
		}
	},
}

// Packet represents the Type-Length-Value (TLV) contract for all Telesect routing.
type Packet struct {
	Type   byte   // The operational or application-specific identifier frame.
	Length uint32 // The explicit size of the payload value in bytes.
	Value  []byte // The raw underlying binary payload.
}

// NewPacket constructs a validated Packet wrapper. It returns ErrPayloadTooLarge
// if the value byte slice exceeds the internal protocol safety thresholds.
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

// Release zeroes out the Packet fields and returns the instance to the internal sync.Pool.
// This must be explicitly called by the application layer once processing is complete.
func (p *Packet) Release() {
	p.Type = 0
	p.Length = 0
	p.Value = p.Value[:0]
	packetPool.Put(p)
}

// Marshal serializes the Packet into an io.Writer using a pre-allocated scratchpad buffer.
// The provided buf slice must be at least HeaderSize (5 bytes) long to bypass escape analysis.
func (p *Packet) Marshal(w io.Writer, buf []byte) error {
	if len(p.Value) > MaxPayloadSize {
		return ErrPayloadTooLarge
	}
	if len(buf) < HeaderSize {
		return ErrInvalidPacket
	}

	buf[0] = p.Type
	binary.BigEndian.PutUint32(buf[1:5], p.Length)

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

// UnmarshalPacket reads a raw byte stream from an io.Reader and extracts a structured Packet.
// It draws allocations from a sync.Pool and utilizes the provided scratchpad buffer to remain zero-alloc.
func UnmarshalPacket(r io.Reader, buf []byte) (*Packet, error) {
	if len(buf) < HeaderSize {
		return nil, ErrInvalidPacket
	}

	if _, err := io.ReadFull(r, buf[:HeaderSize]); err != nil {
		return nil, err
	}

	pType := buf[0]
	length := binary.BigEndian.Uint32(buf[1:5])

	if length > MaxPayloadSize {
		return nil, ErrPayloadTooLarge
	}

	pkt := packetPool.Get().(*Packet)
	pkt.Type = pType
	pkt.Length = length

	if length > 0 {
		if cap(pkt.Value) < int(length) {
			pkt.Value = make([]byte, length)
		} else {
			pkt.Value = pkt.Value[:length]
		}

		if _, err := io.ReadFull(r, pkt.Value); err != nil {
			pkt.Release()
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				return nil, ErrInvalidPacket
			}
			return nil, err
		}
	}

	return pkt, nil
}
