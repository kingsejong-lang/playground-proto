# RPC Design (v1)

Status: Accepted (2026-10-29). Contract files: `proto/playground/api/v1` (public) and
`proto/playground/svc/v1` (service-to-service). Security rules: [SECURITY_PROTOCOL.md](SECURITY_PROTOCOL.md).

## 1. Old surface → new surface

| Old (backend `shared/proto`, REST, WS, SSE) | New public RPC | Notes |
|---|---|---|
| `GET /api/v1/languages`, `LanguageService.ListLanguages` | `LanguageService.ListLanguages` | no docker image / commands in public view |
| `GET /api/v1/languages/:id` | `LanguageService.GetLanguage` | |
| `GET /api/v1/languages/:id/templates/:t` | `LanguageService.ListTemplates`, `GetTemplate` | list without contents, fetch one |
| `POST /api/v1/languages/:id/format` | `LanguageService.FormatCode` | returns `Diagnostic`s on parse errors |
| `POST /api/v1/execute`, `ExecutionService.Execute` | — removed | single path: streaming `Run` |
| `POST /api/v1/execute/stream` (SSE), WS `execute`, `ExecuteStream` | `ExecutionService.Run` (server stream) | typed events with `seq` |
| `POST /api/v1/execute/:id/cancel`, WS `cancel` | `ExecutionService.Cancel` (aborting the stream alone does not stop a run) | owner-checked |
| `GET /api/v1/execute/:id/status` (mock) | `ExecutionService.GetExecution`, `ResumeRun` | real state and output replay, owner-checked |
| `POST/GET/DELETE /api/v1/share` | `ShareService.CreateShare/GetShare/DeleteShare` | delete token for anonymous |
| `GET /p/:id`, `GET /download/:id` | frontend route + `GetShare` | download built client-side |
| `POST/GET /api/v1/settings` | `SettingsService.GetSettings/UpdateSettings` | field mask |
| `POST /api/v1/auth/*`, `AuthService.*` | `AuthService.Register/Login/Logout/RefreshSession/GetSession` | cookies only, no tokens in bodies |
| `GET /api/v1/csrf/token` | — removed | replaced by Origin + protocol header + SameSite (L2) |
| `AuthService.VerifyToken/CreateAPIKey/ValidateAPIKey/RevokeAPIKey` | internal only | never on the public edge |
| `EventService` (presence/pubsub) | — not carried over | no caller in gateway or frontend; reintroduce with a design when collaboration is built |
| WS `ping/pong` | `RunEvent.Heartbeat` | |

## 2. Run stream semantics

```
client                          gateway                         execution
  │ Run(RunRequest) ───────────▶│ L1–L6 checks                  │
  │                             │ policy := tier(principal)     │
  │                             │ InternalRun(ctx,req,policy) ─▶│ admission (L7)
  │◀── Accepted{id, limits} ────│◀───────────────────────────── │
  │◀── Queued{position}* ───────│◀───────────────────────────── │
  │◀── Started{COMPILE|RUN} ────│◀───────────────────────────── │ fresh container
  │◀── Output / Diagnostic / Truncated / Heartbeat ───────────── │
  │◀── Finished{status, exit_code, metrics} ─────────────────── │ container destroyed
  │   (stream ends with OK)     │                               │
```

* Rejections before `Accepted` are RPC errors (`InvalidArgument`, `ResourceExhausted`, `Unavailable`).
  After `Accepted`, every outcome — including timeouts and sandbox faults — is a `Finished` event and the
  RPC ends with OK. Clients therefore have exactly one place to read the result.
* A run outlives its stream. Closing the stream before `Accepted` cancels the call and nothing runs.
  After `Accepted`, a closed or lost stream (fetch `AbortSignal`, network drop, gateway restart) does
  not stop the run: it continues until it finishes, hits a limit or is cancelled, and its events keep
  going to the replay buffer. **Stop means `Cancel`**: clients call `Cancel(execution_id)` when the
  user stops a run (aborting the fetch as well is fine). The run keeps its concurrency slot (L5) until
  it ends, so dropping streams does not raise the number of parallel runs.
* `client_request_id` makes retries idempotent for 60 s.
* After a dropped stream the client calls `ResumeRun(execution_id, after_seq = last received seq)`.
  The execution service appends every event of a run to a Redis stream capped at 1 MiB, kept until
  5 minutes after `Finished`. Replay sends buffered events with `seq > after_seq` (preceded by
  `ReplayGap` when some were evicted) and then live events until `Finished`; the client
  de-duplicates by `seq`. Nothing buffered ⇒ `FAILED_PRECONDITION` + `REASON_REPLAY_UNAVAILABLE`.
* `Output.data` is bytes; the frontend uses a streaming `TextDecoder` per stream.

## 3. Versioning and compatibility

* `buf breaking --against '.git#branch=main'` runs in CI; breaking changes require a new package
  (`playground.api.v2`).
* Repo tags `vMAJOR.MINOR.PATCH` publish the Go module (`github.com/kingsejong-lang/playground-proto`)
  and the npm package (`@kingsejong-lang/playground-proto`) from the same commit.
* Consumers pin an exact version. Local development: Go uses a `go.work` `use ../playground-proto`
  line; the frontend installs the tarball produced by `make ts-pack`.

## 4. Open decisions (defaults applied, revisit)

| Decision | Default applied | Alternative |
|---|---|---|
| Edge codec | binary protobuf only | also allow JSON for debugging (`connecthttp.WithProtoJSON`) — larger surface |
| connect-go major | v2 (interceptors run before decode, so rejected calls cost no parsing) | v1 — more examples, fewer API changes |
| Internal transport | keep grpc-go stubs, migrate per service | connect-go everywhere — one stack, larger change |
| Anonymous execution | allowed with low quota | sign-in required — simpler abuse story, worse first-run UX |
