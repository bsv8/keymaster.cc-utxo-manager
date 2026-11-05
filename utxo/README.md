# Go memory UTXO pool

```go
import "github.com/bsv8/keymaster.cc-utxo-manager/utxo"

pool, _ := utxo.NewFundingKernel(
    utxo.Scope{Owner: owner, Network: "test"},
    utxo.ManagerConfig{WalletScriptHex: walletScript, DustThresholdSats: "1"},
)
// ApplySnapshot: complete WoC available UTXOs.
// Request: select and freeze inputs by funding needs.
// RequestAll(RequestAllInput{RequestID: id}): freeze all available wallet inputs.
// Returns AllAllocationMaterial{AllocationID, Inputs, TotalInSats}; no fee/recipient policy.
// Caller computes send-all fees/output from frozen inputs, with no change or second request.
// Preparation failures release the original ID; unknown signing/submission results do not.
// Protocol-only inputs need no wallet allocation; successful payouts still inject outputs.
// ResolveAllocation(id): detached held allocation + prevouts, including amount/script.
// Released/used allocations fail; hosts do not recheck internal occupancy fields.
// Caller retains the original fee/change plan and constructs/signs/submits it.
// Allocation state is not a payment-success receipt.
// On EVERY successful submission (including already-known/payout):
err := pool.AcceptTransaction(utxo.AcceptedTransaction{
    TxID: successfulTxid,
    SpentOutpoints: actualSpentOutpoints,
    Outputs: []utxo.AcceptedOutput{{Vout: changeVout, Value: changeSats, ScriptHex: walletScript}},
})
// Caller explicitly releases rejected submissions with pool.Release(id).
// Unknown network results never release inputs or enable change.
```

RequestAll includes small inputs but excludes held/used and non-wallet inputs.
Empty pools fail with no-utxos, reused IDs with id-conflict; invalid sums fail
without partial freezing. Returned input facts are detached. Its shared internal
AllocationView has zero business/fee/change fields, not a transaction plan.

State is memory-only, not thread-safe: callers serialize access. There are no
permission, signature-validation, broadcast, mempool, conflict-observation or
persistence/recovery APIs. Queries expose available UTXOs/balance, allocation
records. No transaction bytes are accepted, parsed or cached. Discarding an instance discards
all state; restart risks are intentionally unmanaged.

Build: `go build ./...`. Uses only the standard library. TS and Go share the same
allocation contract; tests and current docs live in the engineering repo. This
memory-pool API is published at the root module tag `v0.2.0`; the structured
success-input, request/requestAll allocation and allocation-material changes above are not yet published. Source/candidate
checks do not substitute for public-release dependency checks.
