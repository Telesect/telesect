package transport

import (
	"errors"
	"net"
	"sync"
	"time"
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

	// ─── NEW: DECOUPLED I/O STATE ───
	SendChan chan *Frame // Technical: The outbound mailbox. The Loader (writeLoop) pulls from here.

}

// NewConnection initializes a monitored connection wrapper.
func NewConnection(id string, conn net.Conn) *Connection {
	return &Connection{
		Conn:       conn,
		id:         id,
		createdAt:  time.Now(),
		closedChan: make(chan struct{}),
		SendChan:   make(chan *Frame, 100), // Buffered to allow up to 100 frames to queue before blocking the sender.
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
