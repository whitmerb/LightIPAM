# Releases and image publication

The maintainer authorizes a release. Use the [GitHub workflow skill](../.agents/skills/lightipam-github-workflow/SKILL.md)
and [release-prep skill](../.agents/skills/lightipam-release-prep/SKILL.md); keep
version/changelog changes separate from product fixes. This guide describes the
existing workflow, not a new publication mechanism.

## Prepare and merge

1. Verify the GitHub identity, canonical `whitmerb/LightIPAM` repository and remote
   main. Inspect the diff since the latest release tag and choose SemVer: patch for
   compatible fixes, minor for capabilities, major for a breaking `/api/v1`, scanner
   protocol or destructive migration change. Confirm the intended tag is unused.
2. Keep one task per functional branch/PR. Review complete diffs, local validation,
   current-head CI/CodeQL, review requests and unresolved threads. Partial dependency
   remediation must name residual advisories; it must not claim a clean audit.
3. After maintainer authorization, mark complete drafts ready and squash-merge with
   `type(scope): summary (#PR)` as the subject. If main introduces conflicts, merge
   current main into the feature branch, preserve both in-scope changes, validate and
   wait for fresh checks. This branch synchronization is not a merge commit to main:
   the PR itself still uses squash. Do not force-push or bypass protections.
4. Verify each merge SHA, delete the merged remote branch, and fast-forward local
   main. Preserve unmerged local branches/worktrees; squash topology alone is not
   permission to force-delete them.
5. Prepare a release branch/PR with the versioned changelog/date, compare links,
   `package.json` and lockfile root versions, README/installation version examples,
   `.env.example`, published-image Compose defaults, and Project state. Roadmap must
   distinguish completed changes from the remaining planning backlog. Historical
   release sections/ADRs keep their original version references.
6. Run [Validation](agent/VALIDATION.md), including both images, database regressions
   when relevant and a smoke test of the combined changes. Merge the release PR by
   the same authorized, checked squash method. The tag must point to this verified
   main commit, not an earlier feature branch.

## Trigger the existing workflow

Merging main runs CI/CodeQL. It does **not** publish images. Pushing the approved
annotated `v*` tag triggers `.github/workflows/release.yml`:

- `test` runs Go vet/tests; database integration tests currently require separate
  local evidence because the job does not provision their test URL.
- `images` depends on test and builds app plus scanner for `linux/amd64` and
  `linux/arm64` with the tag stamped as the binary version.
- `GITHUB_TOKEN` has `contents: read` and `packages: write`. GHCR image names derive
  from the repository owner, currently `ghcr.io/whitmerb/lightipam` and
  `ghcr.io/whitmerb/lightipam-scanner`.
- Docker metadata creates the exact release and moving minor tags; its automatic
  stable flavor also supplies `latest`. Prereleases must not move stable `latest`.

Example after the approved release PR is merged (replace the example version with
the approved target):

```sh
release_version=v1.2.2
git switch main
git pull --ff-only origin main
git tag -a "$release_version" -m "LightIPAM $release_version"
git push origin "$release_version"
gh run list --repo whitmerb/LightIPAM --workflow release.yml --limit 5
# Watch the run matching the tag and exact main SHA:
gh run watch <run-id> --repo whitmerb/LightIPAM --exit-status
```

Do not manually `docker push`, repoint a published tag, or repeatedly rerun a failed
publisher without understanding the failure. If registry authorization fails after
an ownership transfer, check package linkage/access and report the exact failure;
do not broaden workflow permissions or expose credentials to work around it.

## Verify before announcing

Confirm the successful run's tag/head SHA and both image jobs, then inspect both
registry manifests and their exact/minor/latest tags. Each release index should
contain amd64 and arm64; provenance/attestation entries may show unknown platforms.
Pull the exact images and verify their reported binary version and readiness in an
isolated deployment. The scanner check needs valid mTLS and an authorized scope;
no production scan is required just to verify publication.

```sh
docker buildx imagetools inspect ghcr.io/whitmerb/lightipam:1.2.2
docker buildx imagetools inspect ghcr.io/whitmerb/lightipam-scanner:1.2.2
```

A GitHub Release is separate from pushing a tag and publishing images. After
verification, create it with reviewed release notes covering changes, compatibility,
known limits, image references and upgrade instructions. Preserve actual newlines
using a notes file or structured tool argument. Record workflow URL, tag/merge SHAs
and image digests in the handoff/report; do not call publication successful based
only on a pushed tag.
