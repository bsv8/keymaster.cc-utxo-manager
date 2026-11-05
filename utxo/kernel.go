// In-memory allocation pool. Callers own signing, submission and release decisions.
package utxo

import (
	"encoding/json"
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
type AllocationMaterial struct {
	Allocation AllocationView `json:"allocation"`
	Inputs     []SnapshotUtxo `json:"inputs"`
}
type RequestAllInput struct {
	RequestID string `json:"requestId"`
}
type AllAllocationMaterial struct {
	AllocationID string         `json:"allocationId"`
	Inputs       []SnapshotUtxo `json:"inputs"`
	TotalInSats  string         `json:"totalInSats"`
}
type Funds struct {
	AvailableSats string `json:"availableSats"`
	FrozenSats    string `json:"frozenSats"`
	Version       int64  `json:"version"`
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
	scope        Scope
	config       ManagerConfig
	utxos        map[string]*PoolUtxo
	allocations  map[string]*AllocationView
	unavailable  map[string]bool
	accepted     map[string]bool
	generation   int64
	revision     int64
	fundsVersion int64
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
	before := k.fundsIdentity()
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
	if before != k.fundsIdentity() {
		k.fundsVersion++
	}
	return result, nil
}
func (k *FundingKernel) Request(input RequestInput) (AllocationMaterial, error) {
	view, err := k.request(input)
	if err != nil {
		return AllocationMaterial{}, err
	}
	return k.allocationMaterial(view.AllocationID)
}
func (k *FundingKernel) request(input RequestInput) (AllocationView, error) {
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
	pool := k.spendableOutpoints()
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

// RequestAll freezes every available wallet input without transaction policy.
// Callers serialize access, as with all other kernel operations.
func (k *FundingKernel) RequestAll(input RequestAllInput) (AllAllocationMaterial, error) {
	defer k.debugTrace("requestAll", input)()
	if _, ok := k.allocations[input.RequestID]; ok {
		return AllAllocationMaterial{}, fail(ErrIDConflict, "")
	}
	keys := k.spendableOutpoints()
	if len(keys) == 0 {
		return AllAllocationMaterial{}, fail(ErrNoUTXOs, "")
	}
	// Legacy request-only amount fields are zero, not a transaction fee plan.
	// freeze validates the entire sum before changing any occupancy.
	allocation, err := k.freeze(input.RequestID, keys, "0", "0", "0")
	if err != nil {
		return AllAllocationMaterial{}, err
	}
	material, err := k.allocationMaterial(allocation.AllocationID)
	if err != nil {
		return AllAllocationMaterial{}, err
	}
	return AllAllocationMaterial{AllocationID: allocation.AllocationID, Inputs: material.Inputs, TotalInSats: allocation.TotalInSats}, nil
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
	k.fundsVersion++
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
	k.fundsVersion++
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
	before := k.fundsIdentity()
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
	if before != k.fundsIdentity() {
		k.fundsVersion++
	}
	return nil
}
func (k *FundingKernel) spendableOutpoints() []string {
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
func (k *FundingKernel) readAllocation(id string) (AllocationView, error) {
	row := k.allocations[id]
	if row == nil {
		return AllocationView{}, fail(ErrAllocationUnknown, id)
	}
	return copyAllocation(row), nil
}

// allocationMaterial is internal; signing facts leave the kernel only as
// part of a successful allocation, never through a general query.
func (k *FundingKernel) allocationMaterial(id string) (AllocationMaterial, error) {
	view, err := k.readAllocation(id)
	if err != nil {
		return AllocationMaterial{}, err
	}
	if view.State != "held" {
		return AllocationMaterial{}, fail(ErrAllocationState, "")
	}
	inputs := []SnapshotUtxo{}
	for _, key := range view.Inputs {
		inputs = append(inputs, k.utxos[key].SnapshotUtxo)
	}
	return AllocationMaterial{Allocation: view, Inputs: inputs}, nil
}
func (k *FundingKernel) AssertAllocationUsable(id string) error {
	row := k.allocations[id]
	if row == nil {
		return fail(ErrAllocationUnknown, "")
	}
	if row.State != "held" {
		return fail(ErrAllocationState, "")
	}
	return nil
}
func (k *FundingKernel) Funds() Funds {
	var available, frozen uint64
	for key, row := range k.utxos {
		if row.ScriptHex != k.config.WalletScriptHex || row.Used {
			continue
		}
		value, _ := ParseAmount(row.Value)
		if !k.unavailable[key] {
			available += value
		} else if a := k.allocations[row.OccupiedBy]; a != nil && a.State == "held" {
			frozen += value
		}
	}
	return Funds{AvailableSats: mustFormat(available), FrozenSats: mustFormat(frozen), Version: k.fundsVersion}
}
func (k *FundingKernel) fundsIdentity() string {
	keys := []string{}
	for key, row := range k.utxos {
		if row.ScriptHex == k.config.WalletScriptHex {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	rows := []any{}
	for _, key := range keys {
		row := k.utxos[key]
		var occupied, state any
		if row.OccupiedBy != "" {
			occupied = row.OccupiedBy
			if a := k.allocations[row.OccupiedBy]; a != nil {
				state = a.State
			}
		}
		rows = append(rows, []any{key, row.Value, k.unavailable[key], row.Used, occupied, state})
	}
	raw, _ := json.Marshal(rows)
	return string(raw)
}
