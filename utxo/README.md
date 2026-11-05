# Go memory UTXO pool

Import `github.com/bsv8/keymaster.cc-utxo-manager/utxo`.

- `Request`: select and freeze atomically; return `AllocationMaterial{Allocation, Inputs}` to the builder.
- `RequestAll`: freeze all available wallet inputs, return ID, signing facts and total; no fee/recipient policy.
- `Funds`: business-only `AvailableSats / FrozenSats / Version`; no UTXOs, counts or confirmation amount categories.
- `AssertAllocationUsable`: check held allocation without returning data.
- `Release`: explicit caller release, idempotent for released records, refuses successfully used inputs.
- `ApplySnapshot`: merge complete observations; stale observations cannot revive allocated/used inputs.
- `AcceptTransaction`: trusted structured successful inputs/outputs; own outputs immediately available, idempotent by txid.

No public Lookup, AllocationView method, ResolveAllocation or outpoint enumeration.
The AllocationView data type remains part of the request result, not a query.
Callers serialize access and own construction, signing, submission and evidence.
Any pre-broadcast error, including signing-result loss, cancels the original
intention before release; late signer results cannot dispatch. Explicit rejection
releases; broadcast uncertainty does not. Successful pool funding cannot release
its original wallet inputs: only protocol close/refund/arbitration may recover funds.

State is memory-only; restart does not restore wallet locks. Funds versions are
instance-local. Build with `go build ./...` (standard library only). The
`utmparity` build tag exists exclusively for full-state cross-language verification,
not production. This API belongs to root-module tag v0.3.0; host adoption and
npm publication are separate.
