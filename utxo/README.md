# Go memory UTXO pool

Import `github.com/bsv8/keymaster.cc-utxo-manager/utxo`.

- `Request`: select and freeze atomically; return flat `AllocationMaterial{AllocationID, Inputs, BusinessSats, FeeSats, ChangeSats, TotalInSats}` to the builder.
- `RequestAll`: freeze all available wallet inputs, return ID, signing facts and total; no fee/recipient policy.
- `Funds`: business-only `AvailableSats / Version`; no UTXOs, counts or confirmation amount categories.
- `AssertAllocationUsable`: check held allocation without returning data.
- `Release`: trusted explicit caller release, idempotent for released records; no success-state guard.
- `ApplySnapshot`: merge complete observations; stale observations cannot revive allocated inputs that have not been released.
- `InjectP2pkhUtxos`: inject `[]P2pkhUtxo{{TxID, Vout, Value}}`, immediately allocatable without waiting for WoC/confirmation. Hosts guarantee wallet P2PKH ownership and spendability on both input paths; the kernel stores no script and returns script-free materials. Change-fee estimation uses the fixed P2PKH locking-script length 25. Deduplicate by outpoint, including after pool removal; allow separate batches from one transaction. No successful-transaction registry or input/allocation transitions. Skip the call when no own outputs exist.

Injection preflights canonical amounts and individual max-money bounds for the
entire batch, including duplicate outpoints, before mutation. Any amount-invalid
error leaves pool, deduplication, locks and version unchanged. This is arithmetic,
not script or ownership validation.

No public Lookup, internal record exposure, ResolveAllocation or outpoint enumeration.
The internal allocation record remains storage-only and is never returned; public
results are flat materials without state or duplicate string inputs.
Callers serialize access and own construction, signing, submission and evidence.
Any pre-broadcast error, including signing-result loss, cancels the original
intention before release; late signer results cannot dispatch. Explicit rejection
releases; broadcast uncertainty does not. Successful pool funding cannot release
its original wallet inputs: only protocol close/refund/arbitration may recover funds.

State is memory-only; restart does not restore wallet locks. Funds versions are
instance-local. Build with `go build ./...` (standard library only). The
`utmparity` build tag exists exclusively for full-state cross-language verification,
not production. API version: root-module tag v0.6.0 candidate (published: v0.5.0).
Host dependency adoption and npm publication are separate.
