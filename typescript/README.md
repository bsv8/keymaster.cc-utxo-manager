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
// Caller constructs, signs and submits using allocation.inputs.
// Every successful submission (including already-known and protocol payouts):
pool.acceptTransaction(successfulRawTxHex); // change is immediately allocatable
// Explicit transaction rejection: caller may pool.release(allocation.allocationId).
// Unknown network result: do not release and do not inject unaccepted change.
```

`allocate(id, outpoints)` atomically freezes caller-selected pool inputs.
`spendableBalance`, `spendableOutpoints`, `lookup`, `allocationView` and
`previousTransaction` are queries. Amounts use decimal strings; fees use sats/KB.

Old WoC responses cannot revive allocated/used inputs. Local outputs survive WoC
lag until first observed; later complete observations update chain-sourced funds.
Discard the instance to discard all state. Restart risks are explicitly unmanaged.

Build with `npm install && npm run build`. The memory-pool API is published as
`keymaster-utxo-manager@0.2.0`. Shared tests/docs belong to the engineering repo;
source/candidate checks and public-release dependency checks are separate.
