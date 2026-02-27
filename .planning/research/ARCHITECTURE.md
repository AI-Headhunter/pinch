# Architecture Research

**Domain:** Encrypted agent-to-agent relay messaging (E2E encrypted, store-and-forward, WebSocket-based)
**Researched:** 2026-02-26
**Confidence:** HIGH

## Standard Architecture

### System Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Agent Layer (Clients)                        │
│  ┌──────────────┐  ┌──────────────┐          ┌──────────────┐      │
│  │  Agent A      │  │  Agent B      │   ...    │  Agent N      │    │
│  │  (OpenClaw    │  │  (OpenClaw    │          │  (OpenClaw    │    │
│  │   Skill)      │  │   Skill)      │          │   Skill)      │    │
│  └──────┬───────┘  └──────┬───────┘          └──────┬───────┘    │
│         │                  │                         │            │
├─────────┴──────────────────┴─────────────────────────┴────────────┤
│                        Crypto Layer (Client-Side)                  │
│  ┌─────────────────────────────────────────────────────────────┐  │
│  │  Keypair Manager | Encryption Engine | Connection Manager   │  │
│  │  (Ed25519/X25519)  (NaCl box/secretbox) (Trust + Autonomy) │  │
│  └─────────────────────────────────────────────────────────────┘  │
├───────────────────────────────────────────────────────────────────┤
│                        Transport Layer (WebSocket)                 │
│  ┌─────────────────────────────────────────────────────────────┐  │
│  │  WS Connection | Heartbeat | Reconnection | Auth Handshake  │  │
│  └─────────────────────────────────────────────────────────────┘  │
├───────────────────────────────────────────────────────────────────┤
│                        Relay Server (Go)                          │
│  ┌───────────┐  ┌───────────┐  ┌───────────┐  ┌───────────┐    │
│  │ WS Hub    │  │ Router    │  │ Message   │  │ Auth      │    │
│  │ (Conn     │  │ (Address  │  │ Queue     │  │ (Keypair  │    │
│  │  Mgmt)    │  │  Lookup)  │  │ (Store &  │  │  Verify)  │    │
│  │           │  │           │  │  Forward) │  │           │    │
│  └───────────┘  └───────────┘  └───────────┘  └───────────┘    │
├───────────────────────────────────────────────────────────────────┤
│                        Storage Layer (Relay-Side)                  │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐                       │
│  │ Message  │  │ Conn     │  │ Channel  │                       │
│  │ Store    │  │ Registry │  │ Metadata │                       │
│  │ (queued  │  │ (online  │  │ (members │                       │
│  │  blobs)  │  │  state)  │  │  only)   │                       │
│  └──────────┘  └──────────┘  └──────────┘                       │
└───────────────────────────────────────────────────────────────────┘
```

### Component Responsibilities

| Component | Responsibility | Typical Implementation |
|-----------|----------------|------------------------|
| **Keypair Manager** (client) | Generate, store, export Ed25519 keypairs; derive X25519 for encryption; manage `pinch:<hash>@<relay>` identity | libsodium keygen, local encrypted keystore |
| **Encryption Engine** (client) | Encrypt/decrypt messages with NaCl box (1:1) and secretbox (group shared key); sign messages with Ed25519 | NaCl crypto_box for 1:1, shared symmetric key + secretbox for groups |
| **Connection Manager** (client) | Manage connection requests, approval/rejection, blocking, muting, autonomy levels, permissions manifest | Local state machine per contact; persisted connection list |
| **WS Hub** (relay) | Accept WebSocket connections, maintain connection map (address -> socket), detect disconnects, handle heartbeat/ping | Go goroutine-per-connection with hub pattern; in-memory map |
| **Router** (relay) | Resolve destination address, route encrypted blobs to correct connection or queue | Address lookup in connection registry; fan-out for group channels |
| **Message Queue** (relay) | Store encrypted blobs for offline recipients, deliver on reconnect, TTL-based expiration | Persistent queue (BoltDB/SQLite/Redis); 7-day TTL like Signal |
| **Auth** (relay) | Verify client identity via Ed25519 signature challenge on connect; no passwords, no tokens | Challenge-response: relay sends nonce, client signs with Ed25519 private key |
| **Message Store** (relay) | Persist queued encrypted blobs durably | Embedded DB (BoltDB/SQLite) for single-instance; Redis for multi-instance |
| **Connection Registry** (relay) | Track which addresses are currently online | In-memory map; Redis pub/sub for multi-instance |
| **Channel Metadata** (relay) | Track channel membership (who can receive fan-out); relay sees member list but not message content | Minimal metadata: channel ID -> list of member addresses |

## Recommended Project Structure

```
pinch/
├── relay/                      # Go relay server
│   ├── cmd/
│   │   └── pinchd/             # Server binary entry point
│   │       └── main.go
│   ├── internal/
│   │   ├── hub/                # WebSocket hub (connection management)
│   │   │   ├── hub.go          # Central hub: register/unregister/route
│   │   │   └── client.go       # Per-connection state, readPump/writePump
│   │   ├── auth/               # Ed25519 challenge-response authentication
│   │   │   └── challenge.go
│   │   ├── router/             # Message routing (address resolution, fan-out)
│   │   │   └── router.go
│   │   ├── queue/              # Store-and-forward message queue
│   │   │   ├── queue.go        # Queue interface
│   │   │   └── bolt.go         # BoltDB implementation
│   │   ├── protocol/           # Wire protocol (message types, envelope format)
│   │   │   └── envelope.go
│   │   └── store/              # Persistent storage abstractions
│   │       └── store.go
│   ├── go.mod
│   └── go.sum
├── skill/                      # TypeScript OpenClaw skill
│   ├── SKILL.md                # OpenClaw skill definition
│   ├── src/
│   │   ├── crypto/             # Keypair management, encryption/decryption
│   │   │   ├── keypair.ts      # Ed25519 keygen, X25519 derivation
│   │   │   ├── encrypt.ts      # NaCl box (1:1), secretbox (group)
│   │   │   └── identity.ts     # pinch:<hash>@<relay> address format
│   │   ├── connection/         # Connection request model, trust management
│   │   │   ├── manager.ts      # Connection lifecycle (request/accept/reject/block)
│   │   │   ├── autonomy.ts     # 4-tier autonomy levels
│   │   │   └── permissions.ts  # Inbound permissions manifest
│   │   ├── transport/          # WebSocket client, heartbeat, reconnection
│   │   │   ├── ws.ts           # WebSocket connection management
│   │   │   └── heartbeat.ts    # OpenClaw heartbeat cycle integration
│   │   ├── channels/           # 1:1 and group channel logic
│   │   │   ├── direct.ts       # 1:1 encrypted channel
│   │   │   └── group.ts        # Group channel (shared key management)
│   │   ├── tools/              # Outbound OpenClaw tools
│   │   │   ├── send.ts         # Send message tool
│   │   │   ├── connect.ts      # Manage connections tool
│   │   │   └── history.ts      # Review message history tool
│   │   └── listener/           # Inbound message processing
│   │       └── handler.ts      # Process incoming messages/requests
│   ├── package.json
│   └── tsconfig.json
├── protocol/                   # Shared protocol definitions
│   └── messages.md             # Wire format documentation
└── .planning/                  # Project planning
```

### Structure Rationale

- **relay/internal/**: Go convention for unexported packages. Each subdirectory owns a single concern. The hub pattern (Signal, gorilla/websocket examples) is the standard Go WebSocket architecture.
- **skill/src/crypto/**: Isolates all cryptographic operations. Nothing outside this directory touches raw keys or NaCl primitives.
- **skill/src/connection/**: Separates trust management (connection requests, autonomy, permissions) from transport and crypto. This is Pinch's unique value -- it deserves its own boundary.
- **skill/src/transport/**: WebSocket and heartbeat logic isolated from business logic. Can be tested/mocked independently.
- **protocol/**: Shared wire format documentation. Go and TypeScript must agree on envelope format; keeping the spec in one place prevents drift.

## Architectural Patterns

### Pattern 1: Cryptographically Blind Relay

**What:** The relay server never possesses private keys, never decrypts message content, and routes only opaque encrypted blobs. All encryption/decryption happens client-side. The relay sees: sender address, recipient address, encrypted blob, timestamp.
**When to use:** Always -- this is the core security guarantee.
**Trade-offs:** The relay cannot inspect content for spam filtering, rate limiting based on content, or server-side search. Rate limiting must be per-sender/per-connection instead. Worth it: this is what makes Pinch trustworthy.

**How Signal does it:** Signal server stores encrypted message blobs in Redis/DynamoDB with a 7-day TTL. Server code is "quite simple" because it never processes message content -- just routes bytes. ([Source: SoftwareMill analysis](https://softwaremill.com/what-ive-learned-from-signal-server-source-code/))

**How Matrix does it:** Matrix homeservers *do* see event metadata and room state, but with E2E encryption enabled (Megolm), message bodies are encrypted blobs. Pinch should follow Signal's stricter model -- the relay should see *less* metadata than Matrix homeservers do.

### Pattern 2: Hub-and-Spoke WebSocket Management (Go)

**What:** A central Hub goroutine manages a map of connected clients. Each client connection spawns two goroutines: readPump (reads from WebSocket, sends to Hub) and writePump (reads from Hub, writes to WebSocket). The Hub serializes all register/unregister/route operations through a single goroutine with channels.
**When to use:** Standard pattern for any Go WebSocket server. Used by gorilla/websocket examples, adopted widely.
**Trade-offs:** Single Hub goroutine can become a bottleneck at extreme scale (100K+ concurrent connections). For Pinch v1, this is a non-issue. If needed later, shard the Hub by address prefix.

**Example (Go):**
```go
type Hub struct {
    clients    map[string]*Client  // address -> client
    register   chan *Client
    unregister chan *Client
    route      chan *Envelope
}

func (h *Hub) Run() {
    for {
        select {
        case client := <-h.register:
            h.clients[client.Address] = client
            h.deliverQueued(client) // flush stored messages
        case client := <-h.unregister:
            delete(h.clients, client.Address)
        case env := <-h.route:
            if client, ok := h.clients[env.To]; ok {
                client.send <- env.Payload // online: deliver immediately
            } else {
                h.queue.Store(env) // offline: store for later
            }
        }
    }
}
```

### Pattern 3: Challenge-Response Authentication

**What:** On WebSocket connect, the relay sends a random nonce. The client signs the nonce with their Ed25519 private key and sends back the signature + public key. The relay verifies the signature, derives the `pinch:<hash>@<relay>` address from the public key, and registers the connection. No passwords, no tokens, no sessions.
**When to use:** When identity is keypair-based (like Pinch). Signal uses phone-number-based auth; Pinch uses Ed25519 keypairs directly.
**Trade-offs:** No account recovery if private key is lost. This is acceptable for agent identities (agents can generate new keypairs). If human-recoverable identity is needed later, add a key-backup mechanism.

### Pattern 4: Store-and-Forward with TTL

**What:** When a recipient is offline, the relay stores the encrypted blob in a persistent queue keyed by recipient address. When the recipient reconnects, queued messages are flushed in order. Messages expire after a TTL (Signal uses 7 days).
**When to use:** Whenever agents are not always online (which is the normal case for AI agents that spin up and down).
**Trade-offs:** Requires persistent storage at the relay. BoltDB (embedded, no external dependencies) is ideal for single-instance. TTL prevents unbounded storage growth. Queue must be durable across relay restarts.

### Pattern 5: Server-Side Fan-Out for Groups

**What:** For group channels, the sender encrypts the message once with a symmetric group key (NaCl secretbox), sends it to the relay, and the relay fans out the ciphertext to all group members. Each member decrypts with the shared group key.
**When to use:** Groups of 3+ members. For 1:1 channels, use NaCl box (asymmetric) directly.
**Trade-offs:** Requires secure distribution of group keys (use NaCl box to send the group key to each member individually). When a member is removed, the group key must be rotated and redistributed. Simpler than Signal's Sender Keys ratchet but sufficient for v1.

**Group key distribution flow:**
1. Group creator generates random symmetric key
2. Creator encrypts that key with each member's X25519 public key (NaCl box), sends individually
3. All messages in group encrypted with symmetric key (NaCl secretbox)
4. On member removal: creator generates new key, redistributes to remaining members

([Source: James Fisher - E2E encryption with server-side fan-out](https://jameshfisher.com/2017/10/25/end-to-end-encryption-with-server-side-fanout/))

## Data Flow

### Connection Request Flow

```
Agent A                          Relay                         Agent B
   │                               │                              │
   │  1. ConnectionRequest         │                              │
   │  {from, to, pubkey,           │                              │
   │   signed_payload}             │                              │
   │  ─────────────────────────►   │                              │
   │                               │  2. Route/Queue request      │
   │                               │  ─────────────────────────►  │
   │                               │                              │
   │                               │  3. ConnectionResponse       │
   │                               │  {accept|reject|block,       │
   │                               │   pubkey (if accept)}        │
   │                               │  ◄─────────────────────────  │
   │  4. Response delivered        │                              │
   │  ◄─────────────────────────   │                              │
   │                               │                              │
   │  [If accepted: both sides     │                              │
   │   now have each other's       │                              │
   │   public keys. Channel open.] │                              │
```

### 1:1 Message Flow (Online)

```
Agent A (sender)                 Relay                     Agent B (recipient)
   │                               │                              │
   │  1. Encrypt(plaintext,        │                              │
   │     B_pubkey, A_privkey)      │                              │
   │     → NaCl box ciphertext     │                              │
   │                               │                              │
   │  2. Envelope{to: B_addr,      │                              │
   │     from: A_addr,             │                              │
   │     payload: ciphertext,      │                              │
   │     type: "message"}          │                              │
   │  ─────── WebSocket ─────────► │                              │
   │                               │  3. Lookup B_addr in hub     │
   │                               │     B is online → deliver    │
   │                               │  ─────── WebSocket ─────────►│
   │                               │                              │
   │                               │                 4. Decrypt(  │
   │                               │                    ciphertext│
   │                               │                    A_pubkey, │
   │                               │                    B_privkey)│
   │                               │                    → plaintext
   │                               │                              │
   │                               │                 5. Process   │
   │                               │                    per       │
   │                               │                    autonomy  │
   │                               │                    level     │
```

### 1:1 Message Flow (Offline / Store-and-Forward)

```
Agent A (sender)                 Relay                     Agent B (offline)
   │                               │                              │
   │  1-2. Same as online flow     │                              │
   │  ─────── WebSocket ─────────► │                              │
   │                               │  3. Lookup B_addr in hub     │
   │                               │     B is OFFLINE → queue     │
   │                               │     Store(B_addr, envelope)  │
   │                               │     → persistent queue       │
   │                               │                              │
   │       ... time passes ...     │                              │
   │                               │                              │
   │                               │  4. B reconnects, auths     │
   │                               │  ◄─────── WebSocket ─────── │
   │                               │                              │
   │                               │  5. Flush queued messages    │
   │                               │     for B_addr, in order     │
   │                               │  ─────── WebSocket ─────────►│
   │                               │                              │
   │                               │  6. Delete delivered msgs    │
   │                               │     from queue               │
```

### Group Message Flow

```
Agent A (sender)                 Relay                     Agents B, C, D
   │                               │                              │
   │  1. Encrypt(plaintext,        │                              │
   │     group_symmetric_key)      │                              │
   │     → NaCl secretbox          │                              │
   │                               │                              │
   │  2. Envelope{to: channel_id,  │                              │
   │     from: A_addr,             │                              │
   │     payload: ciphertext,      │                              │
   │     type: "group_message"}    │                              │
   │  ─────── WebSocket ─────────► │                              │
   │                               │  3. Lookup channel members   │
   │                               │     Fan out to B, C, D       │
   │                               │     (queue if offline)       │
   │                               │  ──── WebSocket (each) ────► │
   │                               │                              │
   │                               │            4. Each decrypts  │
   │                               │               with shared    │
   │                               │               group key      │
```

### Authentication Flow (WebSocket Connect)

```
Client                           Relay
   │                               │
   │  1. WebSocket upgrade         │
   │  ─────────────────────────►   │
   │                               │
   │  2. Challenge{nonce: random}  │
   │  ◄─────────────────────────   │
   │                               │
   │  3. Authenticate{             │
   │     pubkey: Ed25519_pub,      │
   │     signature: Sign(nonce,    │
   │       Ed25519_priv)}          │
   │  ─────────────────────────►   │
   │                               │
   │  4. Verify signature          │
   │     Derive address from       │
   │     pubkey                    │
   │     Register in hub           │
   │                               │
   │  5. Authenticated{            │
   │     address: "pinch:hash@     │
   │       relay"}                 │
   │  ◄─────────────────────────   │
   │                               │
   │  6. Flush any queued msgs     │
   │  ◄─────────────────────────   │
```

### Key Data Flows

1. **Connection establishment:** Client authenticates via Ed25519 challenge-response, gets registered in the Hub, receives any queued messages.
2. **Connection request:** One agent requests a connection to another; relay routes the request; recipient approves/rejects; both sides exchange public keys on approval.
3. **Direct message:** Client-side encryption with NaCl box, relay routes opaque blob, recipient decrypts. Online = immediate, offline = queued.
4. **Group message:** Client-side encryption with shared symmetric key + NaCl secretbox, relay fans out to channel members. Key distribution happens via 1:1 encrypted channels.
5. **Heartbeat cycle:** Skill maintains WebSocket via OpenClaw heartbeat, processing inbound messages between heartbeats. No messages lost because store-and-forward covers gaps.

## Scaling Considerations

| Scale | Architecture Adjustments |
|-------|--------------------------|
| 0-100 agents | Single Go binary with BoltDB. Hub pattern handles all connections in one process. This is v1 target. |
| 100-10K agents | Same architecture. Go handles 10K concurrent WebSocket connections trivially (goroutines are cheap). May need to move from BoltDB to SQLite or Redis for faster queue operations under load. |
| 10K-100K agents | Shard the Hub by address prefix. Add Redis for cross-instance connection registry and pub/sub (same pattern Signal uses). Multiple relay instances behind a load balancer with sticky sessions. |
| 100K+ agents | Federation between relay instances (out of scope for v1). Each relay handles a subset of the address space. |

### Scaling Priorities

1. **First bottleneck:** Message queue I/O. BoltDB is single-writer. Under heavy offline-message load, move to Redis or a concurrent-write-friendly store.
2. **Second bottleneck:** Hub goroutine throughput. At extreme connection counts, shard the Hub. This is unlikely to matter before 50K+ connections.

## Anti-Patterns

### Anti-Pattern 1: Relay Decrypts or Inspects Content

**What people do:** Add server-side content filtering, search, or spam detection that requires decrypting messages.
**Why it is wrong:** Breaks the cryptographic blindness guarantee. If the relay can decrypt, a compromised relay exposes all messages.
**Do this instead:** All content-based operations (filtering, search, moderation) happen client-side. Rate limiting at the relay is per-sender/per-connection (metadata only).

### Anti-Pattern 2: Storing Private Keys on the Relay

**What people do:** Have the server generate keypairs and store private keys for "convenience" or "account recovery."
**Why it is wrong:** Turns the relay into a high-value target. Compromised relay = all identities compromised.
**Do this instead:** Keypair generation and storage is exclusively client-side. The relay only ever sees public keys.

### Anti-Pattern 3: Unbounded Message Queues

**What people do:** Store messages for offline agents indefinitely with no TTL or size limits.
**Why it is wrong:** Relay storage grows without bound. A malicious sender can DoS the relay by sending millions of messages to an offline address.
**Do this instead:** TTL-based expiration (7 days, matching Signal). Per-address queue size limits. Reject messages to full queues with backpressure.

### Anti-Pattern 4: Trusting the Relay for Connection Approval

**What people do:** Let the relay decide whether a connection request is valid or approved.
**Why it is wrong:** The relay should be a dumb pipe. Trust decisions belong to the agents (and their humans).
**Do this instead:** The relay routes connection requests like any other message. Approval/rejection logic lives entirely in the skill's Connection Manager.

### Anti-Pattern 5: Rolling Custom Crypto

**What people do:** Implement custom encryption schemes instead of using NaCl/libsodium primitives.
**Why it is wrong:** Custom crypto is almost always broken. NaCl is audited, battle-tested, and misuse-resistant by design.
**Do this instead:** Use NaCl box for 1:1, NaCl secretbox for group symmetric encryption. No custom primitives.

## Integration Points

### External Services

| Service | Integration Pattern | Notes |
|---------|---------------------|-------|
| OpenClaw runtime | Skill registers tools + heartbeat listener per SKILL.md spec | Skill is the only client type for v1. Wire format must be stable. |
| libsodium (Go) | `golang.org/x/crypto/nacl/box` and `nacl/secretbox` | Standard Go crypto library, wraps NaCl. No CGo needed. |
| libsodium (TS) | `tweetnacl` or `libsodium-wrappers` npm packages | tweetnacl is smaller; libsodium-wrappers is more complete. Use tweetnacl for v1. |

### Internal Boundaries

| Boundary | Communication | Notes |
|----------|---------------|-------|
| Relay <-> Skill | WebSocket with binary/JSON envelope | Must agree on envelope format. Versioned protocol. |
| Crypto module <-> Connection module (skill) | Function calls; crypto provides encrypt/decrypt, connection decides *when* | Crypto never makes trust decisions. Connection module owns the trust model. |
| Hub <-> Queue (relay) | Hub calls queue.Store() for offline, queue.Flush() on reconnect | Queue interface must be swappable (BoltDB -> Redis later). |
| Listener <-> Tools (skill) | Shared state (connection list, channel state, message history) | Listener writes inbound state; tools read/write outbound state. Need a shared store. |

## Build Order (Dependency Chain)

Build order is driven by what each component depends on. You cannot test message routing without authentication; you cannot test encryption without keypair management.

```
Phase 1: Foundation (no dependencies)
├── Relay: WebSocket server + Hub (accept connections, echo)
├── Skill: Keypair generation + address derivation
└── Protocol: Envelope format definition

Phase 2: Authentication (depends on Phase 1)
├── Relay: Challenge-response auth
├── Skill: Auth handshake client
└── Wire: Both sides speak the same auth protocol

Phase 3: Direct Messaging (depends on Phase 2)
├── Relay: Message routing (hub lookup + deliver)
├── Skill: NaCl box encrypt/decrypt for 1:1
└── Integration: Agent A sends encrypted message to Agent B (proof of life)

Phase 4: Store-and-Forward (depends on Phase 3)
├── Relay: Persistent message queue + TTL
├── Relay: Flush on reconnect
└── Skill: Handle message burst on reconnect

Phase 5: Connection Requests (depends on Phase 3)
├── Skill: Connection request/accept/reject/block flow
├── Skill: Public key exchange on approval
├── Relay: Routes connection requests like messages (no special logic)
└── Skill: Autonomy levels + permissions manifest

Phase 6: Group Channels (depends on Phase 3 + 5)
├── Relay: Channel membership + fan-out
├── Skill: Group key generation + distribution via 1:1 channels
├── Skill: NaCl secretbox encrypt/decrypt for groups
└── Skill: Member management (add/remove + key rotation)

Phase 7: Human Oversight (depends on Phase 5)
├── Skill: Activity feed (log all agent communication)
├── Skill: Audit log (connection events + messages)
├── Skill: Human intervention (override agent actions)
└── Skill: Rate limiting + circuit breakers per connection
```

**Build order rationale:**
- Phases 1-3 are the critical path to "proof of life" (two agents exchange an encrypted message). This should be the first milestone.
- Phase 4 (store-and-forward) is needed before any real usage because agents are intermittently online.
- Phase 5 (connection requests) is the trust layer. It can be built in parallel with Phase 4 since both depend on Phase 3.
- Phase 6 (groups) depends on both 1:1 encryption and the connection model being solid.
- Phase 7 (oversight) is the human safety layer. It can be built incrementally on top of anything after Phase 5.

## Sources

- [Signal Server source code analysis (SoftwareMill)](https://softwaremill.com/what-ive-learned-from-signal-server-source-code/) -- Signal server internals: Redis pub/sub, DynamoDB, message flow, WebSocket handling [MEDIUM confidence]
- [Signal Server GitHub](https://github.com/signalapp/Signal-Server) -- Official Signal server repository [HIGH confidence]
- [Signal Protocol Documentation](https://signal.org/docs/) -- Official Signal protocol specifications [HIGH confidence]
- [Matrix Specification](https://spec.matrix.org/latest/) -- Official Matrix protocol spec [HIGH confidence]
- [Synapse Architecture Overview (DeepWiki)](https://deepwiki.com/matrix-org/synapse/1.2-architecture-overview) -- Matrix Synapse homeserver architecture [MEDIUM confidence]
- [Dendrite Architecture (DeepWiki)](https://deepwiki.com/matrix-org/dendrite) -- Matrix Dendrite modular architecture with NATS JetStream [MEDIUM confidence]
- [E2E encryption with server-side fan-out (James Fisher)](https://jameshfisher.com/2017/10/25/end-to-end-encryption-with-server-side-fanout/) -- Group encryption architecture with sender keys [MEDIUM confidence]
- [NaCl crypto_box documentation](https://nacl.cr.yp.to/box.html) -- Official NaCl public-key authenticated encryption [HIGH confidence]
- [Libsodium sealed boxes](https://libsodium.gitbook.io/doc/public-key_cryptography/sealed_boxes) -- Official libsodium documentation [HIGH confidence]
- [Go NaCl box package](https://pkg.go.dev/golang.org/x/crypto/nacl/box) -- Official Go crypto package [HIGH confidence]
- [WebSocket architecture best practices (Ably)](https://ably.com/topic/websocket-architecture-best-practices) -- WebSocket patterns including per-client queues [MEDIUM confidence]
- [Scaling WebSockets in Go (Ably)](https://ably.com/topic/websockets-golang) -- Go WebSocket architecture patterns [MEDIUM confidence]
- [Gorilla WebSocket package](https://pkg.go.dev/github.com/gorilla/websocket) -- Standard Go WebSocket library [HIGH confidence]

---
*Architecture research for: Encrypted agent-to-agent relay messaging (Pinch)*
*Researched: 2026-02-26*
