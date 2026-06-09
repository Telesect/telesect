// Package transport provides the core TCP socket lifecycle, flow control,
// and state management for the Telesect communication backbone.
package transport

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"sync"
	"time"

	"telesect/pkg/protocol"
)

// Server handles listening for and orchestrating incoming raw TCP streams.
// It maintains a thread-safe registry of active client connections and
// actively monitors global memory saturation to enforce engine backpressure.
type Server struct {
	addr     string
	listener net.Listener
	shutdown chan struct{}
	wg       sync.WaitGroup

	// mu protects the connections map from concurrent read/write panics
	// during high-throughput client registration and teardown.
	mu          sync.RWMutex
	connections map[string]*Connection
	isShutdown  bool

	// InboundRouter is the centralized master switchboard. All isolated client
	// ReadLoops push their validated, zero-allocation application packets here.
	InboundRouter chan *protocol.Packet
}

// NewServer initializes an unstarted server instance configured for the provided
// network address. It pre-allocates the internal connection registry and provisions
// a high-capacity master routing channel for backpressure absorption.
func NewServer(addr string) *Server {
	return &Server{
		addr:          addr,
		shutdown:      make(chan struct{}),
		connections:   make(map[string]*Connection),
		InboundRouter: make(chan *protocol.Packet, 1000),
	}
}

// Start opens the network listener and asynchronously spawns the connection
// acceptance loop and the global backpressure watchdog.
func (s *Server) Start() error {
	l, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("transport: failed to bind address %s: %w", s.addr, err)
	}
	s.listener = l

	s.wg.Add(2)
	go s.acceptLoop()
	go s.monitorBackpressure()

	return nil
}

// acceptLoop continuously polls the OS listener for incoming connection events.
// It gracefully exits when the server's master shutdown channel is closed.
func (s *Server) acceptLoop() {
	defer s.wg.Done()

	for {
		rawConn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.shutdown:
				return
			default:
				fmt.Printf("[Server Error] Ingress accept failure: %v\n", err)
				continue
			}
		}

		s.wg.Add(1)
		go s.handleRawStream(rawConn)
	}
}

// handleRawStream manages the full-duplex lifecycle of a newly accepted data stream.
// It registers the connection, spins up an asynchronous WriteLoop, and blocks the
// current goroutine with the ReadLoop until the connection terminates.
func (s *Server) handleRawStream(rawConn net.Conn) {
	defer s.wg.Done()

	id, _ := generateID()
	conn := NewConnection(id, rawConn)

	s.register(conn)

	// LIFO cleanup: Ensure connection is closed before deregistering from map.
	defer s.deregister(conn.ID())
	defer conn.Close()

	fmt.Printf("[Server Context] Inbound socket assigned identity: %s (Remote: %s)\n", conn.ID(), conn.RemoteAddr())

	go conn.WriteLoop(30 * time.Second)
	conn.ReadLoop(s.InboundRouter, 5*time.Minute)
}

// register safely attaches an active Connection to the server registry
// using an exclusive write lock.
func (s *Server) register(c *Connection) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.connections[c.ID()] = c
	fmt.Printf("[Registry Active] Client %s mapped securely to internal switchboard.\n", c.ID())
}

// deregister safely purges a connection from the server registry by its identifier,
// allowing the garbage collector to reclaim the tracking structure.
func (s *Server) deregister(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.connections[id]; exists {
		delete(s.connections, id)
		fmt.Printf("[Registry Idle] Client %s scrubbed completely from switchboard.\n", id)
	}
}

// Get performs a thread-safe lookup of an active connection by its unique identifier.
// It uses a shared read lock to allow high-throughput concurrent access.
func (s *Server) Get(id string) *Connection {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.connections[id]
}

// Stop cleanly terminates the server. It closes the port listener, signals all loops
// to halt, force-evicts any remaining active client sockets, and waits for all
// background goroutines to safely drain.
func (s *Server) Stop() error {
	s.mu.Lock()
	if s.isShutdown {
		s.mu.Unlock()
		return nil
	}
	s.isShutdown = true
	close(s.shutdown)
	s.mu.Unlock()

	var err error
	if s.listener != nil {
		err = s.listener.Close()
	}

	s.mu.Lock()
	for id, conn := range s.connections {
		_ = conn.Close()
		delete(s.connections, id)
	}
	s.mu.Unlock()

	s.wg.Wait()
	return err
}

func generateID() (string, error) {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// monitorBackpressure continuously reviews the saturation of the InboundRouter.
// If the buffer exceeds 80% capacity, it issues a broadcast pause frame to throttle clients.
// Once the buffer drains below 20%, it issues a resume frame.
func (s *Server) monitorBackpressure() {
	defer s.wg.Done()

	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()

	var isPaused bool

	for {
		select {
		case <-s.shutdown:
			return
		case <-ticker.C:
			currentLength := len(s.InboundRouter)

			if currentLength >= 800 && !isPaused {
				isPaused = true
				fmt.Printf("[ALARM] InboundRouter saturated (%d/1000). Broadcasting PAUSE to all nodes.\n", currentLength)
				s.broadcastFlowControl(protocol.WindowStatePause)
			}

			if currentLength <= 200 && isPaused {
				isPaused = false
				fmt.Printf("[CLEAR] InboundRouter drained (%d/1000). Broadcasting RESUME to all nodes.\n", currentLength)
				s.broadcastFlowControl(protocol.WindowStateResume)
			}
		}
	}
}

// broadcastFlowControl forces an internal control packet into the send buffer
// of every active connection, enforcing global engine states.
func (s *Server) broadcastFlowControl(state byte) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	controlPacket, err := protocol.NewPacket(protocol.TypeWindowUpdate, []byte{state})
	if err != nil {
		return
	}

	for _, conn := range s.connections {
		select {
		case conn.SendChan <- controlPacket:
		default:
		}
	}
}
