# Go memory UTXO pool

```go
import "github.com/bsv8/keymaster.cc-utxo-manager/utxo"

pool, _ := utxo.NewFundingKernel(
    utxo.Scope{Owner: owner, Network: "test"},
    utxo.ManagerConfig{WalletScriptHex: walletScript, DustThresholdSats: "1"},
)
// ApplySnapshot: complete WoC available UTXOs.
// Request or Allocate: select and freeze inputs immediately.
// Caller constructs/signs/submits the transaction.
// On EVERY successful submission (including already-known/payout):
err := pool.AcceptTransaction(successfulRawHex)
// Caller explicitly releases rejected submissions with pool.Release(id).
// Unknown network results never release inputs or enable change.
```

State is memory-only, not thread-safe: callers serialize access. There are no
permission, signature-validation, broadcast, mempool, conflict-observation or
persistence/recovery APIs. Queries expose available UTXOs/balance, allocation
records and successfully injected parent bytes. Discarding an instance discards
all state; restart risks are intentionally unmanaged.

Build: `go build ./...`. Uses only the standard library. TS and Go share the same
allocation contract; tests and current docs live in the engineering repo. This
new API is candidate source, not a claim that a matching release is published.
