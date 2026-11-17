# Engineering and product handoff

This is the entry point for the next maintainer or coding agent. Updated 2026-11-17.
Release/migration facts live in [Project state](agent/PROJECT_STATE.md); shipped
changes live in [Changelog](../CHANGELOG.md); the **36-item planning backlog** lives
in [Roadmap](ROADMAP.md#planning-backlog--36-items). Do not copy their state into agent
contracts or turn proposed work into a claim that it shipped.

## Start here

1. Read [AGENTS.md](../AGENTS.md), [Project state](agent/PROJECT_STATE.md),
   [Validation](agent/VALIDATION.md) and [Review checklist](agent/CODE_REVIEW.md).
2. Inspect the current branch, clean/dirty state, remotes, recent commits and live
   GitHub issues/PRs. Verify the connected identity and repository before remote
   mutations. The canonical repository is `whitmerb/LightIPAM`; old GitHub links may
   redirect, but GHCR image pulls use the explicit owner.
3. Read the relevant product/runbook docs and ADRs. Scanner/discovery work requires
   all three `SCANNER_*` guides. State the problem, assumptions, non-goals, smallest
   safe change and acceptance evidence before editing.
4. Use the [focused skills](../.agents/skills): GitHub workflow for lifecycle actions,
   bugfix, scanner-change, review, doc-sync and release-prep as applicable. Keep one
   task per functional branch from current main. A prior maintainer instruction can
   authorize merges/releases; do not infer that permission for unrelated later work.

## Product and boundaries

LightIPAM serves small-business/homelab IPv4 inventory: sparse addresses, globally
blocked subnet overlaps, devices/MACs, tags/custom fields, import/export, a stable
JSON API/CLI and a server-rendered web UI. Discovery observations enter a review
queue; only explicitly trusted agents may auto-import non-conflicting observations.
Multi-homed device correlation links records rather than merging them.

The app has zero Linux capabilities and no scanning tools. Active/unprivileged
network discovery both belong to scanner-agent; only its nmap backend needs
`NET_RAW`. App-to-agent transport is mTLS with independent app/agent allowlists.
PostgreSQL native network types, append-only audit and strict CSP remain contracts,
not roadmap options. No framework or new dependency is implicitly authorized.

Read [Architecture](ARCHITECTURE.md), [Security](SECURITY.md),
[Discovery](SCANNER_DISCOVERY.md), [API](API.md), [Settings](SETTINGS.md) and
[Branding](BRANDING.md) for implementation details.

## Recent hardening and remaining limits

The November hardening covers OIDC binding replacement, webhook redirects, complete
bounded JSON bodies, current-owner release images, compatible CSS dependency patches,
concurrent subnet/address containment, single-use recovery codes under contention,
consistent CSV import limits, and PostgreSQL regressions enabled in PR/main CI.
The changelog is the canonical list.

Important limits for follow-up work:

- First-time SSO username linking still trusts the configured claim; issuer changes
  need a deliberate identity-migration policy. Existing subject bindings cannot be
  replaced through the username fallback.
- Literal-IP outbound URL guards intentionally permit private LANs and do not resolve
  DNS; webhook delivery refuses redirects. Do not silently alter that policy.
- Containment advisory locks coordinate application membership writers with subnet
  edits. Manual SQL bypasses them, edits serialize globally, and long imports can
  delay edits. Existing bad data is not automatically repaired.
- The CSS refresh is **partial audit remediation**, not Tailwind 4. At the
  2026-11-17 audit, 10 high findings remained on the picomatch path; npm's suggested
  full remedy was a major toolchain migration. Re-run audit before acting rather
  than treating this count as current forever. Browser data in the existing bundled
  toolchain also warns as stale.
- PR/main CI provisions PostgreSQL and runs store regressions. Local runs still
  need an explicit test URL; the tag-triggered release job does not provision one.
  Report local database evidence and release-job limitations accurately.
- Discovery is IPv4/SNMPv2c/TCP-nmap only; real-hardware qualification is outstanding.
  Online agent-pull renewal, SNMPv3 and richer tagged-VLAN context are not shipped.
- Complete encryption-key re-sealing is not automated. Do not rotate keys by merely
  restarting against a production database; review [Key rotation](KEY_ROTATION.md).
  Database sessions/API tokens require explicit revocation independent of key changes.

## Improvement index

These are links to the same 36 roadmap items, not an additional backlog or an
implementation commitment. Acceptance targets, priority and approval gates are
maintained only in Roadmap.

| IDs | Work area | Roadmap items |
| --- | --- | --- |
| R01–R06 | Security, data integrity and build maintenance | [R01: PostgreSQL release and full-stack integration gates](ROADMAP.md#r01); [R02: Subnet overlap and containment qualification](ROADMAP.md#r02); [R03: OIDC issuer identity binding](ROADMAP.md#r03); [R04: Explicit first-login OIDC linking policy](ROADMAP.md#r04); [R05: CSS toolchain major migration](ROADMAP.md#r05); [R06: MFA verification attempt budgets](ROADMAP.md#r06) |
| R07–R12 | Qualification, recovery and release readiness | [R07: Real-hardware discovery qualification](ROADMAP.md#r07); [R08: Bridge, macvlan and routed egress qualification](ROADMAP.md#r08); [R09: Automated restore drills](ROADMAP.md#r09); [R10: Certificate expiry visibility](ROADMAP.md#r10); [R11: Release upgrade qualification](ROADMAP.md#r11); [R12: Safe encryption-key rotation and recovery](ROADMAP.md#r12) |
| R13–R18 | Settings and discovery workflows | [R13: General Settings tab](ROADMAP.md#r13); [R14: App-side scanning defaults](ROADMAP.md#r14); [R15: Discovery queue filtering and bulk workflow](ROADMAP.md#r15); [R16: Discovery evidence provenance](ROADMAP.md#r16); [R17: Last-seen and stale-state clarity](ROADMAP.md#r17); [R18: Device-link management usability](ROADMAP.md#r18) |
| R19–R24 | Accessibility, visuals and API ergonomics | [R19: Accessibility qualification](ROADMAP.md#r19); [R20: Dense tables and small-screen usability](ROADMAP.md#r20); [R21: Visual system and brand consistency](ROADMAP.md#r21); [R22: Search and operational filters](ROADMAP.md#r22); [R23: API pagination and filtering](ROADMAP.md#r23); [R24: API and CLI compatibility coverage](ROADMAP.md#r24) |
| R25–R30 | Credentials, integrations and operations | [R25: API token lifecycle controls](ROADMAP.md#r25); [R26: Import preview and diagnostics](ROADMAP.md#r26); [R27: Reliable webhook delivery design](ROADMAP.md#r27); [R28: Operational diagnostics and correlation](ROADMAP.md#r28); [R29: Backup scheduling and file retention](ROADMAP.md#r29); [R30: Audit export and archival policy](ROADMAP.md#r30) |
| R31–R36 | Optional discovery/integration expansion and compatibility | [R31: Online agent-pull certificate renewal](ROADMAP.md#r31); [R32: Agent-local SNMPv3](ROADMAP.md#r32); [R33: VLAN and interface context expansion](ROADMAP.md#r33); [R34: OUI data refresh workflow](ROADMAP.md#r34); [R35: Terraform provider feasibility](ROADMAP.md#r35); [R36: Dependency, image and deprecation support policy](ROADMAP.md#r36) |

## Tooling and verification

Use the pinned npm lockfile and committed generated CSS; edit Tailwind source, then
rebuild. Current tooling uses Tailwind 3, Node 22 in image builds, Go 1.25 from
`go.mod`/Dockerfiles and PostgreSQL 16. Do not move to Tailwind 4 by forcing
`npm audit fix`; [R05](ROADMAP.md#r05) requires an explicit migration with dependency,
CSS/visual and CSP checks. Existing Go import/module identity
`github.com/devSealWare/LightIPAM` is a compatibility identifier; repository transfer
alone does not authorize rewriting it.

Run `./scripts/verify.sh --docker` and the relevant targeted/runtime tests. Enable
`LIGHTIPAM_TEST_DATABASE_URL` against an isolated database for store changes and
report unexpected skips. The validation guide gives commands and tier scope. Use
sanitized fixtures; never commit certificates, secrets, node_modules or root binaries.

## Supported, legacy and discouraged methods

| Method | Actual status | Use instead / caution |
| --- | --- | --- |
| `compose.release.yaml` with exact version | Supported published-image path | Use `ghcr.io/whitmerb` app/scanner at matching versions; preserve project/volume names on upgrade. |
| `compose.yaml` source builds | Supported developer workflow, not deprecated | Production deployments should use reviewed published images and operator secrets. |
| Old `ghcr.io/devsealware/*` image references | Obsolete deployment namespace | Update both image owners; a GitHub repository redirect does not repair image pulls. |
| `cmd/scanner-certs` development CA | Supported for development, not deprecated | Use managed CA/operator-deployed bundles in production. Online enrollment is not implemented. |
| CLI `--insecure` | Supported development escape hatch, unsafe for production | Configure trust and verify TLS; do not recommend disabling verification to fix production cert errors. |
| Scanner protocol `passive` mode | Still accepted no-packet compatibility mode | It is hidden in the UI, not removed or formally deprecated. |
| Address state `deprecated` | Supported inventory status | It is not a statement that an API/method is deprecated. |
| `User.IsAdmin` alongside `role` | Internal compatibility field | `role` is authoritative; do not remove the compatibility field without a scoped migration. |
| `HANDOFF_CLAUDE.md` | Compatibility entry point | The product-neutral handoff is this file. |

No stable API route or scanner-protocol version is formally deprecated by this
handoff. Declare support windows and migration guidance before removing behavior.

## Merge, release and image publishing

Follow the [GitHub workflow skill](../.agents/skills/lightipam-github-workflow/SKILL.md)
and [release guide](RELEASING.md). Recheck exact head SHA, CI, review requests,
unresolved threads and mergeability before each authorized squash merge. Use
`type(scope): summary (#PR)` as the squash subject; delete the remote branch after
verified merge and fast-forward main. Keep unrelated local changes intact.

Merging main runs CI/CodeQL, **not image publishing**. The existing release workflow
runs only when a `v*` tag is pushed, then tests and publishes both architectures for
both images. Verify the run and registry manifests before announcing images. No
manual Docker push is part of the supported release procedure. A GitHub Release is
separate from image publication and should link the tagged changelog and evidence.
