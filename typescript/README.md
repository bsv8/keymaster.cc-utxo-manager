# keymaster-utxo-manager

Memory-only UTXO allocation pool for trusted internal callers. No persistence,
permission checks, signing, broadcasting, mempool tracking or transaction recovery.

```ts
import { FundingKernel } from 'keymaster-utxo-manager';
const pool = new FundingKernel(
  { owner, network: 'test' },
  { walletScriptHex, dustThresholdSats: '1' },
);
pool.applySnapshot({ generation: 1, revision: 1, utxos });
const allocation = pool.request({
  requestId: 'payment', businessAmountSats: '1000', feeRateSatPerKb: '10',
  outputs: [{ scriptHex: recipientScript, value: '1000' }],
});
const material = pool.resolveAllocation(allocation.allocationId);
// Caller keeps the original transaction plan using material.inputs (prevouts),
// allocation.feeSats and allocation.changeSats, then signs/submits that plan.
// resolveAllocation rejects released/used allocations; no host occupancy checks.
// Every successful submission (including already-known and protocol payouts):
pool.acceptTransaction({
  txid: successfulTxid,
  spentOutpoints: allocation.inputs,
  outputs: [{ vout: changeVout, value: changeSats, scriptHex: walletScriptHex }],
}); // caller supplies actual successful prevouts/outputs; change is immediately allocatable
// Explicit transaction rejection: caller may pool.release(allocation.allocationId).
// Unknown network result: do not release and do not inject unaccepted change.
```

`request` is the only allocation entry: callers supply funding needs, never selected
outpoints. Protocol-only inputs need no wallet allocation; payouts still inject wallet outputs.
`spendableBalance`, `spendableOutpoints`, `lookup` and `allocationView` are queries.
`resolveAllocation(id)` returns detached `{allocation, inputs}` for held allocations;
missing IDs report allocation-unknown, released/used allocations report allocation-state.
Business payment success comes from that exact transaction's receipt, never from allocation state.
The pool never accepts, parses or caches transaction bytes. Amounts use decimal strings; fees use sats/KB.

Old WoC responses cannot revive allocated/used inputs. Local outputs survive WoC
lag until first observed; later complete observations update chain-sourced funds.
Discard the instance to discard all state. Restart risks are explicitly unmanaged.

Build with `npm install && npm run build`. The memory-pool API is published as
`keymaster-utxo-manager@0.2.0`; the structured success-input, request-only allocation and allocation-material changes above are
source-only and not yet published. Shared tests/docs belong to the engineering repo;
source/candidate checks and public-release dependency checks are separate.
