# The Protocol Schema

Telesect utilizes a streamlined, binary Type-Length-Value (TLV) frame layout designed for low parsing overhead and strict deterministic memory bounds. All multi-byte integers are transmitted in network byte order (**Big-Endian**).

---

## The 5-Byte Wire Header

Every frame crossing the network boundary begins with an immutable, fixed-size 5-byte header. This uniform layout ensures that the transport engine's read routine (`ReadLoop`) can extract exactly 5 bytes into a local stack-allocated scratchpad before making any heap allocations for the payload.

| Byte Offset | Field | Data Type | Description |
| :--- | :--- | :--- | :--- |
| **Byte 0** | `Type` | `uint8` (1 Byte) | Defines the command intent or payload category. |
| **Bytes 1–4** | `Length` | `uint32` (4 Bytes) | The exact size of the following payload in bytes. |

---

## Internal Control Frames (`0x00` to `0x0F`)

The protocol reserves the lower hex range (`0x00` through `0x0F`) exclusively for engine-level control mechanisms. These frames handle connectivity lifecycles and application-layer flow adjustment. They are intercepted directly by the `Connection` state machine and are *not* routed to the global application layer.

### Reserved Frame Glossary

| Hex Identifier | Frame Name | Payload Expectation | System Behavioral Response |
| :--- | :--- | :--- | :--- |
| **`0x00`** | `TypeWindowUpdatePause` | Zero bytes (`Length = 0`) | Tells the receiver to immediately halt transmission. |
| **`0x01`** | `TypeWindowUpdateResume` | Zero bytes (`Length = 0`) | Tells the receiver that the buffer has cleared; resumes transmission. |
| **`0x02`** | `TypePing` | Optional timestamp | Connection verification probe; requires an immediate response. |
| **`0x03`** | `TypePong` | Mirrors incoming Ping payload | Acknowledges connection liveness. |
| **`0x04`** | `TypeDisconnectGoAway` | Optional error code string | Signal indicating clean session teardown; drops descriptor immediately after. |

---

## Application Data Frames (`0x10` and Above)

Any frame matching an identifier of **`0x10` or higher** is classified as a standard application payload. The core Telesect backbone treats these frames as completely opaque chunks of data. It parses the envelope and forwards the raw bytes straight onto the `InboundRouter` channel for consumption by higher-level modules (like your Bubble Tea TUIs or telemetry trackers).

---

## Safety Caps & Allocation Safeguards

To defend against Malicious Frame Injection and DoS vector attacks (such as a rogue client writing an artificial `Length` value of $2^{32}-1$ to force a massive memory allocation panic), the framework enforces a strict hard ceiling:

!!! danger "Maximum Payload Envelope"
    The maximum allowable payload size for a single frame is strictly limited to **4 Megabytes (4,194,304 bytes)**. 

If an incoming frame header declares a `Length` element exceeding this boundary, the engine drops the connection automatically, releases the pooled buffers, and lists a resource violation error in the local security logs to keep the server completely isolated from memory exhaustion.