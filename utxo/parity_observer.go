//go:build utmparity

package utxo

import "sort"

// ParityState is compiled only for the cross-language verifier, never production.
func (k *FundingKernel) ParityState() map[string]any {
	available := []any{}
	for _, key := range k.spendableOutpoints() {
		row := k.utxos[key]
		var occupied any
		if row.OccupiedBy != "" {
			occupied = row.OccupiedBy
		}
		available = append(available, map[string]any{"key": key, "txid": row.TxID, "vout": row.Vout, "value": row.Value, "scriptHex": row.ScriptHex, "status": row.Status, "height": row.Height, "source": row.Source, "occupiedBy": occupied, "used": row.Used})
	}
	names := []string{}
	for id := range k.allocations {
		names = append(names, id)
	}
	// Use the core's deterministic outpoint-independent ID ordering.
	sort.Strings(names)
	allocations := []AllocationView{}
	for _, id := range names {
		allocations = append(allocations, copyAllocation(k.allocations[id]))
	}
	funds := k.Funds()
	all := []any{}
	keys := []string{}
	for key := range k.utxos {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		row := k.utxos[key]
		var occupied any
		if row.OccupiedBy != "" {
			occupied = row.OccupiedBy
		}
		all = append(all, map[string]any{"key": key, "txid": row.TxID, "vout": row.Vout, "value": row.Value, "scriptHex": row.ScriptHex, "status": row.Status, "height": row.Height, "source": row.Source, "occupiedBy": occupied, "used": row.Used})
	}
	unavailable, accepted := []string{}, []string{}
	for key := range k.unavailable {
		unavailable = append(unavailable, key)
	}
	for key := range k.accepted {
		accepted = append(accepted, key)
	}
	sort.Strings(unavailable)
	sort.Strings(accepted)
	return map[string]any{"balance": funds.AvailableSats, "funds": funds, "available": available, "allocations": allocations, "internal": map[string]any{"utxos": all, "unavailable": unavailable, "accepted": accepted, "generation": k.generation, "revision": k.revision}}
}
