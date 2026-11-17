# Architecture

## Goals

Light IPAM should be small enough to run in a single Docker Compose stack, but structured enough to safely discover networks without giving the web application raw network privileges.

## Components

### Web/API App

The app owns:

- User authentication and authorization.
- The default/site association, optional subnet VLAN metadata, sparse IPv4 subnets
  and addresses, reservations/statuses, devices and MAC records. VRF-aware inventory
  is not implemented; subnet overlaps are blocked globally.
- Scan policies, schedules, and result review.
- Audit logs and configuration history.
- Reporting and export.
- Runtime configuration — a tabbed **Settings** panel backed by the `app_settings`
  store (env values seed boot defaults). See `docs/SETTINGS.md`. Agent-local secrets
  and raw-socket configuration are deliberately **not** part of this panel; they stay
  on the scanner agent.

The app should run as an unprivileged container with no Linux capabilities.

### Scanner Agent

The scanner agent owns:

- Staged nmap host discovery (ICMP/ARP) then TCP service and OS fingerprinting on
  live hosts. (Privileged probing via `NET_RAW`, confined here.)
- SNMP ARP-table harvesting (`arp_table`), SNMP device inventory with 802.1Q
  VLAN/interface mapping (`snmp_inventory`), and LLDP/CDP neighbor harvesting
  (`lldp_cdp`) over unprivileged UDP/161 — implemented.
- NetBIOS + unicast mDNS host-name resolution (`name_lookup`) over unprivileged
  UDP/137 and UDP/5353 — implemented.
- DNS reverse/forward enrichment (`dns_lookup`) over UDP/TCP/53 and DHCP
  lease-file ingestion (`dhcp_leases`, ISC dhcpd/dnsmasq, mounted read-only) —
  implemented; both unprivileged.
- A combined scan that fuses nmap with every passive pass (both SNMP passes, the
  name lookup, DNS, DHCP, and the LLDP/CDP harvest) per host, enriching the hosts
  nmap discovers.

Agents are deployed near the networks they scan. The app sends scan jobs over
mTLS (each carrying an explicit IPv4 allowlist), and agents return normalized
observations that flow through the discovery review queue.

### Database

PostgreSQL is the source of truth, with native `inet`, `cidr` and `macaddr` columns.
Application writes use parameterized containment/overlap queries (`cidr && $1`);
there is no VRF-scoped overlap exception. Subnet CIDR edits and application address
membership writes coordinate through transaction advisory locks and fresh containment
checks. These locks are not a database trigger for arbitrary external SQL.

## Discovery Pipeline

1. Define explicit IPv4 target scopes and per-agent allowlists; the app and agent
   each validate containment. There is no VRF scope selector.
2. Agent performs low-impact host discovery.
3. Agent performs policy-limited port/service scans.
4. Agent normalizes observations into host, address, service, OS, and evidence records.
5. App presents changes as reviewable findings before mutating critical IPAM records.

## UI direction and implemented surfaces

The inventory, search, dashboard, discovery, device, schedule and Settings surfaces
are server-rendered. The list below describes design direction as well as existing
views; it does not imply that a subnet tree, topology viewer, every filter or a
general-purpose policy editor has shipped. Remaining usability work is in Roadmap.

The UI should combine a beautiful dashboard with dense operational views:

- Dashboard with global search, discovery review, subnet health, utilization, recent changes, and scan status widgets.
- Subnet tree with utilization and conflict indicators.
- Address grid with filters for status, device, owner, services, and last seen.
- Device detail page with addresses, services, scan history, and evidence.
- Scan policy editor with explicit scope, rate limits, and probe types.
- Review queue for newly detected hosts, conflicts, and stale records.
- A tabbed Settings panel for configuring the app (security, scanning defaults,
  discovery, agents, and more — see `docs/SETTINGS.md`).

Visual direction: Apple-inspired restraint, strong spacing, polished dark mode, and database-style information density where operators need to compare many records.
