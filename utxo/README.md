# Go memory UTXO pool

```go
import "github.com/bsv8/keymaster.cc-utxo-manager/utxo"

pool, _ := utxo.NewFundingKernel(
    utxo.Scope{Owner: owner, Network: "test"},
    utxo.ManagerConfig{WalletScriptHex: walletScript, DustThresholdSats: "1"},
)
// ApplySnapshot: complete WoC available UTXOs.
// Request: the only entry to select and freeze wallet inputs immediately.
// Protocol-only inputs need no wallet allocation; successful payouts still inject outputs.
// Caller constructs/signs/submits the transaction.
// On EVERY successful submission (including already-known/payout):
err := pool.AcceptTransaction(utxo.AcceptedTransaction{
    TxID: successfulTxid,
    SpentOutpoints: actualSpentOutpoints,
    Outputs: []utxo.AcceptedOutput{{Vout: changeVout, Value: changeSats, ScriptHex: walletScript}},
})
// Caller explicitly releases rejected submissions with pool.Release(id).
// Unknown network results never release inputs or enable change.
```

State is memory-only, not thread-safe: callers serialize access. There are no
permission, signature-validation, broadcast, mempool, conflict-observation or
persistence/recovery APIs. Queries expose available UTXOs/balance, allocation
records. No transaction bytes are accepted, parsed or cached. Discarding an instance discards
all state; restart risks are intentionally unmanaged.

Build: `go build ./...`. Uses only the standard library. TS and Go share the same
allocation contract; tests and current docs live in the engineering repo. This
memory-pool API is published at the root module tag `v0.2.0`; the structured
success-input and request-only allocation changes above are not yet published. Source/candidate
checks do not substitute for public-release dependency checks.
