# Edge Security Protocol (v1)

Status: Accepted (2026-10-29) · Scope: browser ↔ gateway ↔ internal services · Owner: playground-proto

This document defines what the backend guarantees when the frontend (or anything pretending to be it)
sends hostile input. The threat model assumes **the browser is fully attacker-controlled**: any field,
header, size, rate or sequence of calls can be forged. Nothing below relies on the official frontend
behaving well.

## 1. Why this exists — defects found in the current system

| # | Severity | Defect (current backend) | Evidence |
|---|---|---|---|
| D1 | Critical | `/grpc/*` mounts a gRPC-Web proxy of **every** internal RPC with no auth middleware: `AuthService.CreateAPIKey` takes `user_id` from the request, so anyone can mint API keys for any user. `DeleteShare`, `SaveUserSettings` take ids/user ids from the body without an owner check. `VerifyToken`/`ValidateAPIKey` act as oracles. | `services/gateway/internal/server/routes.go` (`/grpc/*any`), `services/gateway/internal/grpcweb/proxy.go`, `services/auth/internal/server/server.go` CreateAPIKey, `services/storage/internal/server/server.go` DeleteShare |
| D2 | Critical | gRPC-Web wrapper allows every origin (`AllowOrigins: []` ⇒ allow all) | `services/gateway/internal/grpcweb/wrapper.go` DefaultConfig |
| D3 | Critical | Execution containers are pooled and **reused across users** (up to 100 uses); `/workspace` tmpfs and any process left behind are shared between tenants | `services/execution/internal/pool/pool.go` (`MaxContainerUsages`), `docker_executor.go` Acquire/Release |
| D4 | High | No request bounds: file count/size, stdin, args, timeout are unchecked; client `timeout_seconds` is honoured | `services/execution/internal/server/server.go` Execute, `services/gateway/internal/handlers/execution_handler.go` |
| D5 | High | gVisor runtime commented out; containers share the host kernel | `services/execution/internal/pool/pool.go` HostConfig |
| D6 | High | Internal error text (`err.Error()`) returned to clients | gateway handlers `Details: err.Error()` |
| D7 | High | Language runtime commands are shell strings (`run_cmd`), a shell-injection vector once filenames are interpolated | `services/language/internal/registry/languages.go` |
| D8 | Medium | Four parallel client transports (REST, SSE, WebSocket, gRPC-Web) each with its own validation and auth path; browser gRPC client reads a JWT from `localStorage` | frontend `src/lib/grpc/clients/config.ts`, `src/lib/api.ts`, `src/lib/websocket/*` |
| D9 | Medium | Rate limiting off by default and keyed only by IP/user; no per-identity concurrency cap | `services/gateway/cmd/main.go` `RATE_LIMIT_ENABLED=false` |
| D10 | Medium | `improbable-eng/grpc-web` is in maintenance mode with no release since v0.15.0 | gateway `go.mod` |

## 2. Transport decision

Browsers cannot speak native gRPC (no HTTP/2 trailer access from `fetch`). The edge therefore uses
the **Connect protocol** (connect-go v2 server, connect-es v2 client), which is wire-compatible with
gRPC semantics (same `.proto`, status codes, server streaming) over plain HTTP/1.1 or HTTP/2.

* Edge (browser → gateway): Connect protocol only, binary protobuf (`application/proto`).
  gRPC and gRPC-Web handlers are **not** enabled on the public listener.
* Internal (gateway → services): gRPC over HTTP/2 with mutual TLS. Services may be grpc-go (existing)
  or connect-go with `connecthttp.WithGRPC()`.
* REST, SSE and WebSocket endpoints are removed. `GET /p/:id` (share landing page) is served by the
  frontend, which calls `ShareService.GetShare`.

Alternative considered — Next.js BFF calling gRPC server-side: removes CORS entirely and keeps the
gateway private, but every execution stream is proxied twice and the Node server becomes a stateful
streaming tier. Rejected for v1; revisit if the gateway must not be internet-facing.

## 3. Layered defences (request order)

Every public request passes these layers in order. A layer rejects with the listed Connect code and an
`ErrorDetail.reason`; later layers never see rejected input.

| Layer | Mechanism | Limit / rule | Code |
|---|---|---|---|
| L0 Connection | `http.Server` timeouts | `ReadHeaderTimeout 5s`, `IdleTimeout 60s`, `MaxHeaderBytes 16 KiB`; unary `WriteTimeout` via deadline, streams bounded by L7 | — |
| L1 Body size | `http.MaxBytesHandler` + `connecthttp.WithReadMaxBytes` | raw body ≤ 1 MiB; decoded message ≤ 512 KiB (after decompression, so gzip bombs fail) | `ResourceExhausted` / `PAYLOAD_TOO_LARGE` |
| L2 Origin & CSRF | Origin allow-list middleware + `WithRequireConnectProtocolHeader()` + content-type `application/proto` only; HTTP GET disabled | Missing/unknown `Origin` on a cookie-bearing request ⇒ reject. The required `Connect-Protocol-Version` header and non-simple content type force a CORS preflight, which the allow-list blocks cross-site. Cookies are `SameSite=Strict`. No CSRF token endpoint is needed. | `PermissionDenied` / `ORIGIN_REJECTED` |
| L3 Recovery | outermost interceptor | panics → `Internal`, logged with request id; process keeps serving | `Internal` |
| L4 Identity | session interceptor | `__Host-pg_at` cookie verified via `AuthInternalService.VerifyAccessToken` (cached 30 s). No cookie ⇒ anonymous principal from signed `__Host-pg_anon` cookie (issued on first call). Identity is **never** read from message fields. | `Unauthenticated` |
| L5 Rate & concurrency | token buckets in Redis keyed by principal **and** by client IP /64 (IPv6) or /32 | anonymous: 10 runs/min, 1 concurrent run (a run counts until `Finished`, also after its stream dropped), 60 other calls/min; user: 30 runs/min, 2 concurrent; auth RPCs: 5/min/IP. Redis unavailable ⇒ **fail closed** for `Run`, fail open (local bucket) for read-only RPCs | `ResourceExhausted` / `RATE_LIMITED`, `CONCURRENCY_LIMIT` (+ `retry_after_seconds`) |
| L6 Validation | `connectrpc.com/validate` server interceptor (protovalidate rules in these protos) + `go/limits.CheckFiles` | field rules in `proto/playground/api/v1/*.proto`; total source ≤ 256 KiB | `InvalidArgument` / `VALIDATION` (+ `field`) |
| L7 Admission | bounded global run queue in execution service | queue ≤ 2 × worker count; full ⇒ reject immediately (no unbounded waiting). Per-stream: max duration = policy wall time + 45 s (up to 30 s queue wait after `Accepted` + 15 s slack), output capped at `max_output_bytes`, heartbeat every 10 s, server closes idle streams after 30 s without client read progress. Closing a stream after `Accepted` ends only the stream; the run stays bounded by its wall-time limit and stops early only on `Cancel` | `Unavailable` / `OVERLOADED` |
| L8 Authorization | in the owning service, using `CallContext.principal` | share delete/settings/cancel/get-execution/resume-run require owner match; anonymous share delete requires a constant-time-compared delete token | `PermissionDenied` / `NOT_FOUND` (do not reveal existence) |
| L9 Sandbox | see §5 | — | reported as `ExecutionStatus` in `Finished`, not as RPC errors |

Errors returned to the browser contain only a safe message and `ErrorDetail`; internal error text,
stack traces, hostnames and container ids are logged with the request id and never sent.

## 4. Identity and trust boundary

```
Browser ──(Connect, cookies)──▶ Gateway ──(gRPC + mTLS, CallContext)──▶ auth / language / storage / execution
   untrusted                    builds Principal                          trust CallContext only from the
                                from cookie only                          peer whose cert SAN = gateway
```

* Public messages contain **no** `user_id`, `request_id`, tier, limit-raising or host fields.
  `RunRequest.wall_time_limit_ms` can only *lower* the policy limit.
* The gateway generates `CallContext.request_id` (ULID) and computes `ExecutionPolicy` from the tier.
* Internal services listen only on the private network, require client certificates, and reject a
  `CallContext` from any peer that is not the gateway. Internal packages (`playground.svc.v1`) are
  never registered on the public mux (Go path `gen/go/playground/svc/v1`; the name avoids Go's `internal` import restriction) — enforced by a gateway test that lists mounted procedures.
* `execution_id` and share ids are random (≥128 bits for executions, 72 bits for shares) and are
  authorisation-checked anyway; ids are not secrets.
* Access token: 15 min JWT (EdDSA), cookie `__Host-pg_at` (HttpOnly, Secure, SameSite=Strict, Path=/).
  Refresh token: 30 days, opaque, rotated on each use, reuse revokes the token family; cookie
  `__Host-pg_rt` (the `__Host-` prefix forces `Path=/`, so the path cannot be narrowed; the gateway
  reads it only in `RefreshSession` and `Logout` and ignores it everywhere else).

## 5. Sandbox contract (execution service)

The language runtime's own sandbox (`kingsejong --sandbox`) is defence in depth only; the container
boundary is authoritative.

1. **One container per run, never reused.** Pre-warmed containers may be created ahead of time but
   are destroyed after a single run. No state crosses runs or tenants.
2. Runtime class `runsc` (gVisor) required in production; startup fails if unavailable unless
   `EXECUTION_ALLOW_RUNC=dev` is set.
3. `--network=none`, read-only root fs, `/workspace` tmpfs (`size` = `max_file_bytes`, `nosuid,nodev`),
   `/tmp` tmpfs 16 MiB, `CapDrop ALL`, `no-new-privileges`, default seccomp profile, user
   `65534:65534`, `PidsLimit = max_processes`, memory = swap = `memory_bytes`, `NanoCPUs` 1 CPU,
   ulimits `cpu`, `fsize`, `nofile 256`, `nproc`.
4. Commands are argv arrays from `RuntimeSpec` (no shell). Entry path is validated before substitution.
   KingSejong runs as `kingsejong --sandbox /workspace/<entry>` with cwd `/workspace`. Facts from the
   kingsejong-lang analysis (`docs/research/kingsejong-lang/analyses/05-design-insights.md`) that the
   container must compensate for:
   * `가져오기` (import) bypasses `SecurityManager` and accepts absolute paths
     (`src/module/ModuleLoader.cpp:157-161`, `:282-292`) ⇒ the image must contain nothing secret and
     the process must not see host paths; `--sandbox` alone does not confine reads.
   * stdlib lookup order is cwd → `KINGSEJONG_PATH` → install dir (`ModuleLoader.cpp:183-217`); the
     variable is `KINGSEJONG_PATH`, not `KINGSEJONG_STDLIB` ⇒ a user file `stdlib/x.ksj` in
     `/workspace` shadows the real stdlib. Reject submitted paths under `stdlib/` at the gateway or
     accept the shadowing as user-scoped (it only affects that run).
   * No memory limit, no Evaluator recursion-depth limit (native stack overflow ⇒ SIGSEGV), no output
     limit, `슬립` unbounded, limit errors catchable by user `시도` blocks ⇒ cgroup memory, wall-clock
     kill, output cap and SIGSEGV→`FAILED` mapping are mandatory host-side.
   * `프로세스_종료(n)` calls `std::exit(n)` and every interpreter failure exits 1 ⇒ status is derived
     from exit code plus stderr markers (`📍 오류 위치:`, `file:line:col:`, `Semantic Error:`), and
     parser errors are mapped to `Diagnostic` events.
   * Info builtins (`현재_디렉토리`, `환경변수_읽기`, `사용자_이름`, `호스트_이름`) are unchecked ⇒ the
     container env must be empty except required variables and the hostname neutral (`sandbox`).
5. Wall-clock timer in the executor kills the **container** (not just the exec) at `wall_time_ms`;
   status `TIME_LIMIT_EXCEEDED`. OOM kill ⇒ `MEMORY_LIMIT_EXCEEDED`.
6. Output is read incrementally in ≤ 32 KiB chunks; after `max_output_bytes` the executor emits one
   `Truncated` event, stops forwarding, kills the container and finishes with `OUTPUT_LIMIT_EXCEEDED`.
7. Images are pinned by digest; the execution service does not pull at request time.
8. The execution service is the only holder of the container-runtime socket and runs on nodes that
   host nothing else. The gateway cannot reach the runtime socket.
9. The replay buffer for `ResumeRun` is keyed by `execution_id`, capped at 1 MiB per run, expires 5
   minutes after the run finishes, and is readable only by the run's owner. It counts toward the
   anonymous/user call limits (L5) like other read RPCs.
10. Result caching keyed by source is **removed** for runs (it returned another user's output for the
   same input and hid nondeterminism). Formatting results may be cached.

## 6. Abuse and availability

* Gateway and services are stateless except the Redis buckets and replay buffers; losing a gateway pod
  drops streams but not the runs behind them, and clients reattach with `ResumeRun`.
* Execution workers are sized by `EXECUTION_WORKERS`; L7 rejects instead of queueing without bound,
  so a flood degrades to fast `OVERLOADED` errors and never to OOM.
* Each component exports `rejections_total{layer,reason}`; alert when L5/L7 rejections exceed 5 % of
  requests for 5 min.

## 7. Verification (acceptance tests)

| Test | Pass criterion |
|---|---|
| `go/validationtest` (this repo) and `ts/src/rules.test.ts` | traversal, duplicates, oversize, bad ids rejected in Go and TS |
| gateway: mounted-procedure test | public mux exposes exactly the `playground.api.v1` procedures |
| gateway: hostile client suite | 2 MiB body ⇒ `ResourceExhausted`; gzip bomb ⇒ `ResourceExhausted`; foreign Origin ⇒ `PermissionDenied`; GET ⇒ 405; missing protocol header ⇒ rejected; 11th anonymous run in a minute ⇒ `RATE_LIMITED`; second concurrent anonymous run ⇒ `CONCURRENCY_LIMIT` |
| storage: authorization | deleting another principal's share ⇒ `NotFound`; wrong delete token ⇒ `NotFound` |
| execution (requires container runtime) | fork bomb ⇒ run ends, host pid count unchanged; `while 1` ⇒ `TIME_LIMIT_EXCEEDED` within wall + 1 s; 100 MiB output ⇒ `OUTPUT_LIMIT_EXCEEDED`, gateway RSS stable; file written in run A not visible in run B; network call fails |
