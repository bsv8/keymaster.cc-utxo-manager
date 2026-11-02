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
export interface AcceptedTransaction {
  txid: string;
  spentOutpoints: string[];
  outputs: Array<{ vout: number; value: string; scriptHex: string }>;
}
export interface RequestInput {
  requestId: string; businessAmountSats: string; feeRateSatPerKb: string;
  outputs: Array<{ scriptHex: string; value: string }>;
}
export interface AllocationView {
  allocationId: string; state: "held" | "used" | "released"; inputs: string[];
  businessSats: string; feeSats: string; changeSats: string; totalInSats: string;
}
export interface PoolUtxo extends SnapshotUtxo { source: "woc" | "local"; occupiedBy: string | null; used: boolean }
const keyOf = (row: Pick<SnapshotUtxo, "txid" | "vout">) => `${row.txid.toLowerCase()}:${row.vout}`;

export class FundingKernel {
  private utxos = new Map<string, PoolUtxo>();
  private allocations = new Map<string, AllocationView>();
  // Includes inputs absent from a later snapshot; stale WoC must not revive them.
  private unavailable = new Set<string>();
  private accepted = new Set<string>();
  private generation = 0;
  private revision = 0;
  constructor(readonly scope: Scope, readonly config: ManagerConfig) {}

  applySnapshot(input: SnapshotInput): { added: number; removed: number } {
    if (input.generation < this.generation || (input.generation === this.generation && input.revision <= this.revision)) {
      throw new UtmError("snapshot-stale");
    }
    const incoming = new Map(input.utxos.map(row => [keyOf(row), row]));
    let added = 0, removed = 0;
    for (const [key, row] of incoming) {
      const prior = this.utxos.get(key);
      if (this.unavailable.has(key)) continue;
      if (!prior) added++;
      this.utxos.set(key, { ...row, txid: row.txid.toLowerCase(), source: "woc", occupiedBy: null, used: false });
    }
    for (const [key, row] of this.utxos) {
      if (row.source === "woc" && !this.unavailable.has(key) && !incoming.has(key)) {
        this.utxos.delete(key); removed++;
      }
    }
    this.generation = input.generation; this.revision = input.revision;
    return { added, removed };
  }

  request(input: RequestInput): AllocationView {
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
    return this.freeze(input.requestId, chosen, business.toString(), fee.toString(), change.toString());
  }

  /** Atomically allocate caller-selected wallet inputs; protocol inputs outside
   * this pool are not wallet funds and must not be inserted here. */
  allocate(requestId: string, outpoints: readonly string[]): AllocationView {
    if (this.allocations.has(requestId)) throw new UtmError("id-conflict");
    const keys = [...new Set(outpoints.map(key => key.toLowerCase()))];
    for (const key of keys) {
      if (!this.utxos.has(key)) throw new UtmError("unknown-input", key);
      if (this.unavailable.has(key)) throw new UtmError("already-occupied", key);
    }
    return this.freeze(requestId, keys, "0", "0", "0");
  }
  private freeze(id: string, keys: string[], business: string, fee: string, change: string): AllocationView {
    const view: AllocationView = { allocationId: id, state: "held", inputs: [...keys], businessSats: business,
      feeSats: fee, changeSats: change, totalInSats: keys.reduce((n, key) => addChecked(n, parseAmount(this.utxos.get(key)!.value)), 0n).toString() };
    for (const key of keys) { this.unavailable.add(key); this.utxos.get(key)!.occupiedBy = id; }
    this.allocations.set(id, view);
    return structuredClone(view);
  }
  release(allocationId: string): void {
    const allocation = this.allocations.get(allocationId);
    if (!allocation) throw new UtmError("allocation-unknown");
    if (allocation.state === "released") return;
    if (allocation.state === "used") throw new UtmError("allocation-state", "successful inputs are already used");
    for (const key of allocation.inputs) {
      const row = this.utxos.get(key);
      if (row?.occupiedBy === allocationId && !row.used) { row.occupiedBy = null; this.unavailable.delete(key); }
    }
    allocation.state = "released";
  }

  /** Caller confirms successful submission. Every caller must invoke this for
   * every successful transaction, including protocol payouts and already-known.
   * The trusted caller supplies prevouts and outputs, never transaction bytes. */
  acceptTransaction(tx: AcceptedTransaction): void {
    const txid = tx.txid.toLowerCase();
    if (this.accepted.has(txid)) return;
    for (const input of tx.spentOutpoints) {
      const key = input.toLowerCase(); this.unavailable.add(key);
      const row = this.utxos.get(key);
      if (row) {
        row.used = true;
        if (row.occupiedBy) {
          const allocation = this.allocations.get(row.occupiedBy);
          if (allocation) allocation.state = "used";
        }
      }
    }
    this.accepted.add(txid);
    tx.outputs.forEach(output => {
      if (output.scriptHex !== this.config.walletScriptHex || BigInt(output.value) === 0n) return;
      const key = `${txid}:${output.vout}`;
      if (this.unavailable.has(key)) return;
      const prior = this.utxos.get(key);
      this.utxos.set(key, { txid, vout: output.vout, value: output.value, scriptHex: output.scriptHex,
        status: prior?.status ?? "unconfirmed", height: prior?.height ?? 0, source: prior?.source ?? "local", occupiedBy: null, used: false });
    });
  }
  spendableOutpoints(): string[] {
    return [...this.utxos].filter(([key, row]) => !this.unavailable.has(key) && row.scriptHex === this.config.walletScriptHex)
      .sort(([a, x], [b, y]) => BigInt(x.value) === BigInt(y.value) ? (a.split(":")[0]! < b.split(":")[0]! ? -1 : a.split(":")[0]! > b.split(":")[0]! ? 1 : Number(a.split(":")[1]) - Number(b.split(":")[1])) : BigInt(x.value) > BigInt(y.value) ? -1 : 1)
      .map(([key]) => key);
  }
  lookup(outpoint: string): PoolUtxo {
    const row = this.utxos.get(outpoint.toLowerCase());
    if (!row) throw new UtmError("unknown-input", outpoint);
    return { ...row };
  }
  allocationView(id: string): AllocationView {
    const row = this.allocations.get(id);
    if (!row) throw new UtmError("allocation-unknown", id);
    return structuredClone(row);
  }
  spendableBalance(): string { return this.spendableOutpoints().reduce((sum, key) => sum + BigInt(this.utxos.get(key)!.value), 0n).toString(); }
}
traceKernel(FundingKernel.prototype, ["applySnapshot", "request", "allocate", "release", "acceptTransaction"],
  (pool: FundingKernel) => ({available: pool.spendableOutpoints(), balance: pool.spendableBalance()}));
