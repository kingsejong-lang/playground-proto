# playground-proto

Shared gRPC/Connect contract for the KingSejong Playground. `playground-frontend` and
`playground-backend` both depend on this repository; neither keeps its own copy of the `.proto` files.

| Path | Contents |
|---|---|
| `proto/playground/api/v1` | public, browser-facing API (Connect protocol at the gateway) |
| `proto/playground/svc/v1` | service-to-service API (gRPC + mTLS); never exposed publicly |
| `gen/go` | generated Go: messages, connect-go v2 handlers/clients, grpc-go stubs (internal) |
| `go/limits` | edge limits protovalidate cannot express (total source size) |
| `ts` | npm package `@kingsejong-lang/playground-proto` (public API only) |
| `docs/SECURITY_PROTOCOL.md` | threat model, layered defences, sandbox contract, acceptance tests |
| `docs/RPC_DESIGN.md` | RPC map old→new, stream semantics, versioning |

## Use

Go:
```go
import (
    apiv1 "github.com/kingsejong-lang/playground-proto/gen/go/playground/api/v1"
    "github.com/kingsejong-lang/playground-proto/gen/go/playground/api/v1/apiv1connect"
)
```
Local development: add `use ../playground-proto` to the backend `go.work`.

TypeScript:
```ts
import { createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { ExecutionService } from "@kingsejong-lang/playground-proto";

const client = createClient(ExecutionService, createConnectTransport({
  baseUrl: process.env.NEXT_PUBLIC_API_URL!, useBinaryFormat: true,
  fetch: (i, init) => fetch(i, { ...init, credentials: "include" }),
}));
for await (const ev of client.run({ languageId: "kingsejong", files: { files: [{ path: "main.ksj", content: '출력("안녕")' }] } })) {
  console.log(ev.event.case, ev.event.value);
}
```
Local development: `make ts-pack`, then in the frontend
`npm install ../playground-proto/ts/kingsejong-lang-playground-proto-0.3.0.tgz`.

## Develop / test

```bash
make all         # buf lint, buf generate, go vet/test (validation rules), ts build + vitest
make breaking    # compatibility against main
```
Tools: buf ≥ 1.75, protoc-gen-go, protoc-gen-go-grpc, protoc-gen-connect-go (connect v2), protoc-gen-es 2.16.
