# Project State (agent-facing)

A compact snapshot of **where LightIPAM is right now**, for agents deciding what is
safe to build on. Keep this short. When shipped/planned status changes, **update this
file** — a stale snapshot here is worse than none.

> This file answers "where are we now?". It is not the source of truth for release
> history or roadmap — see [Source of truth](#source-of-truth) below.

## Current release

- **Latest release: v1.2.2** (see [`CHANGELOG.md`](../../CHANGELOG.md)). Tagged
  releases publish multi-arch app + scanner images to GHCR.
- Database schema is at **migration 23** (additive; migrations 1–23 applied in
  order). Migrations 22–23 (device hardware links, ADR 0029; SNMP hardware
  identity, ADR 0030) shipped in v1.2.0.
- Stable surfaces (SemVer-guarded): the JSON API `/api/v1` and the scanner protocol
  `v1`. A breaking change to either, or a destructive migration, is a major bump.

## Unreleased maintenance

- Subnet creates and imports now participate in the CIDR-edit advisory-lock
  protocol and recheck overlap before writing. PostgreSQL regressions cover stale
  imports, atomic rollback, exact-CIDR upserts, competing create/update/import
  writers and lock-wait cancellation. This is a scoped part of Roadmap R02;
  representative large-import qualification remains open. Schema stays at 23.

## Completed work

**Phases 1–6 are complete.** In brief (details in the roadmap and ADRs):

- **Phase 1 — Manual IPAM:** auth/bootstrap, subnets, sparse addresses, devices,
  MAC/vendor, audit log, dashboard, search, custom fields, bulk edit + CSV import/export.
- **Phases 2–4 — Scanner track:** mTLS agent, staged nmap discovery, and six
  unprivileged discovery sources (SNMP `arp_table` / `snmp_inventory` with 802.1Q VLAN
  / `lldp_cdp`, NetBIOS/mDNS `name_lookup`, `dns_lookup`, `dhcp_leases`), a `combined`
  scan, a `/discoveries` review queue with reconciliation, auto-import, and
  merge-on-rescan.
- **Phase 5 — Production hardening:** admin/viewer roles, TOTP MFA, OIDC SSO,
  encrypted secrets at rest, app-managed CA with rotation, `pg_dump` backup/restore,
  login lockout, session hardening, runtime-editable Settings.
- **Phase 6 — Advanced automation:** policy/health checks, scheduled scan windows,
  change webhooks, NetBox-compatible import/export, and a token-authenticated machine
  API `/api/v1` + `lightipam-cli`.
- **Post-1.0 (shipped in v1.1.0):** routing-aware scanner egress + agent diagnostics
  (ADR 0027) and schedule scope validation + last-run outcome (ADR 0028).
- **Post-1.1 (shipped in v1.2.0):** same-physical-device links (ADR 0029,
  migration 22) — suggest-and-confirm linking of a multi-homed device's per-subnet
  records; link-not-merge. Phase 2 shipped as ADR 0030 (migration 23): the SNMP
  ENTITY-MIB chassis serial persists as a hardware identity, an exact serial
  match is a gold-confidence link suggestion, and a Settings → Discovery toggle
  (default off) opts in to auto-linking serial matches at import time.
- **Post-1.2 (shipped in v1.2.1):** security and audit hardening from the
  2026-07-10 review, consistent API error responses, and a pinned published-image
  Compose installation path. No migration or scanner-protocol change.
- **Maintenance hardening (shipped in v1.2.2):** identity/recovery, request limits,
  concurrent containment, deployment/tooling and documentation fixes; see the
  [changelog](../../CHANGELOG.md#122---2026-11-17). Schema remains 23; API/protocol
  versions are unchanged. The CSS major migration remains open.

## Known limitations

This is the agent-facing snapshot of current implementation constraints.

- **IPv4 only** — no IPv6 anywhere in the stack.
- **SNMP is v2c only**; one read community per agent.
- The SNMP, LLDP/CDP, NetBIOS/mDNS, DNS, and DHCP backends are **unverified against
  real hardware** (hermetic unit tests only).
- nmap is **TCP-only** (no UDP/NSE); VLAN mapping covers the **access VLAN only**.
- DHCP ingestion reads a **mounted lease file** (ISC dhcpd / dnsmasq) — no API/SNMP source.
- **Online agent-pull cert enrollment is not built** (the one deferred Phase 5 item):
  the managed CA issues + rotates and the agent hot-reloads operator-deployed leaf
  keypairs; changed CA trust requires agent restart.

## Open findings and engineering debt

The 2026-07-10 audit findings are all fixed, closed, or investigated. See
[`docs/agent/findings/`](findings/README.md) for the closed historical tracker and
[`CHANGELOG.md`](../../CHANGELOG.md) for the fixes. Separate current engineering debt
includes the unresolved CSS toolchain audit path, database coverage not yet enabled
in the release job (PR/main CI now runs store regressions), MFA attempt-budget
policy, and unautomated encryption-key re-sealing. These are
tracked in the planning backlog, not as reopened July audit findings.

## What's next

**The maintainer requested a 36-item planning backlog; implementation is selected
one scoped task at a time.** See [Roadmap](../ROADMAP.md#planning-backlog--36-items)
for priorities, acceptance targets and approval gates, including the CSS toolchain
major migration. [Handoff](../HANDOFF.md) records operational constraints and supported
versus legacy methods. No roadmap entry authorizes a new dependency or an invariant
change, and no delivery dates are committed.

## Source of truth

When these disagree, resolve toward the more specific source and fix the drift:

| Question | Source of truth |
|----------|-----------------|
| What shipped, and when? | [`CHANGELOG.md`](../../CHANGELOG.md) |
| What is the roadmap / phase plan? | [`docs/ROADMAP.md`](../ROADMAP.md) |
| Why was a design decision made? | [`docs/adr/`](../adr) |
| Where are we now (agent-facing)? | **this file** |
| What audit findings are still open? | [`docs/agent/findings/`](findings/README.md) |
| What are the invariants / rules? | [`AGENTS.md`](../../AGENTS.md) |

**Maintenance rule:** any PR that ships or re-plans a feature must update this file,
`CHANGELOG.md`, and `docs/ROADMAP.md` together so they cannot contradict each other.
The `lightipam-doc-sync` skill checks this.
