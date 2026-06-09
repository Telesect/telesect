
# Flow Control & Backpressure Strategy

Telesect is designed to handle high-throughput, multiplexed streams without allowing slow consumers or sudden traffic spikes to cause memory exhaustion (OOM). Instead of relying blindly on TCP's OS-level window sizing, Telesect implements an **Asymmetric application-layer backpressure ring-fence**.

---

## The Asymmetric Watermark Strategy

The central `Server` orchestrator routes all incoming traffic through a buffered channel known as the `InboundRouter`. This channel has a hard cap of **1,000 frames**. To avoid rapid state-chattering (frequently toggling throttling on and off due to minor fluctuations), the system operates on a dual-threshold asymmetric water-mark system.


```

```
   InboundRouter Channel Capacity (1,000 Packets)

```

┌──────────────────────────────────────────────────────────┐
│████████████████████████████████████████        │         │
└────────────────────────────────────────┬───────┴─────────┘
│
[High-Water Mark: 80% (800)]
Triggers: TypeWindowUpdatePause (0x00)

```

| Saturation State | Threshold | Protocol Action | Engine Behavioral Consequence |
| :--- | :--- | :--- | :--- |
| **High-Water Mark** | **80% Capacity** (800 Frames) | Dispatches `TypeWindowUpdatePause` (`0x00`) | The server signals all active clients to pause transmission immediately. |
| **Low-Water Mark** | **20% Capacity** (200 Frames) | Dispatches `TypeWindowUpdateResume` (`0x01`) | The buffer has safely cleared; the server signals clients that they are clear to resume. |

---

## Throttling Mechanics: The Go Channel Pointer-Swap

When a client connection receives a `TypeWindowUpdatePause` frame from the server, it halts transmission without tearing down the underlying TCP connection or causing socket timeouts. It accomplishes this via a highly optimized, lock-free Go concurrency idiom: **Pointer-Swapping to `nil`**.

Inside the client's localized multiplexing loop, a `select` block coordinates outgoing packets:

```go
// Normal Operations: activeSendChan points to the real Outbox channel
var activeSendChan chan *protocol.Packet = c.SendChan

for {
    select {
    case pkt := <-activeSendChan:
        c.writeFrame(pkt)
    case ctrl := <-c.ControlChan:
        if ctrl.Type == protocol.TypeWindowUpdatePause {
            // THE SWAP: Setting a channel to nil makes it block permanently in a select block
            activeSendChan = nil 
        } else if ctrl.Type == protocol.TypeWindowUpdateResume {
            // RE-ENGAGE: Restore the pointer to resume writing
            activeSendChan = c.SendChan
        }
    }
}

```

### Why this is mathematically and architecturally elegant:

1. **Zero CPU Overhead:** Reading from or writing to a `nil` channel in Go blocks *permanently* without consuming any CPU cycles. The runtime scheduler simply parks the goroutine.
2. **No Spin-Locks:** We avoid complex, error-prone mutex locks or spinning loops that burn CPU cycles while waiting for space to open up.
3. **TCP Buffer Safety:** The kernel's standard TCP buffers remain completely safe, and frames currently in flight are preserved.

---

## Graceful Descriptor Reclamation

Network sockets are finite Operating System resources. If a client goes rogue, drops offline abruptly, or intentionally violates protocol contracts, Telesect enforces immediate, defensive resource cleanup.

When an abnormal event or transport error occurs, the local `Connection` initiates a strict teardown sequence:

1. **State Lockout:** The connection state is atomically marked as closed, preventing new payloads from being queued.
2. **Drain Outbox:** Any remaining outbound packets in the queue are safely discarded to prevent goroutine leaks.
3. **Socket Close:** The raw `net.Conn` descriptor is explicitly closed, immediately returning the file descriptor back to the operating system kernel.
4. **Buffer Return:** The internal 5-byte scratchpad arrays and payload slices are scrubbed and returned to the system's global memory allocation pool (`sync.Pool`), instantly neutralizing heap allocation pressure.

