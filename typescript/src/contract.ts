// Memory pool error codes. No wire or persisted-state schema is defined here.
export const MAX_MONEY_SATS = "2100000000000000";
export const BYTES_PER_KB = 1000;
export const FEE_UNIT = "satoshisPerKb";
export const SELECTION_STRATEGY = "largest-first";
export const TX_BASE_BYTES = 10;
export const P2PKH_INPUT_WEIGHT = 148;
export const OUTPUT_OVERHEAD = 8;
export const ERROR_CODES = ["insufficient", "no-utxos", "amount-invalid", "fee-invalid", "id-conflict", "snapshot-stale", "unknown-input", "already-occupied", "allocation-unknown", "allocation-state", "tx-invalid"] as const;
export type ErrorCode = typeof ERROR_CODES[number];
export class UtmError extends Error {
 readonly code: ErrorCode;
 constructor(code: ErrorCode, message?: string) {super(message ? `${code}: ${message}` : code);this.name = "UtmError";this.code = code;}
}
export function codeOf(error: unknown): string {
 if (error instanceof UtmError) return error.code;
 return `unmapped:${error == null ? "<nil>" : error instanceof Error ? error.message : String(error)}`;
}
