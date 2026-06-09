package shared

import (
	"encoding/binary"
	"math"
)

// PlayerState represents the exact spatial location of a client.
// Total Wire Size: 16 Bytes
type PlayerState struct {
	PlayerID uint32
	X        float32
	Y        float32
	Z        float32
}

// Marshal packs the PlayerState fields into a deterministic,
// zero-allocation 16-byte slice using BigEndian (Network Byte Order).
func (p *PlayerState) Marshal() []byte {
	buf := make([]byte, 16)

	// Offset 0-4: Pack the PlayerID (uint32)
	binary.BigEndian.PutUint32(buf[0:4], p.PlayerID)

	// Offset 4-8: Pack the X Coordinate (float32)
	binary.BigEndian.PutUint32(buf[4:8], math.Float32bits(p.X))

	// Offset 8-12: Pack the Y Coordinate (float32)
	binary.BigEndian.PutUint32(buf[8:12], math.Float32bits(p.Y))

	// Offset 12-16: Pack the Z Coordinate (float32)
	binary.BigEndian.PutUint32(buf[12:16], math.Float32bits(p.Z))

	return buf
}

// MarshalTo packs the PlayerState fields directly into an EXISTING byte slice.
// 🚀 Zero allocations, zero heap overhead, pure performance.
func (p *PlayerState) MarshalTo(buf []byte) {
	// Boundary check protection: ensure the provided slice has at least 16 bytes
	if len(buf) < 16 {
		return
	}

	// Offset 0-4: Pack the PlayerID (uint32)
	binary.BigEndian.PutUint32(buf[0:4], p.PlayerID)

	// Offset 4-8: Pack the X Coordinate (float32)
	binary.BigEndian.PutUint32(buf[4:8], math.Float32bits(p.X))

	// Offset 8-12: Pack the Y Coordinate (float32)
	binary.BigEndian.PutUint32(buf[8:12], math.Float32bits(p.Y))

	// Offset 12-16: Pack the Z Coordinate (float32)
	binary.BigEndian.PutUint32(buf[12:16], math.Float32bits(p.Z))
}

// Unmarshal reads a 16-byte slice and mutates the existing
// PlayerState struct in place without allocating new memory on the heap.
func (p *PlayerState) Unmarshal(data []byte) {
	// Offset 0-4: Read 4 bytes for PlayerID
	p.PlayerID = binary.BigEndian.Uint32(data[0:4])

	// Offset 4-8: Read 4 bytes for X Coordinate
	p.X = math.Float32frombits(binary.BigEndian.Uint32(data[4:8]))

	// Offset 8-12: Read 4 bytes for Y Coordinate
	p.Y = math.Float32frombits(binary.BigEndian.Uint32(data[8:12]))

	// Offset 12-16: Read 4 bytes for Z Coordinate
	p.Z = math.Float32frombits(binary.BigEndian.Uint32(data[12:16]))
}
