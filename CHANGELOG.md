# Changelog

## 0.4.0 (2026-10-29)
- `ExecutionService.ResumeRun` (and `ExecutionInternalService.ResumeRun`): reattach to a run after a
  dropped stream; buffered events with `seq > after_seq` are replayed, then live events follow.
- `RunEvent.ReplayGap` marks events that were evicted from the 1 MiB buffer.
- `ErrorDetail.REASON_REPLAY_UNAVAILABLE` when nothing is buffered any more.
- Backwards compatible with 0.3.0 (`buf breaking` passes).

## 0.3.0 (2026-10-29)
- BREAKING (pre-release): `playground.internal.v1` renamed to `playground.svc.v1` (`proto/playground/svc/v1`,
  Go `gen/go/playground/svc/v1`, packages `svcv1`/`svcv1connect`). Go forbids importing paths
  containing `internal` from another module, so the old stubs were unusable by the backend.
- `ExecutionInternalService.FormatCode` added (backs public `LanguageService.FormatCode`).
- `VerifyAccessTokenResponse.user` added.
- SECURITY_PROTOCOL: refresh cookie path note, stream bound wall + 45 s.

## 0.2.0 (2026-10-29)
- `Language.default_template_id` (field 9) added; `file_extensions` documented as dot-prefixed lower case.
- Documented `delete_token` storage rule and the no-replay decision for `GetExecution`.
- npm package ships only `dist/` and no install scripts.

## 0.1.0 (2026-10-29)
- Initial public (`playground.api.v1`) and internal (`playground.internal.v1`) contract with protovalidate rules.
