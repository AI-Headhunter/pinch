# Project Research Summary

**Project:** Pinch
**Domain:** Secure agent-to-agent encrypted messaging (Go relay + TypeScript OpenClaw skill)
**Researched:** 2026-02-26
**Confidence:** HIGH

## Executive Summary

Pinch is a purpose-built encrypted messaging protocol for AI agents — closer to Signal's server design than to consumer messaging apps. The architecture is a two-component system: a cryptographically blind Go relay that routes opaque encrypted blobs without ever decrypting them, and a TypeScript OpenClaw skill that owns all crypto, trust, and autonomy logic client-side. Research confirms this pattern is well-understood (Signal, ARP, Matrix all validate it), the full technology stack has high-confidence library choices with verified versions, and the core build order is unambiguous. The critical path to "proof of life" is 3 phases: relay WebSocket foundation, Ed25519 challenge-response auth, and NaCl box 1:1 encryption. Everything else builds on that backbone.

Pinch's genuine differentiator is not the encryption (NaCl box is standard) or the relay (WebSocket hub is standard) — it is the configurable per-connection autonomy model (Full Manual through Full Auto) combined with an inbound permissions manifest and a consent-first connection model. No comparable system (A2A, ARP, Matrix) offers graduated human control per agent connection. This is the feature that requires the most careful design and the most security attention: autonomy escalation without audit trails is the most likely way Pinch causes unintended harm. The default for every new connection must be Full Manual, and every autonomy change must be logged.

The most underestimated risk is cross-language cryptographic interoperability between Go's standard library NaCl primitives and TypeScript's `libsodium-wrappers-sumo`. Research explicitly calls for cross-language roundtrip tests (Go encrypts / TypeScript decrypts and vice versa) to be written before any protocol logic, not after. The second-most underestimated risk is nonce reuse in NaCl secretbox/box — a trivially exploitable flaw if nonce generation is careless. Both risks must be addressed in Phase 1, not retrofitted. Store-and-forward, group channels, and the full autonomy system are all achievable but require these foundations to be correct first.

## Key Findings

### Recommended Stack

The stack is split across a Go relay server and a TypeScript SDK, tied together by a shared Protocol Buffers wire format. On the Go side, `github.com/coder/websocket` v1.8.14 is the right WebSocket library (context-aware, maintained by Coder as the nhooyr.io/websocket successor). Cryptography uses Go stdlib (`crypto/ed25519`, `crypto/ecdh`) plus `golang.org/x/crypto/nacl/box` and `nacl/secretbox` for NaCl operations, with `filippo.io/edwards25519` v1.2.0 for Ed25519-to-X25519 key conversion. Message queuing uses `go.etcd.io/bbolt` v1.4.3 — embedded, no CGO, single file, ACID, ideal for the store-and-forward use case at v1 scale. On the TypeScript side, `libsodium-wrappers-sumo` (not the standard `libsodium-wrappers`) is mandatory — the sumo variant is the only one that includes `crypto_sign_ed25519_pk_to_curve25519` and `crypto_sign_ed25519_sk_to_curve25519`, which Pinch requires. `@bufbuild/protobuf` v2.11.0 with `buf` CLI replaces raw protoc and is the only JavaScript Protobuf library that passes full conformance tests.

**Core technologies:**
- `github.com/coder/websocket` v1.8.14: WebSocket server — context-native, maintained successor to nhooyr.io/websocket
- `crypto/ed25519` + `crypto/ecdh` (Go stdlib): signing and X25519 key exchange — no external dependency
- `golang.org/x/crypto/nacl/box` + `nacl/secretbox` v0.48.0+: NaCl 1:1 and group encryption — interoperable with libsodium
- `filippo.io/edwards25519` v1.2.0: Ed25519-to-X25519 conversion — maintained by Go crypto team member
- `go.etcd.io/bbolt` v1.4.3: store-and-forward queue — embedded, ACID, no CGO
- `google.golang.org/protobuf` v1.36.11 + `buf` CLI: wire format with type-safe cross-language code generation
- `libsodium-wrappers-sumo` 0.8.0: TypeScript NaCl — WASM, cross-platform, full API including key conversion
- `@bufbuild/protobuf` v2.11.0: TypeScript Protobuf runtime — conformance-tested, idiomatic TS output

**Critical version note:** Go 1.22+ required for bbolt v1.4.x. Use `libsodium-wrappers-sumo`, NOT `libsodium-wrappers` — the non-sumo variant is missing required key conversion functions.

### Expected Features

Research against Signal, Matrix, A2A, and ARP confirms the feature landscape. The consent-first connection model and configurable autonomy levels are Pinch's unique contributions — no comparable system has both.

**Must have (table stakes):**
- Ed25519 keypair identity with `pinch:<hash>@<relay>` addressing — everything depends on this
- Relay with WebSocket routing (cryptographically blind) — the transport backbone
- E2E encrypted 1:1 messaging (NaCl box) — the core promise
- Connection request and mutual consent model — the consent layer that distinguishes Pinch from A2A
- Store-and-forward for offline agents — ARP's known weakness; Pinch must solve this
- Real-time delivery for online agents — sub-100ms relay hop
- Basic autonomy levels (at minimum Full Manual + Full Auto) — proves the concept
- Activity feed / human oversight — minimum human-on-the-loop capability
- Connection blocking — safety valve, must exist from day one

**Should have (competitive):**
- Full 4-tier autonomy (Full Manual / Notify / Auto-respond / Full Auto) — the core innovation
- Inbound permissions manifest per connection — per-connection firewall ruleset
- Message delivery confirmations — required for reliable agent workflows
- Human intervention / override — real-time step-in capability
- Rate limiting and circuit breakers — abuse prevention; circuit breakers can auto-downgrade autonomy level
- Audit log with hash chaining — tamper-evident for enterprise use
- Muting connections (softer than blocking)

**Defer (v2+):**
- Group encrypted channels — HIGH complexity; get 1:1 solid first, then expand
- Forward secrecy / Double Ratchet — design protocol envelope to accommodate; defer implementation
- Relay federation — address format already supports it; implementation is post-v1
- Post-quantum hybrid encryption — monitor ecosystem; add when libraries stabilize
- MLS (RFC 9420) for group encryption — defer until groups exceed ~10 members and demand is proven

### Architecture Approach

Pinch uses a cryptographically blind relay (Hub-and-spoke WebSocket pattern in Go) paired with a client-side skill that owns all crypto and trust logic. The relay sees sender address, recipient address, encrypted blob, and timestamp — nothing more. All decryption, connection approval, and autonomy decisions happen in the TypeScript skill. The Go relay implements a standard hub goroutine managing a `map[string]*Client` (address to socket), with each connection running independent readPump and writePump goroutines. Authentication is Ed25519 challenge-response: relay sends a nonce, client signs it, relay derives the `pinch:` address from the public key. Store-and-forward uses bbolt with per-recipient buckets and TTL expiration. The project is a monorepo with `relay/`, `skill/`, and `proto/` as top-level directories, with a `buf.gen.yaml` generating type-safe code for both Go and TypeScript from the same `.proto` files.

**Major components:**
1. **Go Relay / WS Hub** — Connection registry, readPump/writePump goroutines per connection, routes opaque encrypted blobs, never inspects content
2. **Go Relay / Message Queue** — bbolt-backed store-and-forward, per-recipient buckets, TTL expiration, flush on reconnect
3. **Go Relay / Auth** — Ed25519 challenge-response, nonce signing, address derivation from public key
4. **TypeScript Skill / Crypto Module** — Keypair management, NaCl box/secretbox encrypt-decrypt, Ed25519-to-X25519 conversion via libsodium-sumo
5. **TypeScript Skill / Connection Manager** — Connection request lifecycle, autonomy levels, inbound permissions manifest, blocking/muting
6. **TypeScript Skill / Transport** — WebSocket client, heartbeat integration with OpenClaw, reconnection handling
7. **Shared Proto Layer** — `.proto` files in `/proto`, `buf` generates Go and TypeScript code, single source of truth for wire format

### Critical Pitfalls

1. **Nonce reuse in NaCl secretbox/box** — Use 24-byte random nonces (CSPRNG) for every message, never counter-based nonces without atomic persistence. Prepend nonce to ciphertext. Write tests with random nonces. Address in Phase 1 before any message handling.

2. **Cross-language crypto mismatch (Go stdlib NaCl vs. libsodium-sumo)** — Write cross-language roundtrip tests FIRST: Go encrypts / TypeScript decrypts and vice versa. Run in CI. Test all 4 operations: signing, key exchange, secretbox, box. This is a prerequisite for all protocol work.

3. **WebSocket goroutine leak (Go relay)** — Implement ping/pong heartbeats at 20-30s intervals with 5-10s pong timeout. Set `SetReadDeadline`/`SetWriteDeadline` on every operation. Monitor `runtime.NumGoroutine()`. Must be correct from Phase 1 or load tests will surface it expensively.

4. **Replay attacks on store-and-forward messages** — NaCl authenticated encryption guarantees authenticity, NOT uniqueness. Include monotonically increasing sequence numbers AND timestamps inside the encrypted payload. Recipients reject already-seen sequence numbers. Address in Phase 1 message format design.

5. **Autonomy level escalation without audit trail** — Default autonomy for ALL new connections must be Full Manual. Every autonomy level change must write an immutable audit log entry. 80% of orgs deploying autonomous agents cannot see what their agents are doing in real time — Pinch must not be in that category. Address in autonomy design phase before shipping any autonomy feature.

6. **Single-keypair risk (Ed25519 for signing + X25519 for encryption)** — Accepted tradeoff for v1, but design the protocol envelope NOW to have separate `signing_key` and `encryption_key` fields (even if they derive from the same keypair in v1). Add protocol version field from day one. Retrofitting this is a breaking protocol change.

## Implications for Roadmap

Based on research, the build order is architecturally driven by hard dependencies and security requirements. The suggested phase structure below follows the dependency chain from ARCHITECTURE.md and maps directly to pitfall prevention from PITFALLS.md.

### Phase 1: Foundation and Crypto Primitives

**Rationale:** Everything else depends on this. The protocol envelope format, nonce handling, and cross-language crypto interoperability must be correct before any higher-level logic is built on top. Retrofitting security properties into a protocol is extremely expensive. PITFALLS.md identifies 3 of the top 6 pitfalls as Phase 1 issues (nonce reuse, cross-language mismatch, replay attacks).

**Delivers:** Working Go relay accepting WebSocket connections; TypeScript skill connecting to relay; Ed25519 keypair generation and address derivation; Protocol Buffers envelope format (with replay protection fields, separate key fields, protocol version); Cross-language crypto roundtrip tests passing in CI.

**Addresses:** Ed25519 keypair identity, relay WebSocket server foundation, protocol envelope design.

**Avoids:** Nonce reuse (random nonce generation from day 1), cross-language mismatch (CI roundtrip tests before protocol logic), replay attacks (sequence numbers in envelope), key separation gap (envelope has signing_key + encryption_key fields), goroutine leaks (ping/pong + deadlines in relay from day 1).

**Research flag:** Standard patterns — Go WebSocket hub and NaCl crypto are well-documented. Cross-language interop test harness may need a focused implementation session but no additional research required.

### Phase 2: Authentication and Connection

**Rationale:** Cannot test any message routing without authentication. Cannot establish autonomy or permissions without established connections. The challenge-response auth flow is the gate through which all subsequent protocol activity passes.

**Delivers:** Ed25519 challenge-response authentication (relay sends nonce, skill signs, relay verifies and registers); `pinch:<hash>@<relay>` address derivation; Connection request / accept / reject / block flow; Public key exchange on approval; Baseline autonomy model (Full Manual and Full Auto, with audit log from day one); Blocking enforcement at relay level.

**Addresses:** Connection request model, mutual consent, cryptographic identity, blocking connections.

**Avoids:** Autonomy escalation without audit trail (autonomy level logged from the first implementation), connection consent bypass (connection acceptances are Ed25519-signed and verified at the skill, not trusted from relay).

**Research flag:** Standard patterns — challenge-response auth is well-documented. Autonomy state machine design may benefit from a brief research pass on agent governance patterns if the 4-tier model needs refinement beyond what FEATURES.md describes.

### Phase 3: Encrypted 1:1 Messaging

**Rationale:** This is the "proof of life" milestone. With auth established, this phase delivers the core product promise: two agents exchanging encrypted messages with the relay seeing only ciphertext. All subsequent features (store-and-forward, groups, autonomy actions) extend this baseline.

**Delivers:** NaCl box (X25519 + XSalsa20-Poly1305) client-side encryption/decryption; Real-time message delivery for online agents (relay routes opaque blobs via hub); Message processing per autonomy level (Full Manual = human approves, Full Auto = agent acts).

**Addresses:** E2E encrypted 1:1 messaging, real-time delivery.

**Avoids:** Relay decryption (relay never touches plaintext), private key relay storage (keys stay client-side exclusively).

**Research flag:** Standard patterns — NaCl box and WebSocket message routing are well-documented.

### Phase 4: Store-and-Forward

**Rationale:** Agents are intermittently online by nature (running on laptops, in containers, on schedules). Without store-and-forward, the product is unusable for real workflows. This is ARP's documented weakness; Pinch explicitly solves it. Must be built before any real usage.

**Delivers:** bbolt-backed persistent message queue at relay; Per-recipient message buckets; TTL-based expiration (7-day default); Queue depth limits with backpressure; Message flush on agent reconnect; Delivery confirmation receipts.

**Addresses:** Store-and-forward, message delivery confirmation, offline agent support.

**Avoids:** Unbounded queue growth (per-agent queue depth limits + TTL enforced), in-memory queue anti-pattern (bbolt from day one in this phase, not "add later"), replay attacks via duplicate delivery (relay deduplicates by message ID; skill checks sequence numbers inside encrypted payload).

**Research flag:** Standard patterns — bbolt queue design is well-documented. Queue depth limits and TTL logic are straightforward.

### Phase 5: Full Autonomy System and Activity Feed

**Rationale:** The 2-tier autonomy from Phase 2 proves the concept; this phase completes Pinch's core differentiator. The full 4-tier model (Full Manual / Notify / Auto-respond / Full Auto), inbound permissions manifest, and human oversight surfaces (activity feed, human intervention / override) are what make Pinch meaningfully different from ARP or A2A.

**Delivers:** Full 4-tier autonomy levels per connection with state machine; Inbound permissions manifest (per-connection firewall ruleset); Activity feed (chronological log of all agent events, queryable); Human intervention / override (human takes over conversation temporarily); Muting connections.

**Addresses:** Full 4-tier autonomy, inbound permissions manifest, activity feed, human intervention, muting.

**Avoids:** Autonomy escalation without audit trail (all changes logged with immutable entries), permissive defaults (Full Manual enforced for new connections with explicit confirmation to escalate).

**Research flag:** May benefit from a research pass on autonomy UX patterns and permissions manifest schema design. The 4-tier model is Pinch's unique contribution — no direct prior art to copy from. The permissions schema needs to be extensible enough for future message types without being so complex it's hard to configure.

### Phase 6: Safety and Abuse Prevention

**Rationale:** Rate limiting, circuit breakers, and the audit log are not glamorous but are non-negotiable before any production exposure. Circuit breakers that auto-downgrade autonomy level are one of Pinch's unique safety mechanisms — they must be built before the system is used at any scale.

**Delivers:** Per-connection rate limiting (token bucket or sliding window); Circuit breakers per connection with configurable thresholds; Auto-autonomy-downgrade when circuit breaker trips; Tamper-evident audit log with hash chaining; Relay certificate pinning in skill configuration.

**Addresses:** Rate limiting, circuit breakers, audit log, connection-scoped protection.

**Avoids:** Per-connection vs. global rate limit anti-pattern (limits are per source-destination pair), cleartext error messages revealing state (sanitized error codes throughout).

**Research flag:** Standard patterns for rate limiting and circuit breakers. Hash-chained audit log is straightforward append-only implementation.

### Phase 7: Group Encrypted Channels (v1.x / v2)

**Rationale:** Group channels have significantly higher complexity than 1:1 messaging (key distribution, member management, key rotation on removal). Deferring until 1:1 is solid and demand is demonstrated is the right call. The server-side fan-out pattern with NaCl secretbox is simpler than Signal's Sender Keys ratchet and sufficient for small groups (<20 members).

**Delivers:** Group channel creation with shared symmetric key; Server-side fan-out at relay; Group membership management; Key rotation on member removal (mandatory — the most common group encryption failure); NaCl secretbox group encrypt/decrypt.

**Addresses:** Group encrypted channels, multi-agent collaboration.

**Avoids:** Group key rotation missing (enforce rotation as part of member removal API — the API cannot succeed without it), per-message public-key encryption for groups (use shared symmetric key), O(n) re-encryption on every message (only re-encrypt group key on membership changes).

**Research flag:** Needs a research pass on group key distribution and Sender Keys pattern when this phase is planned. Group crypto has non-obvious failure modes documented in PITFALLS.md.

### Phase Ordering Rationale

- **Phases 1-3 are the critical path.** Nothing else can be tested or validated without working authenticated encrypted message exchange. This should be the first milestone and demo target.
- **Phase 4 (store-and-forward) unblocks real usage.** It can be developed in parallel with Phase 5 (autonomy) since both depend only on Phase 3. However, Phase 4 should be prioritized because agents will be intermittently offline during development testing.
- **Phase 5 (autonomy) is Pinch's differentiator.** It is complex enough to warrant its own phase and should not be compressed into Phase 2's 2-tier baseline.
- **Phase 6 (safety) must precede any external exposure.** Do not share Pinch with outside users before rate limiting and circuit breakers exist.
- **Phase 7 (groups) is explicitly deferred.** The complexity cost is high and the incremental value over 1:1 messaging depends on demonstrated demand.

### Research Flags

Phases likely needing a `/gsd:research-phase` during planning:
- **Phase 5 (Autonomy System):** Permissions manifest schema design is novel territory — no direct prior art. Need to define permission types, enforcement semantics, and configuration UX before implementation.
- **Phase 7 (Group Channels):** Group key distribution and member management have subtle security requirements. Sender Keys pattern needs deeper implementation research before this phase begins.

Phases with standard patterns (can proceed without research-phase):
- **Phase 1 (Foundation/Crypto):** NaCl primitives, Go WebSocket hub, and Protocol Buffers are all thoroughly documented. Cross-language interop tests are a known pattern.
- **Phase 2 (Auth/Connection):** Ed25519 challenge-response is standard. Connection state machines are well-understood.
- **Phase 3 (1:1 Messaging):** NaCl box and WebSocket message routing are thoroughly documented.
- **Phase 4 (Store-and-Forward):** bbolt queue design, TTL expiration, and reconnect flush are well-documented patterns.
- **Phase 6 (Safety/Abuse):** Rate limiting and circuit breaker patterns are thoroughly documented.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | All libraries verified at specific versions. Crypto interoperability matrix fully documented. Version compatibility confirmed. No significant unknowns. |
| Features | MEDIUM-HIGH | Table stakes features are well-defined against 4 comparable systems. Autonomy model is novel (Pinch's differentiator) — no prior art to validate against, which is expected. |
| Architecture | HIGH | Hub-and-spoke WebSocket pattern is well-documented in Go. Signal server design validates the blind relay approach. Build order dependency chain is clear and unambiguous. |
| Pitfalls | HIGH | Pitfalls are well-documented failure modes in cryptography, WebSocket systems, and agent communication. Sources include CVE references, library authors, and production post-mortems. |

**Overall confidence:** HIGH

### Gaps to Address

- **Permissions manifest schema:** The inbound permissions manifest is Pinch's most novel feature. The schema for permission types, enforcement semantics, and how they compose is not defined yet. Needs design work during Phase 5 planning. This is a feature design gap, not a research gap — the concept is sound, the implementation details need specification.

- **OpenClaw skill integration specifics:** ARCHITECTURE.md notes the skill must integrate with OpenClaw's heartbeat cycle and tool registration (per SKILL.md spec), but the exact OpenClaw API surface is not covered in the research files. This needs to be validated against actual OpenClaw documentation when the skill is being built.

- **Key backup/recovery story:** PITFALLS.md flags that losing a device means losing identity and all connections. The research notes encrypted key export/import as the solution but does not specify the implementation. This is a UX design decision needed before v1 ships to real users.

- **Relay TLS and deployment configuration:** The research covers the relay protocol but not the TLS termination, reverse proxy configuration, or Docker packaging for self-hosters. This is an operational concern for the relay deployment phase.

## Sources

### Primary (HIGH confidence)
- [coder/websocket v1.8.14](https://github.com/coder/websocket) — WebSocket library, verified latest release
- [golang.org/x/crypto/nacl](https://pkg.go.dev/golang.org/x/crypto/nacl/box) — NaCl box/secretbox, v0.48.0
- [filippo.io/edwards25519 v1.2.0](https://pkg.go.dev/filippo.io/edwards25519) — Ed25519-to-X25519 BytesMontgomery
- [go.etcd.io/bbolt v1.4.3](https://github.com/etcd-io/bbolt/releases) — embedded key-value store
- [google.golang.org/protobuf v1.36.11](https://github.com/protocolbuffers/protobuf-go/releases) — Go Protobuf v2 API
- [libsodium-wrappers-sumo 0.8.0](https://www.npmjs.com/package/libsodium-wrappers-sumo) — TypeScript NaCl, WASM
- [@bufbuild/protobuf v2.11.0](https://www.npmjs.com/package/@bufbuild/protobuf) — protobuf-es, conformance-tested
- [Signal Protocol Documentation](https://signal.org/docs/) — Double Ratchet, store-and-forward patterns
- [Signal Server GitHub](https://github.com/signalapp/Signal-Server) — blind relay architecture reference
- [NaCl crypto_box documentation](https://nacl.cr.yp.to/box.html) — canonical NaCl API reference
- [Agent Relay Protocol (ARP)](https://github.com/offgrid-ing/arp) — closest comparable system, validated Pinch's approach
- [Libsodium secretbox docs](https://libsodium.gitbook.io/doc/secret-key_cryptography/secretbox) — nonce handling
- [Libsodium Ed25519-to-Curve25519](https://libsodium.gitbook.io/doc/advanced/ed25519-curve25519) — key conversion

### Secondary (MEDIUM confidence)
- [Signal Server analysis (SoftwareMill)](https://softwaremill.com/what-ive-learned-from-signal-server-source-code/) — relay internals, Redis/DynamoDB patterns
- [A2A Protocol Specification](https://a2a-protocol.org/latest/specification/) — competitor feature comparison
- [Matrix Specification](https://spec.matrix.org/latest/) — federation and group encryption reference
- [E2E encryption with server-side fan-out (James Fisher)](https://jameshfisher.com/2017/10/25/end-to-end-encryption-with-server-side-fanout/) — group encryption architecture
- [Strata: AI Agent Identity Crisis (2026)](https://www.strata.io/blog/agentic-identity/the-ai-agent-identity-crisis-new-research-reveals-a-governance-gap/) — 80% orgs lack real-time agent oversight
- [gorilla/websocket goroutine leak issues](https://github.com/gorilla/websocket/issues/134) — WebSocket memory pitfalls

### Tertiary (LOW confidence)
- [Go WebSocket library comparison 2025](https://amf-co.com/which-golang-websocket-library-should-you-use-in-2025/) — ecosystem survey
- [WebSocket heartbeat best practices (2026)](https://oneuptime.com/blog/post/2026-01-27-websocket-heartbeat-ping-pong/view) — heartbeat configuration guidance

---
*Research completed: 2026-02-26*
*Ready for roadmap: yes*
