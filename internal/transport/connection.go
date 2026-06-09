// Package transport provides the core TCP socket lifecycle, flow control,
// and state management for the Telesect communication backbone.
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

// ErrConnectionClosed is returned when a read or write operation is attempted
// on a terminated transport link.
var ErrConnectionClosed = errors.New("transport: connection closed")

// Connection wraps a raw net.Conn with tracking metadata and safe atomic closing mechanisms.
// It acts as the primary state machine for a single client session, managing the bidirectional
// flow of protocol.Packet frames and handling internal backpressure signals invisibly
// to the application layer.
type Connection struct {
	// Conn embeds the raw TCP socket interface.
	net.Conn

	id         string
	createdAt  time.Time
	closeOnce  sync.Once
	closedChan chan struct{}

	// SendChan acts as the thread-safe outbound mailbox.
	// The background WriteLoop pulls packets from here and marshals them into raw wire frames.
	SendChan chan *protocol.Packet

	// FlowControlChan acts as the direct brake pedal for the WriteLoop.
	// It carries the 1-byte state flags (e.g., protocol.WindowStatePause) to enforce
	// internal engine backpressure without dropping the TCP connection.
	FlowControlChan chan byte

	// Pre-allocated scratchpads mapped to the connection lifecycle to completely
	// eliminate header heap allocations during high-throughput Read/Write operations.
	readScratch  [protocol.HeaderSize]byte
	writeScratch [protocol.HeaderSize]byte
}

// NewConnection initializes a monitored connection wrapper with pre-allocated
// control channels and zero-allocation scratch buffers.
func NewConnection(id string, conn net.Conn) *Connection {
	return &Connection{
		Conn:       conn,
		id:         id,
		createdAt:  time.Now(),
		closedChan: make(chan struct{}),
		// Buffered to allow up to 100 type-validated packets to queue before blocking the sender.
		SendChan: make(chan *protocol.Packet, 100),
		// Buffered to 10 to ensure the engine can rapidly shift states without blocking.
		FlowControlChan: make(chan byte, 10),
	}
}

// ID returns the unique string identifier for this client stream.
func (c *Connection) ID() string {
	return c.id
}

// Close ensures the underlying TCP socket and associated OS file descriptors
// are terminated safely exactly once. It immediately broadcasts a shutdown signal
// to all internal goroutines monitoring the Done channel.
func (c *Connection) Close() error {
	var err error
	c.closeOnce.Do(func() {
		close(c.closedChan)
		err = c.Conn.Close()
	})
	return err
}

// Done returns a receive-only channel that is closed when the connection is terminated.
// This provides a thread-safe hook for external subsystems to monitor the lifecycle state.
func (c *Connection) Done() <-chan struct{} {
	return c.closedChan
}

// ReadLoop continuously polls the underlying socket for incoming frames.
// It enforces idle timeouts, intercepts internal engine control frames (like WindowUpdate),
// and routes validated application payloads to the provided inbound channel.
// This function blocks and should be executed in its own goroutine.
func (c *Connection) ReadLoop(packetInboundChan chan<- *protocol.Packet, idleTimeout time.Duration) {
	defer c.Close()

	for {
		if idleTimeout > 0 {
			if err := c.Conn.SetReadDeadline(time.Now().Add(idleTimeout)); err != nil {
				return
			}
		}

		packet, err := protocol.UnmarshalPacket(c.Conn, c.readScratch[:])
		if err != nil {
			switch {
			case errors.Is(err, io.EOF):
				return
			case errors.Is(err, protocol.ErrPayloadTooLarge):
				// Drops the connection instantly to prevent an OOM panic via buffer expansion.
				return
			case errors.Is(err, protocol.ErrInvalidPacket):
				return
			default:
				return
			}
		}

		// Intercept reserved internal engine control frames.
		if packet.Type == protocol.TypeWindowUpdate {
			if len(packet.Value) > 0 {
				stateSignal := packet.Value[0]
				select {
				case c.FlowControlChan <- stateSignal:
				case <-c.closedChan:
					return
				}
			}
			continue
		}

		select {
		case packetInboundChan <- packet:
		case <-c.closedChan:
			return
		}
	}
}

// WriteLoop continuously monitors the SendChan for outbound application packets.
// It dynamically responds to application-layer flow control, actively pausing or resuming
// packet serialization when signaled via the FlowControlChan.
// This function blocks and should be executed in its own goroutine.
func (c *Connection) WriteLoop(writeTimeout time.Duration) {
	defer c.Close()

	activeSendChan := c.SendChan

	for {
		select {
		case <-c.closedChan:
			return

		case state := <-c.FlowControlChan:
			if state == protocol.WindowStatePause {
				activeSendChan = nil // Disables the case below, enforcing backpressure.
				fmt.Printf("[Flow Control] %s triggered BACKPRESSURE PAUSE.\n", c.id)
			} else if state == protocol.WindowStateResume {
				activeSendChan = c.SendChan // Restores pointer, lifting backpressure.
				fmt.Printf("[Flow Control] %s triggered BACKPRESSURE RESUME.\n", c.id)
			}

		case packet, ok := <-activeSendChan:
			if !ok {
				return
			}

			if writeTimeout > 0 {
				if err := c.Conn.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil {
					return
				}
			}

			if err := packet.Marshal(c.Conn, c.writeScratch[:]); err != nil {
				return
			}
		}
	}
}

// Dial connects to a remote Telesect server over TCP and initializes a full-duplex Connection.
// It automatically spins up the background WriteLoop to allow immediate, asynchronous outbound transmission.
func Dial(addr string) (*Connection, error) {
	rawConn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("transport: failed to dial %s: %w", addr, err)
	}

	// generateID() must be defined in the transport package scope.
	id, err := generateID()
	if err != nil {
		rawConn.Close()
		return nil, err
	}

	conn := NewConnection(id, rawConn)
	go conn.WriteLoop(30 * time.Second)

	return conn, nil
}
