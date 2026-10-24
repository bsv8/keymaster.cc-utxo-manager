# keymaster.cc-utxo-manager (Go)

Deterministic UTXO manager kernel + trusted submit orchestration for
contract `utm/1`. No external dependencies. The TypeScript implementation
in `../typescript` is behavior-identical: same selection, fees, validation,
errors and byte-identical state serialization.

## Use

```go
import "github.com/bsv8/keymaster.cc-utxo-manager/utxo"

kernel, _ := utxo.NewFundingKernel(utxo.Scope{Owner: ownerHex, Network: "test"},
    utxo.ManagerConfig{WalletScriptHex: walletScript, DustThresholdSats: "1", MaxUnconfirmedAncestors: 8})

kernel.ApplySnapshot(utxo.SnapshotInput{Generation: 1, Revision: 1, Utxos: [...]})
view, _ := kernel.Request(utxo.RequestInput{RequestID: "r1", BusinessAmount: "800",
    FeeRate: "10", Outputs: []utxo.RequestOutput{{ScriptHex: script, Value: "800"}}})
// Build the template, have the trusted signer sign it, then:
kernel.Prepare(utxo.PrepareInput{AllocationID: "r1", UnsignedTxHex: unsigned})
kernel.CompletePrepare(utxo.CompletePrepareInput{AllocationID: "r1", SignedTxHex: signed})
// Persist the exact bytes BEFORE broadcasting, then:
utxo.SubmitSigned(kernel, utxo.Ports{Store: store, Broadcaster: bc, Chain: chain}, "r1", signed)
```

Host adapters (keymaster.cc-cli wallet, keymaster.cc Worker) own the
`DurableStore`/`Broadcaster`/`ChainObserver` ports. Applications never see
signing or broadcast capabilities.

See `../../BSV8_Engineering/docs/keymaster.cc-utxo-manager/` for established
facts and `../../BSV8_Engineering/todo/keymaster.cc-utxo-manager/` for the
contract draft and施工 records. Tests and E2E live in the engineering repo,
not here.
