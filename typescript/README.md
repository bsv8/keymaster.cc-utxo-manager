# keymaster-utxo-manager

Memory-only wallet funding for trusted callers. No signing, broadcast, transaction bytes or persistence in the kernel.

```ts
const pool = new FundingKernel({owner, network}, {walletScriptHex, dustThresholdSats: '1'});
pool.applySnapshot({generation: 1, revision: 1, utxos});
const {allocation, inputs} = pool.request({
  requestId: 'payment', businessAmountSats: '1000', feeRateSatPerKb: '10',
  outputs: [{scriptHex: recipientScript, value: '1000'}],
});
// Pass these allocated inputs to the builder; never select externally.
pool.assertAllocationUsable(allocation.allocationId); // no returned data
pool.funds(); // {availableSats, frozenSats, version}; no raw UTXO queries
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
unknown broadcast results never release. Every success/already-known injects
`acceptTransaction({txid, spentOutpoints, outputs})`; spent inputs never return.
Protocol-only inputs need no wallet allocation; their returned wallet outputs
still enter through successful receipts. Discarding the instance discards state;
there is no restoration/compensation from caller journals.

Package version: 0.3.0. Typecheck/build with TypeScript using tsconfig.json.
Public registry availability and host dependency adoption are separate from the
package version. Shared docs and parity belong to the engineering repo.
Production must not compile verification-only observers.
