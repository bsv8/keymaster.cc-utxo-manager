// Package utxo is a trusted, memory-only UTXO allocation pool. It has no
// persisted schema, transaction submission or permission/verification workflow.
package utxo

import "fmt"

const MaxMoneySats uint64 = 2100000000000000
const BytesPerKB uint64 = 1000
const P2PKHInputWeight = 148
const (
	ErrInsufficient      = "insufficient"
	ErrNoUTXOs           = "no-utxos"
	ErrAmountInvalid     = "amount-invalid"
	ErrFeeInvalid        = "fee-invalid"
	ErrIDConflict        = "id-conflict"
	ErrSnapshotStale     = "snapshot-stale"
	ErrUnknownInput      = "unknown-input"
	ErrAlreadyOccupied   = "already-occupied"
	ErrAllocationUnknown = "allocation-unknown"
	ErrAllocationState   = "allocation-state"
	ErrTxInvalid         = "tx-invalid"
)

type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string {
	if e.Message == "" {
		return e.Code
	}
	return e.Code + ": " + e.Message
}
func CodeOf(err error) string {
	if e, ok := err.(*Error); ok {
		return e.Code
	}
	return fmt.Sprintf("unmapped:%v", err)
}
func fail(code, message string) *Error {
	debugLog("operation.rejected", func() any { return map[string]any{"code": code, "reason": message} })
	return &Error{Code: code, Message: message}
}
