# API & Package Reference

Telesect is built as a modular set of internal primitives with zero external core dependencies. This document maps our logical architecture to the physical Go package design, providing a guide for digging into the codebase.

---

## Package Map

| Package | Purpose | Core Structural Components |
| :--- | :--- | :--- |
| `pkg/protocol` | Serialization & Frame Spec | `Packet`, `Header`, Payload Encoders |
| `internal/transport` | Socket I/O & Connection Lifecycle | `Server`, `Connection`, `Listener` |

---

## Codebase Component Reference

### 1. Wire Framing Protocol (`pkg/protocol`)
The foundational layer defining the binary format sent across the wire. It uses an ultra-low allocation Type-Length-Value (TLV) layout optimized for speed and safety.

* **Key Source Files:** `protocol.go`
* **What to look for:** Look at how the buffer pools scratchpad memory to prevent the Go garbage collector from triggering under heavy transmission loads.

### 2. Transport Infrastructure (`internal/transport`)
The execution engine that drives the socket layer, coordinates worker pools, and implements systemic flow-control.

* **Key Source Files:** `server.go`, `listener.go`, `connection.go`
* **What to look for:** Check out the background `ReadLoop` and `WriteLoop` routines in `connection.go` to see how full-duplex transmission is decoupled using non-blocking Go channels.

---

## Accessing Native GoDoc (Recommended)

While MkDocs hosts our high-level architecture maps and design patterns, the authoritative, line-by-line low-level code documentation is served natively via Go's document utility.

Because Telesect uses private and unpublished local Go modules, avoid using `pkgsite` which requires internet-facing proxies. Instead, run the lightweight, native `godoc` tool locally.

### 1. Spin up the Local GoDoc Server
From the root of the `telesect` project directory, execute the following command in an independent terminal window:

```bash
godoc -http=:6060