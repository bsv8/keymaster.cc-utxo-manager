// In-memory UTXO allocation pool. Callers are trusted; no signing, broadcast,
// transaction-result recovery, mempool tracking or persistence lives here.
import { UtmError } from "./contract.js";
import { traceKernel } from "./debug.js";
import { estimateFeeSats, parseAmount, parseFeeRate, addChecked } from "./amounts.js";
import { estimateSignedSize } from "./txcodec.js";

export interface Scope { owner: string; network: "main" | "test" }
export interface ManagerConfig { walletScriptHex: string; dustThresholdSats: string }
export interface SnapshotUtxo {
  txid: string; vout: number; value: string; scriptHex: string;
  status: "confirmed" | "unconfirmed"; height: number;
}
export interface SnapshotInput { generation: number; revision: number; utxos: SnapshotUtxo[] }
export interface P2pkhUtxo { txid: string; vout: number; value: string }
export interface RequestInput {
  requestId: string; businessAmountSats: string; feeRateSatPerKb: string;
  outputs: Array<{ scriptHex: string; value: string }>;
}
export interface AllocationView {
  allocationId: string; state: "held" | "released"; inputs: string[];
  businessSats: string; feeSats: string; changeSats: string; totalInSats: string;
}
export interface AllocationMaterial { allocation: AllocationView; inputs: SnapshotUtxo[] }
export interface RequestAllInput { requestId: string }
export interface AllAllocationMaterial { allocationId: string; inputs: SnapshotUtxo[]; totalInSats: string }
export interface Funds { availableSats: string; version: number }
interface PoolUtxo extends SnapshotUtxo { source: "woc" | "local"; occupiedBy: string | null }
const keyOf = (row: Pick<SnapshotUtxo, "txid" | "vout">) => `${row.txid.toLowerCase()}:${row.vout}`;

export class FundingKernel {
  private utxos = new Map<string, PoolUtxo>();
  private allocations = new Map<string, AllocationView>();
  // Includes inputs absent from a later snapshot; stale WoC must not revive them.
  private unavailable = new Set<string>();
  // Injection deduplication survives removal from the current pool.
  private injected = new Set<string>();
  private generation = 0;
  private revision = 0;
  private fundsVersion = 0;
  constructor(readonly scope: Scope, readonly config: ManagerConfig) {}

  applySnapshot(input: SnapshotInput): { added: number; removed: number } {
    if (input.generation < this.generation || (input.generation === this.generation && input.revision <= this.revision)) {
      throw new UtmError("snapshot-stale");
    }
    const before = this.fundsIdentity();
    const incoming = new Map(input.utxos.map(row => [keyOf(row), row]));
    let added = 0, removed = 0;
    for (const [key, row] of incoming) {
      const prior = this.utxos.get(key);
      if (this.unavailable.has(key)) continue;
      if (!prior) added++;
      this.utxos.set(key, { ...row, txid: row.txid.toLowerCase(), scriptHex: this.config.walletScriptHex, source: "woc", occupiedBy: null });
    }
    for (const [key, row] of this.utxos) {
      if (row.source === "woc" && !this.unavailable.has(key) && !incoming.has(key)) {
        this.utxos.delete(key); removed++;
      }
    }
    this.generation = input.generation; this.revision = input.revision;
    if (before !== this.fundsIdentity()) this.fundsVersion++;
    return { added, removed };
  }

  request(input: RequestInput): AllocationMaterial {
    if (this.allocations.has(input.requestId)) throw new UtmError("id-conflict");
    const business = parseAmount(input.businessAmountSats);
    const rate = parseFeeRate(input.feeRateSatPerKb);
    const dust = parseAmount(this.config.dustThresholdSats);
    const chosen: string[] = [];
    let total = 0n, fee = 0n, change = 0n, covered = false;
    const pool = this.spendableOutpoints();
    const lens = input.outputs.map(row => row.scriptHex.length / 2);
    for (let i = 0; i < pool.length; i++) {
      const key = pool[i]!; chosen.push(key); total = addChecked(total, parseAmount(this.utxos.get(key)!.value));
      const withChange = estimateFeeSats(rate, estimateSignedSize(chosen.length, [...lens, this.config.walletScriptHex.length / 2]));
      if (total >= business + withChange + dust) {
        fee = withChange; change = total - business - fee; covered = true; break;
      }
      if (i === pool.length - 1 && total >= business + estimateFeeSats(rate, estimateSignedSize(chosen.length, lens))) {
        fee = total - business; covered = true;
      }
    }
    if (!covered) throw new UtmError(pool.length ? "insufficient" : "no-utxos");
    const allocation = this.freeze(input.requestId, chosen, business.toString(), fee.toString(), change.toString());
    return this.allocationMaterial(allocation.allocationId);
  }

  /** Freeze every currently available wallet input, without any transaction policy. */
  requestAll(input: RequestAllInput): AllAllocationMaterial {
    if (this.allocations.has(input.requestId)) throw new UtmError("id-conflict");
    const keys = this.spendableOutpoints();
    if (!keys.length) throw new UtmError("no-utxos");
    // Legacy request-only amount fields remain unset (zero); callers own the
    // send-all amount/fee plan. freeze validates the entire sum before mutation.
    const allocation = this.freeze(input.requestId, keys, "0", "0", "0");
    return { allocationId: allocation.allocationId, totalInSats: allocation.totalInSats,
      inputs: this.allocationMaterial(allocation.allocationId).inputs };
  }

  private freeze(id: string, keys: string[], business: string, fee: string, change: string): AllocationView {
    const view: AllocationView = { allocationId: id, state: "held", inputs: [...keys], businessSats: business,
      feeSats: fee, changeSats: change, totalInSats: keys.reduce((n, key) => addChecked(n, parseAmount(this.utxos.get(key)!.value)), 0n).toString() };
    for (const key of keys) { this.unavailable.add(key); this.utxos.get(key)!.occupiedBy = id; }
    this.allocations.set(id, view);
    this.fundsVersion++;
    return structuredClone(view);
  }
  release(allocationId: string): void {
    const allocation = this.allocations.get(allocationId);
    if (!allocation) throw new UtmError("allocation-unknown");
    if (allocation.state === "released") return;
    for (const key of allocation.inputs) {
      const row = this.utxos.get(key);
      if (row?.occupiedBy === allocationId) { row.occupiedBy = null; this.unavailable.delete(key); }
    }
    allocation.state = "released";
    this.fundsVersion++;
  }

  /** Inject immediately allocatable P2PKH outputs. The caller guarantees wallet
   * ownership and spendability; no script or transaction-result checks live here.
   * Input locks are unchanged. Successful allocations must not be released. */
  injectP2pkhUtxos(outputs: readonly P2pkhUtxo[]): void {
    // Validate the entire batch before changing either money or deduplication.
    // This is amount arithmetic, not script ownership or transaction validation.
    for (const output of outputs) parseAmount(output.value);
    const before = this.fundsIdentity();
    for (const output of outputs) {
      if (output.value === "0") continue;
      const key = keyOf(output);
      if (this.injected.has(key)) continue;
      this.injected.add(key);
      if (this.unavailable.has(key) || this.utxos.has(key)) continue;
      this.utxos.set(key, { ...output, txid: output.txid.toLowerCase(), scriptHex: this.config.walletScriptHex,
        status: "unconfirmed", height: 0, source: "local", occupiedBy: null });
    }
    if (before !== this.fundsIdentity()) this.fundsVersion++;
  }
  private spendableOutpoints(): string[] {
    return [...this.utxos].filter(([key]) => !this.unavailable.has(key))
      .sort(([a, x], [b, y]) => BigInt(x.value) === BigInt(y.value) ? (a.split(":")[0]! < b.split(":")[0]! ? -1 : a.split(":")[0]! > b.split(":")[0]! ? 1 : Number(a.split(":")[1]) - Number(b.split(":")[1])) : BigInt(x.value) > BigInt(y.value) ? -1 : 1)
      .map(([key]) => key);
  }
  private readAllocation(id: string): AllocationView {
    const row = this.allocations.get(id);
    if (!row) throw new UtmError("allocation-unknown", id);
    return structuredClone(row);
  }
  /** Internal materialization for successful allocation results only. */
  private allocationMaterial(id: string): AllocationMaterial {
    const allocation = this.readAllocation(id);
    if (allocation.state !== "held") throw new UtmError("allocation-state");
    return { allocation, inputs: allocation.inputs.map(key => {
      const {txid, vout, value, scriptHex, status, height} = this.utxos.get(key)!;
      return {txid, vout, value, scriptHex, status, height};
    }) };
  }
  assertAllocationUsable(id: string): void {
    const allocation = this.allocations.get(id);
    if (!allocation) throw new UtmError("allocation-unknown");
    if (allocation.state !== "held") throw new UtmError("allocation-state");
  }
  funds(): Funds {
    let available = 0n;
    for (const [key, row] of this.utxos) {
      if (!this.unavailable.has(key)) available += BigInt(row.value);
    }
    return {availableSats: available.toString(), version: this.fundsVersion};
  }
  private fundsIdentity(): string {
    return JSON.stringify([...this.utxos]
      .map(([key, row]) => [key, row.value, this.unavailable.has(key), row.occupiedBy, row.occupiedBy ? this.allocations.get(row.occupiedBy)?.state : null])
      .sort((a, b) => String(a[0]).localeCompare(String(b[0]))));
  }
}
traceKernel(FundingKernel.prototype, ["applySnapshot", "request", "requestAll", "release", "injectP2pkhUtxos"],
  (pool: FundingKernel) => pool.funds());
