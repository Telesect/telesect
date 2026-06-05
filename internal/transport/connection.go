package transport

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"telesect/pkg/protocol"
)

var ErrConnectionClosed = errors.New("transport: connection closed")

// Connection wraps a raw net.Conn with tracking metadata and safe atomic closing mechanisms.
// ─── ANALOGY ───
// This represents an individual Cargo Truck parked in a loading bay. It contains the raw content
// (the network connection) along with metadata tracking how long it has been there.
type Connection struct {
	net.Conn                 // Technical: Embeds raw TCP socket interface. Analogy: The truck's cargo hold.
	id         string        // Technical: Unique UUID/string locator. Analogy: The custom barcode attached to the truck.
	createdAt  time.Time     // Technical: Timestamp tracking connection initiation. Analogy: When the truck passed check-in.
	closeOnce  sync.Once     // Technical: Prevents multiple close operations. Analogy: A single-use safety clamp.
	closedChan chan struct{} // Technical: Broadcast coordination channel. Analogy: The truck's dashboard alarm siren.

	// ─── UPDATED: TYPE-AWARE CODES ───
	// SendChan acts as the thread-safe outbound mailbox. The background WriteLoop
	// pulls packets from here and marshals them into raw wire frames.
	SendChan chan *protocol.Packet

	// ─── NEW: INTERNAL FLOW CONTROL ───
	// FlowControlChan acts as the direct brake pedal for the WriteLoop.
	// It carries the 1-byte state flags (0x00 Pause, 0x01 Resume).
	FlowControlChan chan byte

	// ─── NEW: ZERO-ALLOCATION SCRATCH BUFFERS ───
	// Allocated once per connection lifetime. Completely eliminates header heap generation.
	readScratch  [protocol.HeaderSize]byte
	writeScratch [protocol.HeaderSize]byte
}

// NewConnection initializes a monitored connection wrapper.
func NewConnection(id string, conn net.Conn) *Connection {
	return &Connection{
		Conn:       conn,
		id:         id,
		createdAt:  time.Now(),
		closedChan: make(chan struct{}),
		// Buffered to allow up to 100 type-validated packets to queue before blocking the sender.
		SendChan: make(chan *protocol.Packet, 100),
		// Buffered to 10 to ensure the engine can rapidly shift states without blocking
		FlowControlChan: make(chan byte, 10),
	}
}

// ID returns the unique string identifier for this client stream.
func (c *Connection) ID() string {
	return c.id
}

// Close ensures the underlying TCP socket is terminated safely exactly once.
// ─── TECHNICAL ───
// Uses sync.Once to protect against race conditions if multiple processing goroutines try to destroy
// the connection at the same time. It handles resource descriptor cleanup safely.
// ─── ANALOGY ───
// The safety clamp is engaged. The robot snaps the alarm wire (closes closedChan) to notify the facility,
// and then physically unplugs the truck from the dock (closes the OS File Descriptor).
func (c *Connection) Close() error {
	var err error
	c.closeOnce.Do(func() {
		// Technical: Instantly unblocks all receivers reading from this channel across the application.
		// Analogy: Sounding the local siren so everyone knows this specific truck is leaving.
		close(c.closedChan)

		// Technical: Tells the OS Kernel to release the underlying socket file descriptor immediately.
		// Analogy: Disconnecting the physical fuel and cargo links from the truck.
		err = c.Conn.Close()
	})
	return err
}

// Done returns a receive-only channel that is closed when the connection is terminated.
// ─── TECHNICAL ───
// Returns a read-only token channel (<-chan) enforcing compiler boundaries so external consumers
// can only listen to lifecycle states, not manipulate or close them.
// ─── ANALOGY ───
// Provides a sensor hook that allows warehouse subsystems to check if this truck's siren has gone off.
func (c *Connection) Done() <-chan struct{} {
	return c.closedChan
}

// ReadLoop acts as the dedicated Unloader.
func (c *Connection) ReadLoop(packetInboundChan chan<- *protocol.Packet, idleTimeout time.Duration) {
	// Defensively ensure the connection is torn down when the reader breaks out
	defer c.Close()

	for {
		// 1. Defend Against Idle/Stalled Senders (Slowloris Protection)
		if idleTimeout > 0 {
			if err := c.Conn.SetReadDeadline(time.Now().Add(idleTimeout)); err != nil {
				// Kernel-level failure setting socket options; log and abort
				return
			}
		}

		// 2. ─── UPDATED: PASS THE SCRATCHPAD ───
		// Pass c.readScratch[:] to bypass interface escape analysis.
		packet, err := protocol.UnmarshalPacket(c.Conn, c.readScratch[:])
		if err != nil {
			switch {
			case errors.Is(err, io.EOF):
				return

			case errors.Is(err, protocol.ErrPayloadTooLarge):
				// 🛡️ SECURITY BREACH TRAP: A client is trying to crash our engine via buffer expansion.
				// We drop the connection instantly without waiting for a graceful flush.
				return

			case errors.Is(err, protocol.ErrInvalidPacket):
				// WIRE INTEGRITY FAULT: The client sent truncated data or wire corruption occurred.
				return

			default:
				// Catch generic network dropping/kernel resets (e.g., "connection reset by peer")
				return
			}
		}

		// 3. 🛡️ NEW: INTERCEPT RESERVED INTERNAL ENGINE CONTROL FRAMES
		if packet.Type == protocol.TypeWindowUpdate {
			if len(packet.Value) > 0 {
				stateSignal := packet.Value[0]
				select {
				case c.FlowControlChan <- stateSignal:
					// Signal successfully handed off to our local WriteLoop brake pedal
				case <-c.closedChan:
					return
				}
			}
			// CRITICAL: Continue the loop immediately!
			// This drops the 0x05 frame so it NEVER reaches the user's application-layer InboundRouter.
			continue
		}

		// 4. Route the Validated Application Frame Upstream to the Central Switchboard
		select {
		case packetInboundChan <- packet:
			// Packet safely handed off to the internal routing engine
		case <-c.closedChan:
			// The connection lifecycle was terminated externally; cease processing
			return
		}
	}
}

// WriteLoop acts as the dedicated Loader, now equipped with Application-Layer Flow Control.
func (c *Connection) WriteLoop(writeTimeout time.Duration) {
	// Ensure cleanup occurs if the write loop exits
	defer c.Close()

	// ─── NEW: THE THROTTLE STATE ───
	// activeSendChan controls whether we are pulling application packets.
	// It starts fully operational (unpaused), pointing directly to the user's outbox.
	activeSendChan := c.SendChan

	for {
		select {
		case <-c.closedChan:
			// Truck is leaving the loading bay; shut down the worker safely
			return

		// ─── NEW: ENGINE CONTROL LISTENER ───
		case state := <-c.FlowControlChan:
			if state == protocol.WindowStatePause {
				// ENGAGE BRAKES: Setting to nil disables the case below.
				// User data safely piles up in c.SendChan without being processed.
				activeSendChan = nil
				fmt.Printf("[Flow Control] %s triggered BACKPRESSURE PAUSE.\n", c.id)

			} else if state == protocol.WindowStateResume {
				// RELEASE BRAKES: Restore the pointer to the real channel.
				activeSendChan = c.SendChan
				fmt.Printf("[Flow Control] %s triggered BACKPRESSURE RESUME.\n", c.id)
			}

			// ─── MODIFIED: ACTIVE SEND CHANNEL ───
			// Notice this now ranges over activeSendChan, NOT c.SendChan directly.
		case packet, ok := <-activeSendChan:
			if !ok {
				return
			}

			if writeTimeout > 0 {
				if err := c.Conn.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil {
					return
				}
			}

			// 2. ─── UPDATED: PASS THE SCRATCHPAD ───
			// Serialize using the permanent write buffer to prevent heap allocations.
			if err := packet.Marshal(c.Conn, c.writeScratch[:]); err != nil {
				return
			}
		}
	}
}
