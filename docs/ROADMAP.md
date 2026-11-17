# Roadmap

## Phase 1: Manual IPAM MVP — complete

- Authentication, sessions, and admin bootstrap.
- Default site, optional VLAN metadata, subnets, and sparse address records.
- Address status workflow: available, reserved, assigned, deprecated, conflict.
- Devices, MAC addresses, private MAC tagging, basic OUI vendor matching, tags, and custom fields.
- Subnet utilization and address grid.
- Address editing, navigation shell, dashboard widgets, empty states, and confirmation flows.
- Dashboard with global search, subnet widgets, review widget, recent changes, and scan status.
- Bulk edit and import/export foundation. *(Done in Phase 4.5 — see "Carried forward from earlier phases" below.)*
- Audit log.

## Phase 2: Scanner Agent Foundation — complete

- Scanner agent registration.
- App-to-agent mTLS.
- Allowed scan scopes.
- Manual and scheduled scan jobs.
- Immutable scan audit trail.
- Optional review mode for detected changes.

## Phase 3: Nmap Discovery MVP — complete

- ICMP and TCP host discovery. ✅
- Nmap-backed OS and service detection. ✅
- Findings review queue (`/discoveries`). ✅
- Last-seen tracking on managed addresses. ✅
- Conflict detection: discoveries are reconciled against managed records and
  flagged new/match/conflict (changed MAC, deprecated-but-responding, MAC seen
  on another address). ✅

Follow-ups merged on top of Phase 3 (#18):

- Per-agent auto-import for trusted agents (`scan_agents.auto_import`); conflicts
  always stay in the review queue. ✅
- Structured scan-result detail UI (per-host services/OS/evidence). ✅

## Phase 4: Network Context

- SNMP ARP-table harvesting (**done**, ADR 0006). The `arp_table` scan type
  queries a gateway/L3 device's `ipNetToMediaTable` over SNMP to recover IP↔MAC
  bindings for subnets the agent cannot reach at Layer 2. Unprivileged (UDP/161,
  no `NET_RAW`); reuses the discovery review-queue + reconciliation pattern.
- SNMP device inventory (**done**, ADR 0007). The `snmp_inventory` scan type reads
  a device's system group (name, OS, location) and its interface/IP tables to
  recover the device's own identity and the MACs of its interfaces.
- Combined all-sources scan + scan-experience polish (**done**, ADRs 0008/0009):
  one `combined` job runs deep nmap + ARP + SNMP inventory, merged per host, with
  unreachable SNMP ignored not failed; simplified Light/Standard/Deep modes;
  staged nmap (host discovery → service/OS on live hosts only); and dynamic,
  generous per-type scan timeouts.
- NetBIOS and mDNS/Bonjour hostname resolution (**done**, ADR 0010). The
  `name_lookup` scan type asks a host directly for its name over NetBIOS (UDP/137)
  and unicast mDNS (UDP/5353), recovering names for SMB, Apple, and IoT devices
  with no DNS PTR record; NetBIOS works across subnets. Folded into the `combined`
  scan as a best-effort enrichment pass. Unprivileged unicast UDP, no new
  dependency; reuses the discovery review-queue + reconciliation, in the agent.
- LLDP/CDP neighbor ingestion (**done**, ADR 0011). The `lldp_cdp` scan type reads
  a switch/router's LLDP (`lldpRemTable` + `lldpRemManAddrTable`) and CDP
  (`cdpCacheTable`) neighbor caches over SNMP to map physical topology — which
  devices are wired to which ports. Targets are the switch/router IPs; each neighbor
  with a management address becomes an IP-keyed observation (name, platform/OS, and
  an LLDP MAC-typed chassis id). Unprivileged (UDP/161, no `NET_RAW`); folded into
  the `combined` scan; reuses the discovery review-queue + reconciliation, in the
  agent.
- DNS forward/reverse enrichment (**done**, ADR 0012). The `dns_lookup` scan type
  resolves each host's name from the network's authoritative DNS (reverse PTR) and
  forward-confirms it, naming managed hosts that already have a DNS record and
  flagging a stale/mismatched PTR. Unprivileged UDP/TCP/53; folded into `combined`;
  reuses the discovery review-queue + reconciliation, in the agent.
- VLAN and interface mapping (**done**, ADR 0013). The `snmp_inventory` scan now
  reads each interface's 802.1Q access VLAN (Q-BRIDGE-MIB `dot1qPvid`, joined to the
  interface through the bridge-port table) and operational status, mapping a host's
  IP → interface → VLAN. A discovered VLAN backfills the containing subnet's VLAN
  when it has none (never overwriting an operator value), so the mapping reaches the
  Subnets and Devices pages. Unprivileged (UDP/161), no new scan type.
- DHCP lease ingestion (**done**, ADR 0014). The `dhcp_leases` scan type reads the
  DHCP server's lease file (ISC dhcpd or dnsmasq, mounted read-only on the agent) for
  the authoritative IP↔MAC binding and client-supplied hostname of each active lease.
  Opt-in and never fatal (an unconfigured file is a clear notice / muted Skipped
  line); folded into `combined`; reuses the discovery review-queue + reconciliation,
  in the agent.

**Phase 4 is complete.** The six Network-Context sources — SNMP ARP (`arp_table`),
SNMP inventory + VLAN/interface (`snmp_inventory`), NetBIOS/mDNS names
(`name_lookup`), DNS names (`dns_lookup`), DHCP leases (`dhcp_leases`), and LLDP/CDP
neighbors (`lldp_cdp`) — all merge per host through one review/import path, and a
single `combined` scan runs them all. The combined scan **enriches the hosts nmap
discovers** (ADR 0015): it expands a CIDR through the deep nmap stage, then runs the
per-host SNMP/name/DNS passes against the live hosts (concurrently, with an SNMP
short-circuit and collapsed skip-notices), so a combined scan of a CIDR recovers
MACs and SNMP inventory instead of degrading to nmap-only.

## Carried forward from earlier phases

A full audit (2026-06-17) found these items scoped to earlier phases that were not
yet built. They are unblocked (the data model is stable) and tracked here so they
are not lost between phases:

- **Bulk edit + CSV import/export (done, Phase 4.5).** Listed under Phase 1 and in
  `docs/MVP.md` ("Bulk edit and import/export should be available in the UI early")
  and backlog #4. Now built: multi-select bulk status/field/tag/delete on the
  Subnets/Addresses/Devices tables (JS-off + JS-on, audited, destructive actions
  through `confirm.html`), and CSV import/export of subnets, addresses, and devices —
  export columns match the forms, import validates **every** row against the same
  IPv4/overlap/sparse/state rules, shows a dry-run preview with per-row errors, and
  applies all-or-nothing in one transaction on confirm. The basic CSV on-ramp,
  distinct from the Phase 6 NetBox-compatible format. See ADR 0016.
- **Dashboard live widgets (done, 2026-06-17).** The Phase 1 "review widget" and
  "scan status" dashboard panels shipped as static placeholders (the scan panel still
  read "planned for Phase 2"). They are now wired to live data: the review queue shows
  the real pending-discovery count and links to `/discoveries`, and scan status lists
  recent jobs with their status badges.
- **Custom fields (done, Phase 5 audit, ADR 0019).** A Phase 1–5 audit (2026-06-19)
  found custom fields — listed under Phase 1 and in backlog #2/#5 — claimed merged but
  never actually built: the `custom_fields`/`custom_field_values` tables existed
  (migration 1) but no code or UI used them. Now built: an admin **Custom fields**
  Settings tab defines text fields per entity type (subnet/address/device), and each
  entity's form/detail page edits and shows the values (sparse storage, audited,
  zero-impact until a field is defined). No schema change. See ADR 0019.

## Phase 5: Production Hardening

Goal: make Light IPAM safe to run as the system of record in a real small-business or
enterprise environment — hardening identity, secrets, the certificate lifecycle, and
data durability. Keep the web app unprivileged; keep all elevated scan capability in
the agent.

### Identity & access

- **OIDC SSO (done, ADR 0018).** Authorization-code + PKCE via `go-oidc`/`oauth2`;
  state/PKCE/nonce in a sealed cookie, ID-token + nonce verified. `users.oidc_subject`
  (migration 16) binds the IdP subject to a local user; login resolves by subject, then
  username (linking only an unlinked account), then optionally auto-provisions a
  read-only viewer. Configured on the admin **Authentication** settings tab with the
  client secret sealed at rest.
- **MFA (TOTP) (done, ADR 0018).** RFC 6238 TOTP (stdlib, RFC-vector tested) with
  single-use recovery codes; the per-user secret is sealed at rest (migration 15). Login
  gains a sealed pending-MFA second step; self-service enrollment (QR + manual key),
  recovery-code display, and disable live on the `/account` page.
- **Roles beyond the single admin (done, ADR 0018).** `users.role` admin vs. read-only
  viewer (migration 14); a central middleware blocks viewers from all mutations, the
  Settings area is admin-only, and a **Users & Roles** tab manages accounts.
- **Session hardening (done, ADR 0017).** Idle **and** absolute session timeouts,
  per-session client IP + User-Agent capture, a tabbed **Settings page** (Security
  tab) listing active sessions with a "log out everywhere" control, and login
  throttling / account lockout (`login_attempts`, migration 12) keyed by username and
  client IP with a pure, unit-tested lock decision. All of this policy — lockout
  thresholds, session timeouts, and whether "log out everywhere" keeps the current
  device — is **editable at runtime** from the Settings page (persisted in
  `app_settings`, migration 13; env provides the boot defaults). The
  username-enumeration timing oracle in the login handler is closed with a decoy
  Argon2 verify. New auth audit events (`auth.login.failed`, `auth.login.locked`,
  `session.revoked_all`, `settings.security.updated`).

### Configurability & the settings panel

The Settings panel is backed by runtime `app_settings` values with pure validators,
cache refresh and audit events. Security, Users & Roles, Authentication, Agent
certificates, Backup & Restore, Custom fields, Policy and Notifications are shipped.
Discovery has the shipped hardware-serial auto-link toggle; its wider policy controls
remain future work. General and app-side Scanning tabs are not implemented. Native
CSV and NetBox exchange already live on Import / Export; a consolidated Data & Audit
tab and audit export/archival remain proposals. Backups are on-demand today, with
operator-scheduled `pg_dump` possible outside the app.

See [Settings](SETTINGS.md) for the current inventory. The planning backlog below
tracks remaining work without relabeling shipped features as planned. Agent-local
credentials, routing, lease paths and allowlists never move into this panel.

### Secrets & certificates

- **Managed agent certificate issuance + rotation (done, ADR 0018).** The app owns a CA
  (private key sealed at rest, migration 17) that signs short-lived agent/app mTLS
  leaves, replacing the hand-run dev CA as the issuing authority. The **Agent
  certificates** settings tab issues downloadable bundles (configurable CN/SANs/TTL),
  downloads the CA, and rotates the CA; the scanner agent hot-reloads a rotated leaf
  keypair under the same CA. A trust-root replacement requires an agent restart. Revocation relies on short TTLs (the accepted alternative to a CRL).
  *Follow-up: online agent-pull enrollment so an agent renews its own cert over a
  bootstrap channel without operator file deployment.*
- **Encrypted secrets at rest (done, ADR 0018).** `internal/secret` seals the TOTP
  secret, the OIDC client secret, and the managed-CA key with AES-256-GCM
  (`APP_ENCRYPTION_KEY`, or derived from `APP_SECRET`). SNMP communities remain
  **agent-local** by design and never reach the app DB.
- **Rotation for the app's own keys** — documented in `docs/KEY_ROTATION.md`
  (`APP_SECRET`, `APP_ENCRYPTION_KEY`, managed CA, and leaf certs).

### Data durability & operations

- **Backup & restore (done, ADR 0018).** `internal/backup` takes on-demand `pg_dump`
  (custom-format) snapshots into `BACKUP_DIR`, each filename capturing the schema-
  migration version. The admin **Backup & Restore** tab creates/lists/downloads/deletes
  them; restore is documented and scripted (`docs/BACKUP_RESTORE.md`,
  `deploy/restore.sh`) with migrations rolling an older dump forward on boot. The app
  image bundles `postgresql16-client` and keeps zero Linux capabilities.
- **Readiness/health depth (done, ADR 0017).** `/healthz` stays the liveness probe;
  `/readyz` is added to ping the DB and report the applied-migration version (503 when
  the DB is unreachable), and the app compose service health-checks it. Agent
  reachability in the probe remains a future enhancement.
- **Disaster-recovery runbook (done).** `docs/DISASTER_RECOVERY.md` covers compose,
  volumes (`pgdata`/`backups`), secrets, and scanner-agent certificates.

### Multi-tenancy (only if needed)

- Organization/tenant separation of IPAM data, users, and agents — deferred unless a
  deployment requires it.

**Exit criteria — met (ADRs 0017 + 0018).** An operator can stand up Light IPAM with
SSO + MFA, a managed CA that issues short-lived agent certs which the agent hot-reloads
on rotation, secrets sealed at rest, and a tested backup/restore path — on top of
admin/viewer roles, login throttling, session hardening, and a runtime-editable
Settings panel. The one explicitly-deferred increment is **online agent-pull
certificate enrollment** (the agent renewing its own cert over a bootstrap channel);
today the managed CA issues and the agent hot-reloads operator-deployed certs. Phase 5
is complete with online renewal explicitly deferred; Phase 6 is also complete below.

## Phase 6: Advanced Automation

- **Policy checks (done, ADR 0020).** A read-only **Policy / Health** view (`/policy`)
  that runs hygiene checks on demand: overlapping subnets (an invariant verifier — the
  create/import paths already block overlaps), stale managed addresses/devices not seen
  within a configurable threshold (device staleness derived from linked addresses'
  `last_seen_at`, so no schema change), and unmanaged/conflicting discovered services
  (reusing the `scan_discoveries` reconcile classification). Pure, unit-tested check
  functions over store snapshots; findings grouped by severity and linked to the
  offending record; a dashboard count widget; and a runtime-editable **Policy** Settings
  tab (enable/disable each check, stale threshold, include-never-seen) following the
  `app_settings` pattern. App-side only, no new privilege, no client JS, no migration.
- **Scheduled scan windows (done, ADR 0021).** A schedule may restrict firing to a
  window — a time-of-day range plus a weekday set, read in the schedule's own IANA
  timezone — layered on top of the existing interval cadence. A due schedule outside
  its window is skipped that tick and re-checked on the next one (`next_run_at` is not
  advanced), so it fires once the window opens. The decision is a pure, unit-tested
  `windowAllows` (handles midnight-wrap, no-time/no-day, and the empty-window =
  always-allowed back-compat case); migration 18 adds `window_start_min` /
  `window_end_min` (minutes since midnight, NULL = no restriction), `window_days`
  (0=Sun..6=Sat, empty = any day), and `window_tz` (default UTC). The schedule form
  gains native time/day/timezone fields (no client JS) and the schedules table shows
  the window. `cmd/server` embeds `time/tzdata` so zone lookups work on the Alpine
  image. Per-schedule config, no Settings tab, no new privilege.
- **Change webhooks (done, ADR 0022).** An admin registers outbound webhook endpoints
  on a **Notifications** settings tab; the app POSTs an HMAC-signed JSON payload to each
  enabled, subscribed endpoint when a matching change is audited. The audit log is the
  change feed — a single mutex-guarded `store` audit hook (on both store instances) fans
  events out via `internal/webhook`, with a pure `categoryForAction` mapping audited
  actions to subscribable categories (ipam / discovery / scan / security). Delivery is
  asynchronous and gated to a no-op when no webhook is enabled; each attempt is recorded
  in a bounded `webhook_deliveries` log (migration 19 adds `webhooks` + that log). The
  per-webhook signing secret is sealed at rest (`internal/secret`); pure
  `parseWebhookForm`/`categoryForAction`/`sign` are unit-tested and an httptest test
  covers the real POST + signature. App-side only, no new privilege, no client JS.
- **NetBox-compatible import/export (done, ADR 0023).** A **NetBox** CSV format
  alongside the native one: a per-type format selector + Export NetBox links. A NetBox
  upload is a **pure, unit-tested translation** (`translateNetBoxImport`) into the
  canonical Light IPAM columns, so it reuses the exact same dry-run preview, validators,
  and all-or-nothing transactional apply — no second pipeline. Exports emit NetBox
  column names (prefixes / IP addresses / devices) with values mapped back. The model
  edges that don't align (prefix has no name, devices need role/type/site) are
  documented and lossy by design; the IPAM core round-trips. See `docs/NETBOX.md`. No
  schema change, no new privilege, no client JS.
- **Machine API + CLI (done, ADR 0024).** A token-authenticated JSON API under
  `/api/v1` (subnets, addresses, devices CRUD + `whoami`) and a stdlib-only
  `lightipam-cli` that consumes it. Per-user **bearer API tokens** (migration 20, SHA-256
  hashed, shown once, created by admins on their Account page) carry the owner's role, so the
  existing admin/viewer authorization applies: reads need any valid token, writes need an
  admin token. The API reuses the existing store methods + validation, is audited, and
  fans out to change webhooks like the UI. The user chose a CLI over a Terraform provider
  (lightweight, in-repo, no heavy dependency); a provider can be added later against the
  same stable API. See `docs/API.md`. No client JS, app stays unprivileged.

**Phase 6 (Advanced Automation) is complete.** Policy/health checks (ADR 0020),
scheduled scan windows (ADR 0021), change webhooks (ADR 0022), NetBox-compatible
import/export (ADR 0023), and the machine API + CLI (ADR 0024) are all merged.

## Discovery UX follow-up

- **Subnet auto-create on import + "Import all" (done, ADR 0026).** Importing a
  discovered host with no managed subnet opens a server-rendered, pure-CSS modal
  pre-filled with the exact network the scan targeted (the containing scan-job target
  CIDR, falling back to a `/24` only for a bare-IP scan) and the scanned VLAN; on save
  the subnet is created and the import resumes. An **Import all** control imports the
  whole pending, non-conflicting `/discoveries` queue in one click, grouping any missing
  subnets by their suggested network and prompting for each (re-checking until none remain) before
  importing everything. App-side only, no migration, no new privilege; reuses the
  existing `CreateSubnet` + `ImportDiscovery` paths and their audit events.

## Post-1.0 hardening (shipped in v1.1.0)

- **Routing-aware scanner egress + scan diagnostics (done, ADR 0027).** A field
  report showed that a macvlan scanner (which pins every nmap probe to the macvlan
  source IP) silently finds **zero hosts** when pointed at a *routed* subnet, because
  the source pin and the kernel route disagree — the scan returns `succeeded` with no
  observations. Egress is now **routing-aware by default**: `AGENT_SCAN_PIN_MODE`
  (default `auto`) pins a target only when it is L2-adjacent to the scan source
  interface and lets routed targets follow the default route, so one macvlan agent
  handles same-subnet *and* routed scans **with no operator config change** (`always`
  restores the old unconditional pin; `off` disables it). Alongside: self-explaining
  zero-host/mismatch notices, an agent `GET /diagnostics` view surfaced on
  `/agents/{id}`, classified app→agent dispatch errors (DNS / TCP / TLS / HTTP — the
  confusing `lookup scanner-agent ... no such host` becomes "container not running"), a
  `scanner-agent --healthcheck` + Compose healthcheck, and docs (bridge-vs-macvlan
  decision matrix, troubleshooting, a one-scanner-per-VLAN example). App stays
  unprivileged, no client JS, no new mandatory dependency, no migration.
- **Schedule scope validation + last-run outcome (done, ADR 0028).** A schedule saved
  with a configuration the agent rejects at dispatch (e.g. a mistyped CIDR) used to fail
  silently on every tick. Scan scope is now validated at save time on both the scan and
  schedule forms (matching the agent's dispatch rules and allowlist containment), and
  each schedule records its most recent run outcome (succeeded / failed / rejected) and
  failure reason in a **Last run** column (migration 21, additive). App-side only, no
  new privilege, no client JS.

## Device correlation (multi-homed devices) (shipped in v1.2.0)

- **Phase 1 — same-physical-device links (done, ADR 0029, migration 22).** A
  reversible link layer over existing device records: the device page suggests
  high-confidence same-hardware matches (identical hostname + same OS family +
  disjoint subnets; `os_detail` deliberately excluded) for the operator to confirm
  or dismiss, with manual link/unlink always available. Link-not-merge — each
  record keeps its own address in its own subnet — and never automatic. Import
  merge-on-MAC is unchanged.
- **Phase 2 — persisted SNMP hardware identity (done, ADR 0030, migration 23).**
  `snmp_inventory`/`combined` scans read the ENTITY-MIB chassis serial (vendor
  placeholders filtered) and `sysObjectID`, persist both through
  `scan_discoveries` onto devices, and use an exact serial match across disjoint
  subnets as a **gold-confidence** link suggestion — plus an opt-in
  Settings → Discovery toggle (default off) that auto-links serial matches at
  import/sync time (`device.link.auto`). The SNMPv3 engine ID stays out of scope
  until SNMPv3 support exists (the stack is v2c-only).

## Maintenance hardening (shipped in v1.2.2)

The November maintenance release is recorded in [Changelog](../CHANGELOG.md#122---2026-11-17).
It preserves schema migration 23, `/api/v1` and scanner protocol `v1`. Compatible
CSS patches are partial remediation; the major migration below remains open.

## Planning backlog — 36 items

Updated 2026-11-17. This is the forward-looking backlog requested by the maintainer,
separate from the completed phases above. **None of these 36 items is represented as
shipped.** Priority is proposed sequencing: P1 foundations/risk, P2 product and
operational improvements, P3 optional expansion. No dates or implementation capacity
are committed. Select a scoped task before implementation; this roadmap does not
approve new dependencies, security-policy changes or architecture-invariant changes.

The 2026-11 hardening fixes belong in [Changelog](../CHANGELOG.md), not in this open
backlog. In particular, further concurrency qualification builds on the containment
fix; CSS migration remains open after the compatible lockfile refresh. Advisory
counts are a dated observation, not a permanent security score.

| ID | Priority | Improvement and acceptance target | Prerequisite / decision |
| --- | --- | --- | --- |
| <a id="r01"></a>R01 | P1 | **PostgreSQL release and full-stack integration gates.** Build on the PostgreSQL-enabled PR/main CI by adding release-job database coverage and service/API integration tests, isolated schemas and failures on unexpected skips. | Approve runner cost; PR/main store CI is implemented, release-job database provisioning is not. |
| <a id="r02"></a>R02 | P1 | **Subnet overlap and containment qualification.** Extend the new containment coverage to concurrent overlapping creates/updates, cancellation, lock ordering and representative large imports; prove bounded completion without stranded records. | Keep native network types and global overlap policy; direct SQL is outside the advisory-lock protocol. |
| <a id="r03"></a>R03 | P1 | **OIDC issuer identity binding.** Threat-model issuer plus subject identity, issuer reconfiguration and migration of existing bindings; implement only an approved design with negative identity-collision tests. | Security/ADR decision; current subject lookup alone is not an issuer migration policy. |
| <a id="r04"></a>R04 | P1 | **Explicit first-login OIDC linking policy.** Choose proof/consent rules for linking an unbound local account by username, including admin accounts and colliding claims; document and test the selected policy. | Security/ADR decision; preserving existing bindings does not remove first-link trust. |
| <a id="r05"></a>R05 | P1 | **CSS toolchain major migration.** Evaluate and migrate the approved Tailwind toolchain to remove the unresolved picomatch path and bundled stale browser data. Verify clean installation, advisory resolution, deterministic CSS and representative light/dark, responsive and JS-off screens. | Approve major-version changes and any new packages first; do not suppress audit output or treat the partial patch as complete. |
| <a id="r06"></a>R06 | P1 | **MFA verification attempt budgets.** Design atomic, bounded TOTP/recovery-code verification attempt limits, failure audit and lockout/recovery behavior; test concurrent and distributed attempts without weakening recovery access. | Security-policy decision; single-use recovery codes do not themselves provide attempt throttling. |
| <a id="r07"></a>R07 | P1 | **Real-hardware discovery qualification.** Test SNMP ARP/inventory/serial/access VLAN, LLDP/CDP, NetBIOS/mDNS, DNS and both DHCP lease formats in an authorized lab; commit sanitized fixtures and publish a support matrix. | Hardware access and scan authorization; hermetic tests alone do not establish hardware support. |
| <a id="r08"></a>R08 | P1 | **Bridge, macvlan and routed egress qualification.** Exercise adjacent/routed scopes, auto/always/off pin modes, missing routes, source mismatch and zero-host diagnostics; publish verified deployment recipes. | Authorized lab; all probing remains in scanner-agent under both allowlists. |
| <a id="r09"></a>R09 | P1 | **Automated restore drills.** Restore a fixture backup into an isolated database, migrate it forward, check inventory/audit counts and encrypted-secret access, and record recovery timings. | Never overwrite a user deployment; retain matching encryption material. |
| <a id="r10"></a>R10 | P1 | **Certificate expiry visibility.** Surface agent/app/CA expiry with actionable renewal thresholds; test expired/not-yet-valid chains, operator bundle distribution and hot reload. | Reuse managed CA; choose alert delivery policy separately. |
| <a id="r11"></a>R11 | P1 | **Release upgrade qualification.** Exercise prior supported images to a candidate, persistent volumes, secrets, readiness and protocol compatibility; document restore-based recovery and unsupported downgrades. | Define supported upgrade range; no destructive migration without required approval. |
| <a id="r12"></a>R12 | P1 | **Safe encryption-key rotation and recovery.** Design an audited re-seal or key-ring migration for TOTP, OIDC, managed CA and webhook secrets; test wrong-key startup, recovery-code behavior and fail-closed CA loading without replacement of recoverable state. | Security/ADR decision; no built-in complete re-seal workflow exists today. |
| <a id="r13"></a>R13 | P2 | **General Settings tab.** Select and implement instance name, default site, page size and date/time/theme defaults with pure validators, cache refresh and audit events. | Choose fields first; this tab is not implemented. |
| <a id="r14"></a>R14 | P2 | **App-side scanning defaults.** Apply safe default scan type/depth/timeout and target hints consistently to manual/scheduled forms, showing effective values without widening agent bounds. | Agent-local credentials, routes and allowlists remain outside Settings. |
| <a id="r15"></a>R15 | P2 | **Discovery queue filtering and bulk workflow.** Improve agent/subnet/classification/age filtering, selection persistence and action previews while keeping conflicts in individual review. | Reuse existing reconciliation and import paths; no silent conflict overwrite. |
| <a id="r16"></a>R16 | P2 | **Discovery evidence provenance.** Explain source, observation time, imported value and conflicting evidence; make serial-based link suggestions understandable and reversible. | Inspect existing evidence before proposing additive schema changes. |
| <a id="r17"></a>R17 | P2 | **Last-seen and stale-state clarity.** Distinguish never-seen, stale, failed-scan and recently observed records; link policy thresholds to relevant scan outcomes without deleting records automatically. | Reuse existing Policy settings; no new retention behavior implied. |
| <a id="r18"></a>R18 | P2 | **Device-link management usability.** Improve linked-member navigation, hardware identity display, dismiss/confirm rationale and reversible unlink flows across subnets. | Preserve link-not-merge and ADRs 0029/0030. |
| <a id="r19"></a>R19 | P2 | **Accessibility qualification.** Audit and fix keyboard navigation, form labels/errors, focus, tables/modals, contrast, zoom and JS-off core flows with reproducible acceptance evidence. | Approve any new testing dependency; preserve strict CSP. |
| <a id="r20"></a>R20 | P2 | **Dense tables and small-screen usability.** Verify navigation, overflow, bulk controls, long names/CIDRs, empty states and column preferences across representative viewport sizes. | Use server-rendered templates and existing progressive enhancement. |
| <a id="r21"></a>R21 | P2 | **Visual system and brand consistency.** Audit existing logo/icon usage, typography, spacing, status colors, light/dark states and authentication/settings screens; implement scoped approved corrections. | Retain the existing brand unless a replacement is explicitly selected. |
| <a id="r22"></a>R22 | P2 | **Search and operational filters.** Improve match context, query persistence and cross-entity navigation; measure representative inventory performance and scope saved filters before adding storage. | Decide saved-filter ownership and persistence; do not invent multi-tenancy. |
| <a id="r23"></a>R23 | P2 | **API pagination and filtering.** Add bounded, deterministically ordered list queries and useful filters with documented compatibility behavior and CLI support. | Stable /api/v1 compatibility review; no silent response-shape changes. |
| <a id="r24"></a>R24 | P2 | **API and CLI compatibility coverage.** Specify actual update/null/error semantics and test authorization, validation, not-found cases and CLI failures against a service. | Machine-readable specification optional; approve added tooling separately. |
| <a id="r25"></a>R25 | P2 | **API token lifecycle controls.** Select expiry, last-used visibility and revocation rules; evaluate scopes/service accounts, test owner-role changes and audit lifecycle operations. | Security decision for new credential types; viewer issuance policy must be explicit. |
| <a id="r26"></a>R26 | P2 | **Import preview and diagnostics.** Show actionable row/column errors, NetBox loss/mapping warnings and accurate preview/apply differences while preserving all-or-nothing apply. | Reuse canonical CSV validation and translation; no parallel importer. |
| <a id="r27"></a>R27 | P2 | **Reliable webhook delivery design.** Specify durable delivery guarantees, bounded retry/backoff, identifiers, replay and sanitized diagnostics; test transient and permanent failures. | ADR/storage decision; preserve signed payloads, redirect rejection and append-only audit feed. |
| <a id="r28"></a>R28 | P2 | **Operational diagnostics and correlation.** Document structured log fields, safe correlation identifiers, scheduler state, DB readiness and classified agent failures; provide redacted diagnostic collection. | Metrics endpoints only after need/scope decision; keep readiness distinct from liveness. |
| <a id="r29"></a>R29 | P2 | **Backup scheduling and file retention.** Add audited scheduling, bounded backup-file retention, failure status and concurrency controls; test full disk and failed dumps. | Choose off-host responsibility; backup file cleanup must not delete audit records. |
| <a id="r30"></a>R30 | P2 | **Audit export and archival policy.** Provide bounded filtered export with stable ordering and actor/target context; document growth and archival without mutating append-only records. | Any deletion/retention exception requires ADR and invariant approval. |
| <a id="r31"></a>R31 | P3 | **Online agent-pull certificate renewal.** Design bootstrap trust, identity binding, overlap, expiry and compromise recovery; test failed renewal and enrollment before replacing operator deployment. | Explicit security/ADR approval; this deferred Phase 5 feature is not shipped. |
| <a id="r32"></a>R32 | P3 | **Agent-local SNMPv3.** Choose supported authentication/privacy combinations, agent-only secret handling and redacted diagnostics; qualify real devices. | ADR/dependency approval if needed; app must not store SNMP credentials. |
| <a id="r33"></a>R33 | P3 | **VLAN and interface context expansion.** Scope tagged/trunk membership and richer port status beyond existing access PVID, with freshness/vendor-gap handling and qualified switch fixtures. | Choose supported models; no trunked network access in the app. |
| <a id="r34"></a>R34 | P3 | **OUI data refresh workflow.** Select a licensed/provenanced vendor dataset and reproducible update process; validate prefixes and preserve private-MAC classification. | Approve data source, distribution and maintenance obligations. |
| <a id="r35"></a>R35 | P3 | **Terraform provider feasibility.** Define identity/import/drift/delete semantics and prototype against the stable API, including overlaps and lifecycle failures. | Separate provider/dependency/maintenance decision; not an app dependency. |
| <a id="r36"></a>R36 | P3 | **Dependency, image and deprecation support policy.** Define supported Go/Node/PostgreSQL/Alpine versions, advisory/update cadence and exception expiry; maintain a register of old namespaces, development methods and compatibility modes with migration guidance before removals. | Maintainer support/dependency decision; no method is deprecated merely by appearing here. |

### Selecting the next task

Start with evidence and an acceptance test; avoid duplicating an open issue/PR.
Security-policy, discovery-privilege, schema and dependency decisions retain the
approval gates in [AGENTS.md](../AGENTS.md). Real-hardware qualification needs an
authorized lab. Product polish should use the existing server-rendered UI/brand.
The [engineering handoff](HANDOFF.md) links these IDs and records current operational
constraints; it does not keep a second copy of the acceptance criteria.
