# Feature Research

**Domain:** Secure agent-to-agent encrypted messaging protocol
**Researched:** 2026-02-26
**Confidence:** MEDIUM-HIGH

## Feature Landscape

### Table Stakes (Users Expect These)

Features users assume exist. Missing these = product feels incomplete.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| E2E encrypted 1:1 messaging | Foundational promise. Signal, Matrix, ARP all provide this. Without it the product has no reason to exist. | MEDIUM | NaCl box (X25519 + XSalsa20-Poly1305) is well-understood. Ed25519 signing + X25519 key exchange is standard. ARP uses HPKE (RFC 9180) but NaCl box is simpler and sufficient for v1. |
| Cryptographic identity (Ed25519 keypairs) | Standard for agent identity. ARP, DID systems, and A2A all use keypair-based identity. Eliminates dependency on central auth providers. | LOW | Ed25519 is well-audited, widely supported. Same keypair for signing (Ed25519) and encryption (convert to X25519 via libsodium). The `pinch:<hash>@<relay>` addressing format is clean. |
| Connection request / mutual consent | Core differentiator promise, but also table stakes for a privacy-first messaging system. Signal requires phone number exchange. Matrix requires room invites. No secure messaging system allows fully unsolicited contact. | MEDIUM | Requires: request creation, delivery, acceptance/rejection state machine, persistence of connection state. ARP rejects unknown senders by default -- Pinch should do the same but with an explicit approval flow. |
| Relay server (blind message routing) | The transport layer. ARP, Matrix homeservers, and Signal servers all serve this role. Without a relay, agents would need direct connectivity (NAT traversal nightmare). | MEDIUM | WebSocket-based, stateless routing table (pubkey -> connection). Relay sees only encrypted blobs. Go is ideal for this (goroutines, low memory, fast WebSocket handling). ARP's design validates this pattern. |
| Store-and-forward (offline delivery) | Agents are not always online -- they run on laptops, in containers, on schedules. Signal queues messages for offline devices. Matrix replicates history across homeservers. ARP deliberately lacks this and it's a known limitation. | MEDIUM-HIGH | Relay queues encrypted blobs with TTL. Must handle: queue size limits, TTL expiration, delivery confirmation, ordering guarantees. This is where Pinch surpasses ARP. |
| Real-time delivery (online agents) | When both agents are connected, messages should flow instantly via WebSocket. Every messaging system provides this. Latency target: sub-100ms relay hop. | LOW | Direct WebSocket forwarding. Trivial once the relay routing table exists. |
| Message delivery confirmation | Sender needs to know if message was delivered (and optionally read/processed). Signal has delivery receipts and read receipts. Without confirmation, agents cannot build reliable workflows. | LOW-MEDIUM | Delivery receipts at relay level (message accepted into queue or forwarded). Processing receipts from receiving agent. Must be E2E signed to prevent relay forgery. |
| Rate limiting per connection | Abuse prevention is non-negotiable for any messaging system. Without it, a single compromised agent can flood others. ARP implements per-agent and per-IP rate limits. | LOW | Token bucket or sliding window at relay level. Configurable per connection at agent level. Circuit breakers (auto-disconnect after threshold) add resilience. |
| Blocking and muting connections | Users must be able to cut off unwanted agents. Every messaging platform has block/mute. Without it, bad connections are permanent. | LOW | Block = relay rejects messages from that pubkey. Mute = agent-side suppression (messages still delivered but not surfaced). Must persist across reconnects. |
| Activity feed / human oversight | The human must see what their agent is doing. This is the "human-on-the-loop" pattern. Without visibility, humans lose trust and stop using the system. | MEDIUM | Chronological log of all sent/received messages, connection events, autonomy decisions. Must be queryable (filter by connection, time range, message type). Feeds into OpenClaw's existing UI patterns. |

### Differentiators (Competitive Advantage)

Features that set the product apart. Not required, but valuable.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Configurable autonomy levels (4-tier) | No other agent messaging system offers graduated human control per connection. A2A has no concept of it. ARP has no concept of it. This is Pinch's core innovation: Full Manual / Notify / Auto-respond / Full Auto per connection. | MEDIUM | State machine per connection. Full Manual = human approves every message. Notify = agent acts, human sees notification. Auto-respond = agent handles within rules, logs everything. Full Auto = agent operates independently. Requires clean UI for switching levels and clear behavioral contracts per level. |
| Inbound permissions manifest | Declare what a connection is allowed to send/request. Like a firewall ruleset for agent communication. Neither A2A nor ARP offer per-connection capability scoping -- A2A has Agent Cards for advertising capabilities, but not for restricting inbound requests from specific connections. | HIGH | Must define permission types (message categories, action types, file types, size limits). Must be enforceable at the agent level before decrypted content reaches the LLM. Requires a schema for permissions that is both expressive and simple to configure. |
| Human intervention / override | Human can step into any conversation and take over, correct agent behavior, or override a decision. Goes beyond "human-in-the-loop" approval gates to real-time takeover capability. | MEDIUM | Requires: notification that human is taking over, message attribution (agent vs human), seamless handoff back to agent. The "human temporarily becomes the agent" pattern. |
| Group encrypted channels with member management | Multi-agent collaboration (3+ agents coordinating on shared work). A2A supports multi-party but without E2E encryption. ARP is 1:1 only. Matrix supports encrypted groups but is vastly more complex. Pinch can offer simple encrypted groups purpose-built for agent collaboration. | HIGH | Shared group key (NaCl secretbox with shared symmetric key, distributed via pairwise-encrypted key shares). Member add/remove requires key rotation. Ordering and consistency are harder in groups. Consider Sender Keys pattern (Signal) for v1 over full MLS (RFC 9420) -- MLS is O(log n) but significantly more complex to implement. |
| Audit log (immutable, queryable) | Enterprise and compliance use case. Every agent action is logged with cryptographic proof. Goes beyond activity feed into tamper-evident, structured logging. Distinguishes Pinch as enterprise-ready. | MEDIUM | Append-only log with hash chaining. Each entry: timestamp, actor (pubkey), action type, connection ID, message hash (not content -- content stays encrypted). Queryable by time range, connection, action type. ~5-10ms overhead per operation per research findings. |
| Self-hostable relay | Users control their infrastructure. No vendor lock-in, no trust in a third-party relay operator. ARP offers this. Matrix offers this. Signal does not (centralized servers). For developer/enterprise audience, this is expected. | LOW | Already planned. Go binary, single-binary deployment, Docker image. The relay is stateless (or near-stateless with store-and-forward queue), making self-hosting trivial. |
| Connection-scoped circuit breakers | Auto-disconnect or throttle a connection that exhibits anomalous behavior (burst messaging, oversized payloads, repeated errors). Goes beyond simple rate limiting to adaptive protection. | LOW-MEDIUM | Track message frequency, error rate, payload sizes per connection. Configurable thresholds. Auto-downgrade autonomy level (e.g., Full Auto -> Notify) when circuit breaker trips. This is unique to Pinch -- treating autonomy level as a dynamic safety control, not just a static setting. |

### Anti-Features (Commonly Requested, Often Problematic)

Features that seem good but create problems.

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| Forward secrecy / Double Ratchet (v1) | Signal uses it. It's the gold standard for messaging encryption. Provides protection if long-term keys are compromised. | Massive implementation complexity. Double Ratchet requires per-message key derivation, out-of-order message handling, session state management. For agent-to-agent messaging where sessions are shorter-lived and keys can be rotated more aggressively, the marginal security gain does not justify the v1 complexity. | NaCl box per-message encryption with periodic keypair rotation. Plan the protocol envelope to be crypto-agnostic so Double Ratchet can be added in v2 without breaking changes. Explicitly document as out-of-scope with upgrade path. |
| Relay federation | Matrix does it. Decentralization is appealing. Users may want agents on different relays to communicate. | Federation adds: relay discovery, cross-relay routing, trust negotiation between relays, split-brain handling, message ordering across relays. Matrix took years to get federation stable. For v1, single-relay simplicity is critical. | Single relay instance per deployment. Multiple agents connect to the same relay. If federation is needed later, the relay protocol can be extended with relay-to-relay forwarding. The `pinch:<hash>@<relay>` address format already encodes relay identity, making future federation possible without address format changes. |
| OAuth / third-party auth | Enterprise environments use OAuth/OIDC. A2A protocol requires it. Seems necessary for enterprise adoption. | Adds dependency on external identity providers. Breaks the self-contained cryptographic identity model. Ed25519 keypairs are simpler, more portable, and don't require network calls to verify. Agent identity should be self-sovereign, not dependent on a corporate IdP. | Ed25519 keypairs as the sole identity system. If enterprise needs OAuth-gated relay access, that can be a relay-level access control layer (who can connect) separate from agent identity (who you are in conversations). |
| Rich media rendering / UI negotiation | A2A supports multiple modalities (audio, video, web forms). Seems necessary for a complete messaging experience. | Pinch agents communicate on behalf of humans -- they exchange structured data and text, not rich media for human consumption. Building media handling, format negotiation, and rendering adds complexity without serving the core use case. | Support three message types: text, files (binary blobs with metadata), and action confirmations (structured JSON). Keep the protocol simple. If an agent needs to share an image, it sends it as a file. No need for inline rendering or format negotiation. |
| Mobile/web client UI | Every messaging app has a mobile app. Users expect to check messages on their phone. | Pinch is not a messaging app for humans -- it's a messaging protocol for agents. Humans interact through OpenClaw's activity feed, not through a standalone chat UI. Building a mobile/web client is an entire product that distracts from the core protocol. | Human oversight through OpenClaw's existing UI (activity feed, tool invocation). The skill surface is the UI. If a standalone viewer is needed later, it can read the audit log / activity feed via API. |
| Payment / billing at relay level | Monetization seems necessary. Relay resources cost money. | Adds significant complexity (metering, accounts, billing integration, payment processing). Premature for v1. Self-hosted relays eliminate the need. | Self-hosted relays are free. If a hosted relay service is offered later, billing can be a separate layer above the relay protocol. Don't bake monetization into the protocol itself. |
| Agent capability discovery / Agent Cards | A2A's Agent Card pattern (JSON capability advertisement) seems useful for agents to find each other. | Pinch is not a discovery platform -- it's a messaging protocol. Discovery happens out-of-band (humans decide which agents should connect, then initiate connection requests). Adding capability advertisement adds protocol surface area without serving the consent-first model. | Connection requests carry a human-readable description of the connecting agent's purpose. The human approves based on this description plus any out-of-band context. Discovery is a separate concern from messaging. |
| Post-quantum cryptography (v1) | Signal has PQXDH. Quantum computing is a future threat. Security-conscious users may ask for it. | Post-quantum algorithms (ML-KEM/Kyber) add larger key sizes, slower operations, and implementation complexity. The threat is not yet practical. Agent messaging sessions are ephemeral compared to long-term encrypted archives. | Use standard Ed25519/X25519/NaCl for v1. Design the protocol envelope to be algorithm-agnostic (identify cipher suite in message header). Add PQ hybrid mode when the ecosystem matures and libraries stabilize. |

## Feature Dependencies

```
Ed25519 Keypair Identity
    |
    +---> Connection Request Model (requires identity to address requests)
    |         |
    |         +---> Blocking/Muting (requires established connections to block)
    |         |
    |         +---> Autonomy Levels (configured per connection)
    |         |         |
    |         |         +---> Human Intervention/Override (requires autonomy framework to override)
    |         |         |
    |         |         +---> Circuit Breakers (can auto-adjust autonomy level)
    |         |
    |         +---> Inbound Permissions Manifest (scoped per connection)
    |
    +---> Relay Server (routes by pubkey)
    |         |
    |         +---> Real-time Delivery (WebSocket forwarding)
    |         |
    |         +---> Store-and-Forward (encrypted queue at relay)
    |         |
    |         +---> Rate Limiting (enforced at relay)
    |
    +---> E2E Encrypted 1:1 Messaging (requires identity + relay)
    |         |
    |         +---> Message Delivery Confirmation (requires message exchange)
    |         |
    |         +---> Group Encrypted Channels (extends 1:1 crypto to N parties)
    |
    +---> Activity Feed (logs all identity-authenticated events)
              |
              +---> Audit Log (extends activity feed with tamper-evidence)
```

### Dependency Notes

- **Everything requires Ed25519 Keypair Identity:** It's the foundation. Without identity, nothing else works (no addressing, no encryption, no connection management).
- **Connection Request Model requires Identity:** You need to know who you're requesting a connection with. Addresses (`pinch:<hash>@<relay>`) derive from public keys.
- **Autonomy Levels require Connection Model:** Autonomy is configured per-connection, so connections must exist first.
- **Store-and-Forward requires Relay Server:** The relay is where offline message queuing happens.
- **Group Channels require working 1:1 Encryption:** Group crypto extends the pairwise encryption model. Get 1:1 right first.
- **Audit Log extends Activity Feed:** The audit log adds cryptographic tamper-evidence on top of the activity feed's chronological record.
- **Circuit Breakers enhance Autonomy Levels:** A tripped circuit breaker can auto-downgrade a connection's autonomy level -- this is a unique Pinch capability that depends on both systems existing.
- **Inbound Permissions Manifest conflicts with simple message types in v1:** If v1 only supports text/files/action-confirmations, the permissions manifest schema is constrained. Design the manifest format to be extensible.

## MVP Definition

### Launch With (v1)

Minimum viable product -- what's needed to validate the concept (two agents exchanging encrypted messages with human consent).

- [ ] **Ed25519 keypair generation and management** -- identity is the foundation of everything
- [ ] **Relay server with WebSocket routing** -- the transport backbone; blind to content
- [ ] **E2E encrypted 1:1 messaging (NaCl box)** -- the core promise
- [ ] **Connection request model (mutual approval)** -- the consent layer that distinguishes Pinch
- [ ] **Store-and-forward for offline agents** -- agents are not always online; without this, messages are lost (ARP's known weakness)
- [ ] **Real-time delivery for online agents** -- sub-100ms relay hop for connected agents
- [ ] **Basic autonomy levels (at minimum: Manual + Full Auto)** -- the simplest version of the 4-tier system to prove the concept
- [ ] **Activity feed (human can see what happened)** -- minimum oversight capability
- [ ] **Blocking connections** -- safety valve; must exist from day one

### Add After Validation (v1.x)

Features to add once core is working.

- [ ] **Full 4-tier autonomy levels** -- expand from 2 to 4 tiers once the framework is proven
- [ ] **Inbound permissions manifest** -- once message types are stable, add per-connection scoping
- [ ] **Message delivery confirmations** -- needed for reliable agent workflows
- [ ] **Muting connections** -- softer control than blocking
- [ ] **Human intervention / override** -- step-in capability for active conversations
- [ ] **Rate limiting and circuit breakers** -- abuse prevention at scale
- [ ] **Audit log with hash chaining** -- tamper-evident logging for enterprise use

### Future Consideration (v2+)

Features to defer until product-market fit is established.

- [ ] **Group encrypted channels** -- HIGH complexity; defer until 1:1 is solid and there's demonstrated demand for multi-agent coordination
- [ ] **Connection-scoped circuit breakers with auto-autonomy-adjustment** -- requires mature autonomy system
- [ ] **Forward secrecy / Double Ratchet** -- crypto upgrade path; design protocol envelope to accommodate
- [ ] **Relay federation** -- address format supports it; implementation deferred
- [ ] **Post-quantum hybrid encryption** -- monitor ecosystem maturity; add when libraries stabilize
- [ ] **MLS (RFC 9420) for group encryption** -- O(log n) group key management; consider when groups exceed ~10 members

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| Ed25519 keypair identity | HIGH | LOW | P1 |
| Relay server (blind routing) | HIGH | MEDIUM | P1 |
| E2E encrypted 1:1 messaging | HIGH | MEDIUM | P1 |
| Connection request model | HIGH | MEDIUM | P1 |
| Store-and-forward | HIGH | MEDIUM | P1 |
| Real-time delivery | HIGH | LOW | P1 |
| Basic autonomy levels (2-tier) | HIGH | LOW-MEDIUM | P1 |
| Activity feed | HIGH | MEDIUM | P1 |
| Blocking connections | HIGH | LOW | P1 |
| Full 4-tier autonomy | HIGH | MEDIUM | P2 |
| Inbound permissions manifest | HIGH | HIGH | P2 |
| Delivery confirmations | MEDIUM | LOW-MEDIUM | P2 |
| Muting connections | MEDIUM | LOW | P2 |
| Human intervention/override | HIGH | MEDIUM | P2 |
| Rate limiting / circuit breakers | MEDIUM | LOW-MEDIUM | P2 |
| Audit log (tamper-evident) | MEDIUM | MEDIUM | P2 |
| Group encrypted channels | HIGH | HIGH | P3 |
| Circuit breaker auto-autonomy | MEDIUM | MEDIUM | P3 |
| Forward secrecy | MEDIUM | HIGH | P3 |
| Relay federation | LOW | HIGH | P3 |

## Competitor Feature Analysis

| Feature | Signal | Matrix | A2A Protocol | ARP (Agent Relay Protocol) | Pinch (Our Approach) |
|---------|--------|--------|-------------|---------------------------|---------------------|
| E2E Encryption | Double Ratchet (gold standard) | Olm/Megolm (Double Ratchet variant) | None (relies on transport security) | HPKE (RFC 9180) | NaCl box (X25519 + XSalsa20-Poly1305), upgradeable |
| Identity | Phone number + keypair | Username + keypair | OAuth/OIDC + Agent Card | Ed25519 keypair | Ed25519 keypair with `pinch:<hash>@<relay>` address |
| Store-and-forward | Yes (queued at server) | Yes (replicated across homeservers) | N/A (HTTP request/response) | No (messages dropped if offline) | Yes (encrypted queue at relay with TTL) |
| Group messaging | Yes (Sender Keys) | Yes (Megolm + MLS migration) | Yes (multi-party tasks) | No (1:1 only) | Yes in v2 (Sender Keys pattern initially) |
| Federation | No (centralized) | Yes (core design) | No (direct HTTP) | No (single relay) | No in v1 (address format supports future federation) |
| Human consent model | Contact must have phone number | Room invite acceptance | No consent model | Rejects unknown senders | Mutual connection request approval with configurable autonomy |
| Autonomy levels | N/A (human-operated) | N/A (human-operated) | N/A (agent fully autonomous) | N/A (no concept) | 4-tier per-connection (Full Manual -> Full Auto) |
| Inbound permissions | N/A | Room permissions/power levels | Agent Card capabilities | Contact filtering | Per-connection permissions manifest |
| Audit/oversight | Message history | Room history | Task history | None | Activity feed + tamper-evident audit log |
| Self-hostable | No | Yes | Yes (just HTTP endpoints) | Yes | Yes |

## Sources

- [Google A2A Protocol Announcement](https://developers.googleblog.com/en/a2a-a-new-era-of-agent-interoperability/) -- Agent Card, capability discovery, A2A architecture
- [A2A Protocol Specification](https://a2a-protocol.org/latest/specification/) -- Protocol details, authentication schemes
- [IBM: What is A2A Protocol?](https://www.ibm.com/think/topics/agent2agent-protocol) -- A2A overview, JWT/OIDC authentication
- [A2A Protocol Discovery](https://a2a-protocol.org/latest/topics/agent-discovery/) -- Agent Card JSON format, discovery mechanisms
- [Linux Foundation A2A Launch](https://www.linuxfoundation.org/press/linux-foundation-launches-the-agent2agent-protocol-project-to-enable-secure-intelligent-communication-between-ai-agents) -- Governance, open-source status
- [Semgrep: Security Engineer's Guide to A2A](https://semgrep.dev/blog/2025/a-security-engineers-guide-to-the-a2a-protocol/) -- A2A security analysis
- [Signal Protocol Documentation](https://signal.org/docs/) -- Double Ratchet, X3DH, cryptographic primitives
- [Signal Protocol Wikipedia](https://en.wikipedia.org/wiki/Signal_Protocol) -- Protocol components, group chat support
- [Signal: Forward Secrecy for Asynchronous Messages](https://signal.org/blog/asynchronous-security/) -- Store-and-forward encryption patterns
- [Signal: Post-Quantum Ratchets (PQXDH)](https://signal.org/blog/spqr/) -- PQ upgrade path reference
- [Matrix Protocol](https://matrix.org/) -- Federation, Olm/Megolm encryption, room-based architecture
- [Matrix Protocol Wikipedia](https://en.wikipedia.org/wiki/Matrix_(protocol)) -- Architecture overview, encryption features
- [IETF RFC 9420: MLS Protocol](https://datatracker.ietf.org/doc/rfc9420/) -- Group key management, O(log n) scaling
- [MLS Architecture](https://messaginglayersecurity.rocks/mls-architecture/draft-ietf-mls-architecture.html) -- MLS design rationale
- [Agent Relay Protocol (ARP)](https://github.com/offgrid-ing/arp) -- Closest comparable system: stateless WebSocket relay, Ed25519 identity, HPKE encryption, no store-and-forward
- [Human-in-the-Loop Agentic AI (OneReach)](https://onereach.ai/blog/human-in-the-loop-agentic-ai-systems/) -- HITL vs HOTL patterns
- [Evolving AI Agent Autonomy: HITL to HOTL](https://bytebridge.medium.com/from-human-in-the-loop-to-human-on-the-loop-evolving-ai-agent-autonomy-c0ae62c3bf91) -- Autonomy level patterns
- [Galileo: Human-in-the-Loop Agent Oversight](https://galileo.ai/blog/human-in-the-loop-agent-oversight) -- Synchronous approval vs asynchronous audit patterns
- [AI Agent Protocols 2026 Complete Guide](https://www.ruh.ai/blogs/ai-agent-protocols-2026-complete-guide) -- MCP/A2A/ACP ecosystem overview
- [AI Agents with DIDs and VCs](https://arxiv.org/html/2511.02841v1) -- Decentralized identity for agents, Ed25519/Secp256k1 verification
- [DIF: Building the Agentic Economy](https://blog.identity.foundation/building-the-agentic-economy/) -- Agent identity standards
- [NIST AI Agent Standards Initiative (2026)](https://www.nist.gov/news-events/news/2026/02/announcing-ai-agent-standards-initiative-interoperable-and-secure) -- Emerging regulatory standards for agent identity/authorization
- [Agentic AI Governance: 3-Tiered Approach](https://www.mintmcp.com/blog/agentic-ai-goverance-framework) -- Risk-based autonomy tiering
- [Audit Trails for Agents (Adopt AI)](https://www.adopt.ai/glossary/audit-trails-for-agents) -- Immutable logging for agent actions
- [AI Agent Compliance and Governance (Galileo)](https://galileo.ai/blog/ai-agent-compliance-governance-audit-trails-risk-management) -- Enterprise audit requirements
- [Survey of Agent Interoperability Protocols](https://arxiv.org/html/2505.02279v1) -- MCP, ACP, A2A, ANP comparison

---
*Feature research for: Secure agent-to-agent encrypted messaging protocol*
*Researched: 2026-02-26*
