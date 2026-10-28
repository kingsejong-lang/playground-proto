// Mirrors go/validationtest: the same proto rules must hold in the browser.
import { describe, expect, it } from "vitest";
import { create } from "@bufbuild/protobuf";
import { createValidator } from "@bufbuild/protovalidate";
import { RunRequestSchema, FilesSchema, SourceFileSchema } from "./index.js";
import { checkTotalSource, MAX_FILE_BYTES } from "./limits.js";

const v = createValidator();
const files = (...paths: string[]) =>
  create(FilesSchema, { files: paths.map((path) => create(SourceFileSchema, { path, content: '출력("안녕")' })) });

describe("RunRequest rules", () => {
  it("accepts a normal multi-file program", () => {
    const r = v.validate(RunRequestSchema, create(RunRequestSchema, { languageId: "kingsejong", files: files("main.ksj", "lib/util.ksj") }));
    expect(r.kind).toBe("valid");
  });
  it.each([["../etc/passwd"], ["a/../b.ksj"], ["/etc/passwd"], ["a b.ksj"]])("rejects path %s", (p) => {
    const r = v.validate(RunRequestSchema, create(RunRequestSchema, { languageId: "kingsejong", files: files(p) }));
    expect(r.kind).toBe("invalid");
  });
  it("rejects duplicate paths", () => {
    const r = v.validate(RunRequestSchema, create(RunRequestSchema, { languageId: "kingsejong", files: files("m.ksj", "m.ksj") }));
    expect(r.kind).toBe("invalid");
  });
});

describe("checkTotalSource", () => {
  it("counts UTF-8 bytes", () => {
    const big = "a".repeat(MAX_FILE_BYTES);
    expect(checkTotalSource([{ content: big }, { content: big }, { content: big }, { content: big }])).toBe(true);
    expect(checkTotalSource([{ content: big }, { content: big }, { content: big }, { content: big }, { content: "가" }])).toBe(false);
  });
});
