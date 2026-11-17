# Validation

How to verify a change to LightIPAM before opening a PR. Run the tier that matches
your change; run everything before requesting a merge. **Never report a command as
passing that you did not actually run** — if a step is skipped, say so and why.

## Tier 0 — always (any code change)

```sh
npm ci                              # install pinned Tailwind toolchain (first run / lockfile change)
npm run build:css                   # regenerate the committed Tailwind CSS
go build ./...
go vet ./...
go test ./...
test -z "$(gofmt -l internal cmd)"  # formatting must be clean (prints nothing on success)
```

`gofmt -l internal cmd` lists any misformatted files; if it prints anything, run
`gofmt -w` on those files. A clean tree is required.

## Tier 1 — container builds (Dockerfile / compose / dependency / UI-embed change)

```sh
docker compose build                    # app image (regenerates CSS in the assets stage)
docker compose --profile scanner build  # scanner-agent image
```

The scanner image is only needed when the agent, its Dockerfile, or the scanner
packages changed — but building both is the safe default.

## Tier 2 — runtime smoke test (behavioral change worth exercising)

```sh
docker compose up -d
docker compose exec app wget -qO- http://127.0.0.1:8080/healthz   # liveness
docker compose exec app wget -qO- http://127.0.0.1:8080/readyz    # readiness (DB + migration version)
docker compose down
```

For the scanner path, generate dev mTLS material first, then bring up the profile:

```sh
go run ./cmd/scanner-certs -dir deploy/scanner-certs
docker compose --profile scanner up -d
```

The app auto-enrolls the bundled agent as `pending`; approve it under **Agents**,
create a subnet, and run a scan from **Scans**. Targets must fall within the agent's
`AGENT_ALLOWED_CIDRS`.

## One command

`scripts/verify.sh` runs the Tier 0 sequence (and, with `--docker`, Tier 1):

```sh
./scripts/verify.sh            # Tier 0
./scripts/verify.sh --docker   # Tier 0 + container builds
```

## PostgreSQL regression tests

Store integration tests skip when `LIGHTIPAM_TEST_DATABASE_URL` is unset. A default
`go test ./...` pass is not evidence that these database tests ran. Use an isolated
PostgreSQL 16 database; tests create/drop private schemas, never target production.
The test role must create/drop schemas. For example, with an unused loopback port:

```sh
docker run --rm -d --name lightipam-test-db \
  -p 127.0.0.1:55439:5432 \
  -e POSTGRES_DB=lightipam_test -e POSTGRES_USER=lightipam_test \
  -e POSTGRES_PASSWORD=lightipam_test \
  --tmpfs /var/lib/postgresql/data postgres:16-alpine
docker exec lightipam-test-db pg_isready -U lightipam_test -d lightipam_test
export LIGHTIPAM_TEST_DATABASE_URL='postgres://lightipam_test:lightipam_test@127.0.0.1:55439/lightipam_test?sslmode=disable'
go test ./internal/store -count=1 -v
./scripts/verify.sh --docker
docker stop lightipam-test-db
unset LIGHTIPAM_TEST_DATABASE_URL
```

Wait for `pg_isready` to succeed before testing. Credentials above are only for the
disposable local test database. For containment/identity changes, run the relevant
race/concurrency regressions as well and report exactly which database tests ran.

## CI

`.github/workflows/ci.yml` runs Tier 0 plus both container builds on every pull
request and on pushes to `main`. It does **not** publish images — publishing happens
only in `.github/workflows/release.yml` when a `v*` tag is pushed. Keep local
validation and CI in step: if you change the validation steps here, update the
workflow too. PR/main CI provisions an ephemeral PostgreSQL 16 service and sets the
test URL so store regressions execute. The release job does not yet provision it;
extending release/full-stack coverage is [R01](../ROADMAP.md#r01).
The release publisher is separate from CI; see [Releasing](../RELEASING.md).

## Dependency and generated-asset checks

Use `npm ci` after lockfile changes. Rebuild CSS and inspect `git diff` for accidental
asset drift; never edit generated CSS directly. `npm audit` is a separate result:
the compatible November lockfile refresh reduced findings but left the picomatch
path unresolved. A passing build does not mean a clean audit. Record current audit
output and use [R05](../ROADMAP.md#r05) for a reviewed major toolchain migration; do
not force an upgrade or suppress warnings to make the report green.

## Troubleshooting

- **Go cache / sandbox permission errors** (e.g. `go: failed to ... permission
  denied` on `$GOCACHE`): rerun with normal Go build-cache permissions rather than
  editing source or vendoring. Do not "fix" a sandbox issue by changing the code.
- **`go build ./cmd/server` or `go build ./cmd/scanner-agent` (one main package, no `-o`) can drop stray `/server` or `/scanner-agent` binaries** at the
  repo root. Both are gitignored — do not commit them.
- **Confirm no silent dependency creep in the agent:**
  `go build -mod=readonly ./cmd/scanner-agent`.
- **Dev mTLS keys** under `deploy/scanner-certs/` are gitignored — never commit them.
