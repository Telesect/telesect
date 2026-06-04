package transport

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
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
}

// NewServer initializes a server instance attached to an address with an active registry map.
func NewServer(addr string) *Server {
	return &Server{
		addr:        addr,
		shutdown:    make(chan struct{}),
		connections: make(map[string]*Connection), // Technical: Must initialize map memory space to avoid nil panics.
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
	go s.writeLoop(conn)

	// Run the Unloader on this current thread.
	// This function will block here infinitely until the network disconnects or faults.
	s.readLoop(conn)
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

// writeLoop acts as the dedicated Loader.
// ─── TECHNICAL ───
// It listens on the internal SendChan for outbound payloads and multiplexes them onto the raw socket.
// It uses a select statement to listen for the shutdown broadcast simultaneously.
func (s *Server) writeLoop(conn *Connection) {
	// Technical: If the write loop exits for any reason, trigger the safety clamp.
	defer conn.Close()

	for {
		select {
		case <-conn.Done():
			// Analogy: The alarm siren went off. Stop loading and exit the facility.
			return
		case frame := <-conn.SendChan:
			// Analogy: A package arrived on the internal conveyor belt. Push it out to the truck.
			if err := frame.Marshal(conn); err != nil {
				fmt.Printf("[Write Fault] Client %s: %v\n", conn.ID(), err)
				return
			}
		}
	}
}

// readLoop acts as the dedicated Unloader.
// ─── TECHNICAL ───
// It continuously polls the network socket for inbound binary frames. Because socket reads are blocking,
// separating this ensures outbound telemetry or data can still flow even if the client goes quiet.
func (s *Server) readLoop(conn *Connection) {
	// Technical: If the unloader hits a network drop or parsing error, trigger the safety clamp.
	defer conn.Close()

	for {
		// Enforce the 5-minute inactivity timeout.
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Minute))

		frame, err := UnmarshalRead(conn)
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
				fmt.Printf("[Server Context] Connection cleanly torn down for client: %s\n", conn.ID())
				return
			}
			fmt.Printf("[Server Context] Protocol framing violation or error on client %s: %v\n", conn.ID(), err)
			return
		}

		// For Milestone 1.3, we simply log the successful extraction.
		// In Phase 2, we will route this frame into the TUI Ring Buffers.
		fmt.Printf("[Protocol Ingress] Stream: %d | Type: 0x%02x | Sizing: %d bytes | Source: %s\n",
			frame.StreamID, frame.Type, frame.Length, conn.ID())
		if frame.Length > 0 {
			fmt.Printf("[Payload Content] -> %s\n", string(frame.Payload))
		}
	}
}
