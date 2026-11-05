# keymaster-utxo-manager

Memory-only wallet funding for trusted callers. No signing, broadcast, transaction bytes or persistence in the kernel.

```ts
const pool = new FundingKernel({owner, network}, {walletScriptHex, dustThresholdSats: '1'});
pool.applySnapshot({generation: 1, revision: 1, utxos});
const {allocation, inputs} = pool.request({
  requestId: 'payment', businessAmountSats: '1000', feeRateSatPerKb: '10',
  outputScriptLengths: [recipientScript.length / 2],
});
// Pass these allocated inputs to the builder; never select externally.
pool.assertAllocationUsable(allocation.allocationId); // no returned data
pool.funds(); // {availableSats, version}; no raw UTXO queries
```

`requestAll({requestId})` atomically freezes all available inputs and returns
`{allocationId, inputs, totalInSats}`, without fee/recipient policy. The caller
calculates send-all fees and a single recipient output without change.

Requests return detached execution materials once. No public lookup,
allocationView, resolveAllocation or spendable-outpoint enumeration exists.
Funds queries do not distinguish confirmed/unconfirmed amounts or expose counts.
The funds version changes with wallet funding facts (including equal-value input
replacement), not confirmation metadata, and is scoped to the instance.

Before broadcast, any construction/signing error (including a missing signature
result) cancels the original intention and releases its allocation. Late signer
results must never dispatch. Explicit non-dispatch/rejection also releases;
unknown broadcast results never release. After success/already-known, the host
selects its wallet's nonzero P2PKH outputs and calls
`injectP2pkhUtxos([{txid, vout, value}])` only when there are outputs.
They are immediately allocatable, without waiting for WoC or block confirmation.
The host guarantees ownership, amounts and spendability on both injection and
snapshot paths; the pool does not check scripts/ownership and uses its configured
wallet script for execution materials. Deduplication is per outpoint and survives
pool removal; separate outputs of one transaction can be injected in batches.
There is no successful-transaction registry. Amounts are preflighted for the entire
batch, including duplicate outpoints, before any mutation. Noncanonical or
individually out-of-range amounts fail with amount-invalid; pool, deduplication,
locks and version remain unchanged. This is arithmetic, not ownership validation.
Input locks and allocation state stay unchanged;
callers must never release successful allocations. The pool does not track spent
inputs or prevent mistaken release, and has no frozen-amount query.
Protocol-only inputs need no wallet allocation; their returned wallet outputs
still enter through successful receipts. Discarding the instance discards state;
there is no restoration/compensation from caller journals.

Package API version: 0.5.0. Typecheck/build with TypeScript using tsconfig.json.
Public registry availability and host dependency adoption are separate from the
package version. Shared docs and parity belong to the engineering repo.
Production must not compile verification-only observers.
