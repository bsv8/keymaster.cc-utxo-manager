export { setDebugSink, debugEnabled } from "./debug.js";
export type { DebugEvent, DebugSink } from "./debug.js";
// In-memory allocation pool. Callers own transaction construction and submission.
export { MAX_MONEY_SATS, BYTES_PER_KB, P2PKH_INPUT_WEIGHT, ERROR_CODES, UtmError, codeOf } from "./contract.js";
export type { ErrorCode } from "./contract.js";
export { parseAmount, formatAmount, addChecked, sumChecked, estimateFeeSats, parseFeeRate } from "./amounts.js";
export { parseTx, buildUnsigned, assembleSigned, estimateSignedSize, encodeVarint, varintSize, sortedOutpointKeys, checkTxid } from "./txcodec.js";
export type { TxInput, TxOutput, ParsedTx, UnsignedTemplate } from "./txcodec.js";
export { sha256, dsha256, bytesToHex, hexToBytes, calcTxid } from "./hash.js";
export { FundingKernel } from "./kernel.js";
export type { P2pkhUtxo, Scope, ManagerConfig, SnapshotUtxo, SnapshotInput, RequestInput, RequestAllInput, AllocationView, AllocationMaterial, AllAllocationMaterial, Funds } from "./kernel.js";
