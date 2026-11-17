# Historical audit findings tracker

Historical tracker for the 2026-07-10 audit. Those findings were fixed, closed or
investigated and their individual files removed; shipped outcomes are recorded in
[Changelog](../../../CHANGELOG.md). This directory is not a claim that there is no
other engineering debt. Current improvements live in [Roadmap](../../ROADMAP.md).

Once a finding is fixed, delete its file from this directory and fold a one-line
mention into `CHANGELOG.md` under the release that shipped the fix (see
[Documentation sync](../../../AGENTS.md#documentation-sync)) — don't let a fixed
finding linger here alongside shipped history.

## Open July audit findings

None. Later hardening and engineering debt are recorded in the changelog/roadmap.

## Priority key

- **High** — realistic exploit path with low attacker effort; fix soon.
- **Medium-High** — real risk but needs a specific precondition (admin role, specific deployment choice).
- **Medium** — worth fixing, not urgent; often a defense-in-depth or config-default question.
- **Low-Medium** — no security impact, but materially hurts operability (e.g. forensics).
- **Low** — cosmetic, consistency, or minor polish.
