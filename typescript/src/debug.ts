// Optional diagnostics only. No clock, environment, network or storage access.
export interface DebugEvent { component: string; event: string; details: unknown }
export type DebugSink = (event: DebugEvent) => void;
let sink: DebugSink | undefined;
export function setDebugSink(value?: DebugSink): void { sink = value; }
export function debugEnabled(): boolean { return sink !== undefined; }
function utf8Length(text: string): number {
  let size = 0;
  for (const character of text) { const code = character.codePointAt(0)!; size += code < 128 ? 1 : code < 2048 ? 2 : code < 65536 ? 3 : 4; }
  return size;
}
function summary(value: unknown, key = "", depth = 0): unknown {
  if (/password|passwd|private.?key|secret|mnemonic|token|credential|wif/i.test(key)) return "[redacted]";
  if (depth > 20) return "[depth]";
  if (value instanceof Uint8Array) return { byteLength: value.byteLength };
  if (value instanceof ArrayBuffer) return { byteLength: value.byteLength };
  if (typeof value === "bigint") return value.toString();
  if (typeof value === "string") {
    if (/signature|script|preimage|raw|signedTx|unsignedTx|draft|body|payloadBytes/i.test(key)
      || value.length > 128 || /^[0-9a-f]{66,}$/i.test(value)) return { byteLength: /^[0-9a-f]*$/i.test(value) && value.length % 2 === 0 ? value.length / 2 : utf8Length(value) };
    return value.replace(/((?:password|secret|token|private.?key)\s*[=:]\s*)[^\s,;]+/gi, "$1[redacted]");
  }
  if (Array.isArray(value)) return value.map(item => summary(item, key, depth + 1));
  if (value && typeof value === "object") return Object.fromEntries(Object.entries(value).map(([name, item]) => [name, summary(item, name, depth + 1)]));
  return value;
}
export function debugLog(event: string, details: () => unknown): void {
  if (!sink) return;
  try { sink({ component: "utxo-manager", event, details: summary(details()) }); } catch { /* diagnostics cannot change money decisions */ }
}

function operationInput(operation: string, values: unknown[]): unknown {
  const fields: Record<string, string[]> = {
    reportExternalSpend: ["outpoint", "txid"], setProtected: ["outpoints", "protect", "source"],
    replace: ["allocationId", "input"], release: ["allocationId"], markSigningIntentDurable: ["allocationId"],
    settleExternalSignature: ["id", "evidence"], supersedeExternalSignature: ["oldId", "newId"],
    markDispatching: ["commandId"], broadcastResult: ["commandId", "result"],
    resolveInputObservations: ["commandId", "observations"], abortUnbroadcast: ["allocationId"],
    cancelUnstarted: ["allocationId"], importUnknown: ["txid", "rawTxHex"], reconcile: ["commandId", "found", "rawTxHex"],
  };
  return fields[operation] ? Object.fromEntries(fields[operation]!.map((key, index) => [key, values[index]])) : values[0];
}

/** Install once on selected business APIs; read-only helpers are not traced recursively. */
export function traceKernel(prototype: object, operations: readonly string[], snapshot: (kernel: any) => unknown): void {
  for (const operation of operations) {
    const original = Object.getOwnPropertyDescriptor(prototype, operation)?.value;
    if (typeof original !== "function") throw new Error(`Missing diagnostic operation: ${operation}`);
    Object.defineProperty(prototype, operation, { configurable: true, writable: true, value: function(this: unknown, ...input: unknown[]) {
      if (!debugEnabled()) return original.apply(this, input);
      debugLog("operation.start", () => ({ operation, input: operationInput(operation, input), state: snapshot(this) }));
      try { return original.apply(this, input); }
      catch (error) {
        debugLog("operation.rejected", () => ({ operation, code: (error as {code?: string})?.code ?? "unmapped", reason: error instanceof Error ? error.message : String(error) }));
        throw error;
      } finally { debugLog("operation.finish", () => ({ operation, state: snapshot(this) })); }
    } });
  }
}
