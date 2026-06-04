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
// ─── ANALOGY ───
// This represents the main Automated Logistics Hub Facility. It holds the configurations
// for the facility gates, the tracking clipboards, and now our central Registry Map Directory.
type Server struct {
	addr     string         // Technical: Network IP:Port binding string. Analogy: The street address of the warehouse.
	listener net.Listener   // Technical: Underlying OS socket server gateway. Analogy: The automated physical shipping gates.
	shutdown chan struct{}  // Technical: Central control coordination context. Analogy: The facility's master power breaker.
	wg       sync.WaitGroup // Technical: Atomic concurrency reference tracking. Analogy: The Supervisor's digital clipboard.

	// ─── NEW REGISTRY LAYER ───
	mu          sync.RWMutex           // Technical: Asymmetric read/write lock protecting the registry map. Analogy: Master lock box on the registry directory.
	connections map[string]*Connection // Technical: Thread-unsafe map mapping client IDs to connection pointers. Analogy: The central directory board.
	isShutdown  bool                   // Technical: Flag preventing duplicate operations. Analogy: State sign displaying "Closed".

	// ─── NEW MASTER SWITCHBOARD ───
	// Technical: The centralized channel where all isolated ReadLoops push their validated packets.
	// Analogy: The main facility conveyor belt where all trucks dump their unloaded, inspected cargo.
	InboundRouter chan *protocol.Packet
}

// NewServer initializes a server instance attached to an address with an active registry map.
func NewServer(addr string) *Server {
	return &Server{
		addr:          addr,
		shutdown:      make(chan struct{}),
		connections:   make(map[string]*Connection),      // Technical: Must initialize map memory space to avoid nil panics.
		InboundRouter: make(chan *protocol.Packet, 1000), // Buffer size of 1000 provides massive backpressure relief
	}
}

// Start opens the network port and enters an asynchronous loop accepting incoming streams.
func (s *Server) Start() error {
	l, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("transport: failed to bind address %s: %w", s.addr, err)
	}
	s.listener = l

	s.wg.Add(1)
	go s.acceptLoop()

	return nil
}

// acceptLoop polls the OS port for incoming connection events.
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

// handleRawStream manages the lifetime of a data stream.
// ─── TECHNICAL ───
// Upgraded to full-duplex operation. It forks the execution path into dedicated read/write goroutines,
// ensuring the server can transmit multiplexed telemetry while simultaneously ingesting massive payloads.
func (s *Server) handleRawStream(rawConn net.Conn) {
	// Let the master wait group know when this supervisor officially exits
	defer s.wg.Done()

	id, _ := generateID()
	conn := NewConnection(id, rawConn)

	s.register(conn)

	// ─── LIFECYCLE PIPELINE ───
	// Defers execute in reverse order (LIFO).
	// 1st: conn.Close() guarantees the OS file descriptor is destroyed and channels are closed.
	// 2nd: s.deregister() completely scrubs the connection from the master registry map.
	defer s.deregister(conn.ID())
	defer conn.Close()

	fmt.Printf("[Server Context] Inbound socket assigned identity: %s (Remote: %s)\n", conn.ID(), conn.RemoteAddr())

	// Spin up the Loader on its own isolated background thread
	go conn.WriteLoop(30 * time.Second)

	// Run the Unloader on this current thread.
	// This function will block here infinitely until the network disconnects or faults.
	conn.ReadLoop(s.InboundRouter, 5*time.Minute)
}

// ─── NEW THREAD-SAFE REGISTRY METHODS ───

// register safely attaches an active connection to the server directory map.
// ─── TECHNICAL ───
// Acquires an exclusive Write Lock (Lock). This blocks any concurrent reads or writes from touching the
// map while the pointer is being safely added to the underlying memory bucket array.
func (s *Server) register(c *Connection) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.connections[c.ID()] = c
	fmt.Printf("[Registry Active] Client %s mapped securely to internal switchboard.\n", c.ID())
}

// deregister safely purges a connection from the server directory map by its identifier.
// ─── TECHNICAL ───
// Acquires an exclusive Write Lock (Lock). Purging the key allows the Go Garbage Collector to safely
// sweep up and reclaim the memory allocated for that connection tracking structure.
func (s *Server) deregister(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.connections[id]; exists {
		delete(s.connections, id)
		fmt.Printf("[Registry Idle] Client %s scrubbed completely from switchboard.\n", id)
	}
}

// Get safe fetches an active connection pointer from the registry map.
// ─── TECHNICAL ───
// Acquires a shared Read Lock (RLock). Hundreds of worker threads can invoke Get() simultaneously because
// reading a memory pointer does not modify the structural map alignment, allowing high-throughput routing.
func (s *Server) Get(id string) *Connection {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.connections[id]
}

// Stop cleanly closes the port listener, terminates all active registry connections, and halts loops.
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

	// ─── NEW DEFENSIVE DRAIN BLOCK ───
	// Technical: Acquire a write lock to safely iterate and force-evict any straggler client sockets
	// currently stuck in deep read blocks during server shutdown.
	// Analogy: The director loops through remaining bays, handing pink slips directly to remaining trucks.
	s.mu.Lock()
	for id, conn := range s.connections {
		_ = conn.Close() // Forces conn.Read() blocks inside handleRawStream to instantly wake up and exit
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
