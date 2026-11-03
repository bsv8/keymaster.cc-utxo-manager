// In-memory allocation pool. Callers own signing, submission and release decisions.
package utxo

import (
	"sort"
	"strconv"
	"strings"
)

type Scope struct {
	Owner   string `json:"owner"`
	Network string `json:"network"`
}
type ManagerConfig struct {
	WalletScriptHex   string `json:"walletScriptHex"`
	DustThresholdSats string `json:"dustThresholdSats"`
}
type SnapshotUtxo struct {
	TxID      string `json:"txid"`
	Vout      uint32 `json:"vout"`
	Value     string `json:"value"`
	ScriptHex string `json:"scriptHex"`
	Status    string `json:"status"`
	Height    int64  `json:"height"`
}
type SnapshotInput struct {
	Generation int64          `json:"generation"`
	Revision   int64          `json:"revision"`
	Utxos      []SnapshotUtxo `json:"utxos"`
}
type AcceptedOutput struct {
	Vout      uint32 `json:"vout"`
	Value     string `json:"value"`
	ScriptHex string `json:"scriptHex"`
}
type AcceptedTransaction struct {
	TxID           string           `json:"txid"`
	SpentOutpoints []string         `json:"spentOutpoints"`
	Outputs        []AcceptedOutput `json:"outputs"`
}
type RequestOutput struct {
	ScriptHex string `json:"scriptHex"`
	Value     string `json:"value"`
}
type RequestInput struct {
	RequestID      string          `json:"requestId"`
	BusinessAmount string          `json:"businessAmountSats"`
	FeeRate        string          `json:"feeRateSatPerKb"`
	Outputs        []RequestOutput `json:"outputs"`
}
type AllocationView struct {
	AllocationID string   `json:"allocationId"`
	State        string   `json:"state"`
	Inputs       []string `json:"inputs"`
	BusinessSats string   `json:"businessSats"`
	FeeSats      string   `json:"feeSats"`
	ChangeSats   string   `json:"changeSats"`
	TotalInSats  string   `json:"totalInSats"`
}
type PoolUtxo struct {
	SnapshotUtxo
	Source     string `json:"source"`
	OccupiedBy string `json:"occupiedBy"`
	Used       bool   `json:"used"`
}
type SnapshotResult struct {
	Added   int `json:"added"`
	Removed int `json:"removed"`
}
type FundingKernel struct {
	scope       Scope
	config      ManagerConfig
	utxos       map[string]*PoolUtxo
	allocations map[string]*AllocationView
	unavailable map[string]bool
	accepted    map[string]bool
	generation  int64
	revision    int64
}

func NewFundingKernel(scope Scope, config ManagerConfig) (*FundingKernel, error) {
	return &FundingKernel{scope: scope, config: config, utxos: map[string]*PoolUtxo{}, allocations: map[string]*AllocationView{}, unavailable: map[string]bool{}, accepted: map[string]bool{}}, nil
}
func poolKey(txid string, vout uint32) string {
	return strings.ToLower(txid) + ":" + strconv.FormatUint(uint64(vout), 10)
}
func (k *FundingKernel) ApplySnapshot(input SnapshotInput) (SnapshotResult, error) {
	defer k.debugTrace("applySnapshot", input)()
	result := SnapshotResult{}
	if input.Generation < k.generation || (input.Generation == k.generation && input.Revision <= k.revision) {
		return result, fail(ErrSnapshotStale, "")
	}
	incoming := map[string]SnapshotUtxo{}
	for _, row := range input.Utxos {
		incoming[poolKey(row.TxID, row.Vout)] = row
	}
	for key, row := range incoming {
		if k.unavailable[key] {
			continue
		}
		prior := k.utxos[key]
		source := "woc"
		if prior == nil {
			result.Added++
		}
		row.TxID = strings.ToLower(row.TxID)
		k.utxos[key] = &PoolUtxo{SnapshotUtxo: row, Source: source}
	}
	for key, row := range k.utxos {
		if row.Source == "woc" && !k.unavailable[key] {
			if _, ok := incoming[key]; !ok {
				delete(k.utxos, key)
				result.Removed++
			}
		}
	}
	k.generation = input.Generation
	k.revision = input.Revision
	return result, nil
}
func (k *FundingKernel) Request(input RequestInput) (AllocationView, error) {
	defer k.debugTrace("request", input)()
	if _, ok := k.allocations[input.RequestID]; ok {
		return AllocationView{}, fail(ErrIDConflict, "")
	}
	business, err := ParseAmount(input.BusinessAmount)
	if err != nil {
		return AllocationView{}, err
	}
	rate, err := ParseFeeRate(input.FeeRate)
	if err != nil {
		return AllocationView{}, err
	}
	dust, err := ParseAmount(k.config.DustThresholdSats)
	if err != nil {
		return AllocationView{}, err
	}
	pool := k.SpendableOutpoints()
	chosen := []string{}
	lens := []int{}
	for _, row := range input.Outputs {
		lens = append(lens, len(row.ScriptHex)/2)
	}
	var total, fee, change uint64
	covered := false
	for i, key := range pool {
		chosen = append(chosen, key)
		value, err := ParseAmount(k.utxos[key].Value)
		if err != nil {
			return AllocationView{}, err
		}
		total, err = AddChecked(total, value)
		if err != nil {
			return AllocationView{}, err
		}
		withLens := append(append([]int{}, lens...), len(k.config.WalletScriptHex)/2)
		size, err := EstimateSignedSize(len(chosen), withLens)
		if err != nil {
			return AllocationView{}, err
		}
		withChange, err := EstimateFeeSats(rate, size)
		if err != nil {
			return AllocationView{}, err
		}
		if total >= business+withChange+dust {
			fee = withChange
			change = total - business - fee
			covered = true
			break
		}
		size, err = EstimateSignedSize(len(chosen), lens)
		if err != nil {
			return AllocationView{}, err
		}
		without, err := EstimateFeeSats(rate, size)
		if err != nil {
			return AllocationView{}, err
		}
		if i == len(pool)-1 && total >= business+without {
			fee = total - business
			covered = true
		}
	}
	if !covered {
		code := ErrInsufficient
		if len(pool) == 0 {
			code = ErrNoUTXOs
		}
		return AllocationView{}, fail(code, "")
	}
	return k.freeze(input.RequestID, chosen, mustFormat(business), mustFormat(fee), mustFormat(change))
}
func copyAllocation(row *AllocationView) AllocationView {
	result := *row
	result.Inputs = append([]string{}, row.Inputs...)
	return result
}
func (k *FundingKernel) freeze(id string, keys []string, business, fee, change string) (AllocationView, error) {
	var total uint64
	for _, key := range keys {
		value, err := ParseAmount(k.utxos[key].Value)
		if err != nil {
			return AllocationView{}, err
		}
		total, err = AddChecked(total, value)
		if err != nil {
			return AllocationView{}, err
		}
	}
	for _, key := range keys {
		k.unavailable[key] = true
		k.utxos[key].OccupiedBy = id
	}
	row := &AllocationView{AllocationID: id, State: "held", Inputs: append([]string{}, keys...), BusinessSats: business, FeeSats: fee, ChangeSats: change, TotalInSats: mustFormat(total)}
	k.allocations[id] = row
	return copyAllocation(row), nil
}
func (k *FundingKernel) Release(id string) error {
	defer k.debugTrace("release", map[string]any{"allocationId": id})()
	row := k.allocations[id]
	if row == nil {
		return fail(ErrAllocationUnknown, "")
	}
	if row.State == "released" {
		return nil
	}
	if row.State == "used" {
		return fail(ErrAllocationState, "successful inputs are already used")
	}
	for _, key := range row.Inputs {
		u := k.utxos[key]
		if u != nil && u.OccupiedBy == id && !u.Used {
			u.OccupiedBy = ""
			delete(k.unavailable, key)
		}
	}
	row.State = "released"
	return nil
}

// AcceptTransaction applies trusted caller-confirmed successful prevouts and outputs.
// No transaction bytes, parsing, signature or result validation lives here.
func (k *FundingKernel) AcceptTransaction(tx AcceptedTransaction) error {
	defer k.debugTrace("acceptTransaction", tx)()
	txid := strings.ToLower(tx.TxID)
	if k.accepted[txid] {
		return nil
	}
	for _, input := range tx.SpentOutpoints {
		key := strings.ToLower(input)
		k.unavailable[key] = true
		if u := k.utxos[key]; u != nil {
			u.Used = true
			if allocation := k.allocations[u.OccupiedBy]; allocation != nil {
				allocation.State = "used"
			}
		}
	}
	k.accepted[txid] = true
	for _, output := range tx.Outputs {
		if output.ScriptHex != k.config.WalletScriptHex || output.Value == "0" {
			continue
		}
		key := poolKey(txid, output.Vout)
		if k.unavailable[key] {
			continue
		}
		status, source := "unconfirmed", "local"
		var height int64
		if prior := k.utxos[key]; prior != nil {
			status = prior.Status
			height = prior.Height
			source = prior.Source
		}
		k.utxos[key] = &PoolUtxo{SnapshotUtxo: SnapshotUtxo{TxID: txid, Vout: output.Vout, Value: output.Value, ScriptHex: output.ScriptHex, Status: status, Height: height}, Source: source}
	}
	return nil
}
func (k *FundingKernel) SpendableOutpoints() []string {
	keys := []string{}
	for key, row := range k.utxos {
		if !k.unavailable[key] && row.ScriptHex == k.config.WalletScriptHex {
			keys = append(keys, key)
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := k.utxos[keys[i]], k.utxos[keys[j]]
		av, _ := ParseAmount(a.Value)
		bv, _ := ParseAmount(b.Value)
		if av != bv {
			return av > bv
		}
		if a.TxID != b.TxID {
			return a.TxID < b.TxID
		}
		return a.Vout < b.Vout
	})
	return keys
}
func (k *FundingKernel) Lookup(key string) (PoolUtxo, error) {
	row := k.utxos[strings.ToLower(key)]
	if row == nil {
		return PoolUtxo{}, fail(ErrUnknownInput, key)
	}
	return *row, nil
}
func (k *FundingKernel) AllocationView(id string) (AllocationView, error) {
	row := k.allocations[id]
	if row == nil {
		return AllocationView{}, fail(ErrAllocationUnknown, id)
	}
	return copyAllocation(row), nil
}
func (k *FundingKernel) SpendableBalance() string {
	var total uint64
	for _, key := range k.SpendableOutpoints() {
		value, _ := ParseAmount(k.utxos[key].Value)
		total += value
	}
	return mustFormat(total)
}
