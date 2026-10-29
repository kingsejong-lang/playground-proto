# Playground Roadmap: Open Work

Status as of 2026-10-29: all items are done except #3, which is on the owner's machine. This list covers work across `playground-proto`, `playground-backend` and
`playground-frontend` that remains after the move to the Connect edge and the shared contract.
Each item is delivered through a pull request that passes CI and is squash-merged.

Legend: **Status** `open` · `in progress` · `done`. **Needs** says what the item depends on.

## A. Release and repository setup

| # | Item | Repo | Decision | Status |
|---|---|---|---|---|
| 1 | Commit the contract adoption, edge security and UI changes through PRs; squash-merge after CI; sync local `main` | backend, frontend | Conventional Commits, no co-author trailers, no tool attribution | done (frontend #1, backend #1) |
| 2 | Publish this repository: `main` initialised with the license, contract added by PR, tag `v0.3.0`, npm tarball attached to the GitHub release | proto | Apache-2.0 (compatible with the Apache-2.0 frontend and the GPL-3.0 backend) | done (#1, release v0.3.0) |
| 3 | Go module cache path on the development machine points at a removed temporary directory | local machine | Owner cleans caches when storage runs low; tooling must not depend on caches surviving. Sessions use their own cache directory. | open (owner) |
| 4 | Keyboard shortcuts collide with browser keys (Cmd+S creates a public share; Cmd+T/L/H/F) | frontend | Change ours. Keep only shortcuts with a clear payoff: Run (Mod+Enter), Stop, shortcut help; everything else through the menu or the editor command palette (F1). Mod+S saves the draft locally instead of sharing. | done (frontend #2) |
| 5 | Multi-file programs: main page ran only the active file, embed ran all files | frontend | Send all files and run the designated entry file (language default, changeable per project); "run this file" stays available as an editor action. | done (frontend #2) |
| 6 | `ui-design-system` skill script has a syntax error (line 64) | local skill | Fix it; for design work prefer the Impeccable plugin | done |
| 7 | `kingsejong-lang/.claude/settings.local.json` pre-approves `cmake`, `make` and test runs | kingsejong-lang | Keep as is (needed when building the runtime) | done |

## B. Verification on real infrastructure (Docker)

| # | Item | Repo | How | Status |
|---|---|---|---|---|
| 8 | Sandbox acceptance tests: fork bomb, infinite loop, 100 MiB output, state across runs, network denied, memory limit, SIGSEGV mapping | backend | `go test -tags sandbox` against a Docker daemon; gVisor (`runsc`) inside a Linux Docker-in-Docker container | done (backend #2; runc and gVisor) |
| 9 | Postgres: storage and session repositories, migrations `000007`/`000008` up and down | backend | `docker compose` Postgres + `make db-migrate` + repository tests | done (backend #2) |
| 10 | End-to-end: real frontend against real gateway and services | frontend, backend | `docker compose up`, Playwright against the stack | done (frontend #3, backend #4) |
| 11 | Runtime images built and pinned by digest; images contain `sleep`, `tar`, `cat` and no secret env | backend | Build images, record digests in the language catalogue | done (backend #2; release images via the Runtime images workflow) |
| 12 | `kingsejong fmt` behaviour inside the sandbox | backend | Part of 8 and 10 | done (backend #2) |

## C. Follow-up development (after 1–12)

| # | Item | Repo | Plan | Status |
|---|---|---|---|---|
| 13 | Collect CPU time and peak memory per run | backend | Read cgroup stats from the container runtime before removal; fill `ExecutionMetrics` | done (backend #5) |
| 14 | Pre-warmed single-use containers to cut start latency | backend | Pool of created-but-unused containers per language; each used once and destroyed | done (backend #6) |
| 15 | Per-user process limit shared across sandboxes under runc (all run as uid 65534) | backend | Distinct uid per run from a range, or rely on gVisor; per-container PID limit remains the bound | done (backend #7, uid per run) |
| 16 | Run Playwright E2E in CI | frontend | Covered by 10 and the CI job | done (frontend #3, backend #4) |
| 17 | Test coverage 34 % (old threshold 70 %) | frontend | Raise coverage of `features/*` and enforce a threshold in CI | done (frontend #4: 84 % lines, enforced in CI) |
| 18 | UI gaps: auto-save switch does nothing; no KingSejong syntax help; language list fetched on the client (LCP) | frontend | Local draft auto-save; keyword/syntax reference panel from the language docs; server-render the language list | done (frontend #2 auto-save, #5 reference panel, #6 server-rendered list) |
| 19 | Monaco loaded from jsDelivr at runtime | frontend | Self-host Monaco assets and tighten CSP | done (frontend #7) |
| 20 | Output after a dropped stream cannot be recovered (`GetExecution` does not replay output) | proto, backend, frontend | Keep the last 1 MiB of output per run in Redis for 5 minutes; `GetExecution` replays from a sequence number (additive contract change, v0.4.0) | done (proto #3 v0.4.0, #4; backend #8, #9; frontend #8, #9) |

## Decisions

- **#20 output replay** (2026-10-29): Redis buffer chosen over keeping the notice-only behaviour or an
  in-memory buffer in the execution service. It works with several gateway and execution instances;
  the cost is at most 1 MiB of Redis memory per run for 5 minutes.

## Found and fixed along the way

- Python and Go could not import a second submitted file (backend #3).
- Stop only closed the stream instead of sending Cancel (frontend #9).
- The editor highlighted English keywords that KingSejong does not have and toggled comments with `//` (frontend #10).
- Cancel and GetExecution worked only on the replica running the program (backend #9).
- Outside development the language service could start with local-registry runtime images (backend #2).

## Follow-up work after the list

Done:

- Release runtime images are published to ghcr.io and pinned in the production overlay (backend #12, #16).
- Service images are published to ghcr.io with the workflow token (backend #13).
- Redis Cluster is supported and tested against a three-primary cluster (backend #14).
- A multi-node kind cluster with a gVisor sandbox node, two gateway and two execution replicas, and Redis
  Cluster passes the stack, cross-replica Cancel and ResumeRun tests (backend #17).
- Alerts for Redis loss, dead execution replicas and failing buffers (backend #11).
- The edge moved from ingress-nginx, whose upstream maintenance ended in March 2026, to Gateway API on
  Envoy Gateway, with brotli, zstd and gzip compression (backend #21). The frontend can turn off its own
  compression behind it with `NEXT_COMPRESS=false` (frontend #16).
- Internal gRPC clients reconnect within seconds after an outage (backend #19).

Still open:

- GitHub Actions does not start jobs on the private backend repository because of an account billing
  problem. Until it is fixed, backend pull requests are verified with the local equivalent of every CI job,
  recorded in a "Local CI" comment on each pull request.
- Deployment jobs stay skipped until a cluster exists and `KUBE_CONFIG_DEV` / `KUBE_CONFIG_PROD` are set.
- Not yet tested: a certificate issued by cert-manager for the Gateway, a real load balancer address,
  and gVisor with the KVM platform (only systrap was tested).
