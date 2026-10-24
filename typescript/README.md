# keymaster-utxo-manager (TypeScript)

Deterministic UTXO manager kernel + trusted submit orchestration for
contract `utm/1`. Zero dependencies. The Go implementation in `../go`
is behavior-identical: same selection, fees, validation, errors and
byte-identical state serialization.

## Use

```ts
import { FundingKernel, submitSigned } from "keymaster-utxo-manager";

const kernel = new FundingKernel(
  { owner: ownerPublicKeyHex, network: "test" },
  { walletScriptHex, dustThresholdSats: "1", maxUnconfirmedAncestors: 8 },
);

kernel.applySnapshot({ generation: 1, revision: 1, utxos: [...] });
const view = kernel.request({
  requestId: "r1",
  businessAmountSats: "800",
  feeRateSatPerKb: "10",
  outputs: [{ scriptHex, value: "800" }],
});
// Build the template, have the trusted signer sign it, then:
kernel.prepare({ allocationId: "r1", unsignedTxHex });
kernel.completePrepare({ allocationId: "r1", signedTxHex });
// Persist the exact bytes BEFORE broadcasting, then:
await submitSigned(kernel, { store, broadcaster, chain }, "r1", signedTxHex);
```

Applications only see request/submit/status. Signing and broadcast stay
behind the host ports (`DurableStore`, `Broadcaster`, `ChainObserver`).

## Rules (utm/1)

- Amounts are canonical decimal strings; fee unit is satoshis per KB.
- Selection is stable largest-first; fees are `ceil(rate × estimatedSize / 1000)`.
- Unknown broadcasts keep inputs occupied and never enable change; only a
  reconcile of the SAME exact bytes settles them.
- Same scope/inputs/facts in, same outputs out — verified by the shared
  vector suite in BSV8_Engineering (`test/keymaster.cc-utxo-manager`).

See `../../BSV8_Engineering/docs/keymaster.cc-utxo-manager/` for established
facts and `../../BSV8_Engineering/todo/keymaster.cc-utxo-manager/` for the
contract draft and施工 records. Tests and E2E live in the engineering repo,
not here.
