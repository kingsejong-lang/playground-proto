/**
 * Edge limits mirrored from go/limits/limits.go. The browser uses them for
 * instant feedback; the gateway remains the authority.
 *
 * Usage:
 *   import { checkTotalSource } from "@kingsejong-lang/playground-proto/limits";
 *   if (!checkTotalSource(files)) showError("...");
 */
export const MAX_FILE_BYTES = 64 * 1024;
export const MAX_FILES = 16;
export const MAX_TOTAL_SOURCE_BYTES = 256 * 1024;
export const MAX_STDIN_BYTES = 64 * 1024;

const encoder = new TextEncoder();

/** Returns true when the UTF-8 size of all contents fits MAX_TOTAL_SOURCE_BYTES. */
export function checkTotalSource(files: ReadonlyArray<{ content: string }>): boolean {
  let total = 0;
  for (const f of files) {
    total += encoder.encode(f.content).byteLength;
    if (total > MAX_TOTAL_SOURCE_BYTES) return false;
  }
  return true;
}
