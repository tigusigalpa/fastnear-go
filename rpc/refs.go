package rpc

// Finality selects how fresh a result must be.
type Finality string

const (
	// FinalityFinal is the canonical finalized state (default).
	FinalityFinal Finality = "final"
	// FinalityNearFinal is one step behind final, slightly fresher.
	FinalityNearFinal Finality = "near-final"
	// FinalityOptimistic is the freshest, not yet finalized state.
	FinalityOptimistic Finality = "optimistic"
)

// WaitUntil controls how far send_tx/tx_status wait for execution.
type WaitUntil string

const (
	// WaitNone returns as soon as the node accepts the transaction.
	WaitNone WaitUntil = "NONE"
	// WaitIncluded waits until the transaction is included in a block.
	WaitIncluded WaitUntil = "INCLUDED"
	// WaitIncludedFinal waits for inclusion in a finalized block.
	WaitIncludedFinal WaitUntil = "INCLUDED_FINAL"
	// WaitExecuted waits until all receipts finish executing.
	WaitExecuted WaitUntil = "EXECUTED"
	// WaitExecutedOptimistic waits for optimistic execution (default).
	WaitExecutedOptimistic WaitUntil = "EXECUTED_OPTIMISTIC"
	// WaitFinal waits for final execution on a finalized block.
	WaitFinal WaitUntil = "FINAL"
)

// ProofType is the subject of a light client proof.
type ProofType string

const (
	// ProofTransaction proves inclusion of the top-level transaction.
	ProofTransaction ProofType = "transaction"
	// ProofReceipt proves inclusion of a specific receipt.
	ProofReceipt ProofType = "receipt"
)

// BlockRef points at a block by height, hash or finality. Exactly one
// field is meaningful at a time; see the constructors.
type BlockRef struct {
	Height   *uint64
	Hash     *string
	Finality Finality
}

// Final refers to the latest finalized block.
func Final() BlockRef { return BlockRef{Finality: FinalityFinal} }

// AtFinality refers to a block at the given finality.
func AtFinality(f Finality) BlockRef { return BlockRef{Finality: f} }

// AtHeight refers to a block by its height.
func AtHeight(h uint64) BlockRef { return BlockRef{Height: &h} }

// AtHash refers to a block by its base58 hash.
func AtHash(hash string) BlockRef { return BlockRef{Hash: &hash} }

// queryParams renders the reference for `query` params: finality and
// block_id are mutually exclusive there.
func (r BlockRef) queryParams() map[string]any {
	if r.Height != nil {
		return map[string]any{"block_id": *r.Height}
	}
	if r.Hash != nil {
		return map[string]any{"block_id": *r.Hash}
	}
	f := r.Finality
	if f == "" {
		f = FinalityFinal
	}
	return map[string]any{"finality": string(f)}
}

// mergeBlockRef copies the rendered reference into params.
func mergeBlockRef(params map[string]any, at *BlockRef) {
	delete(params, "finality")
	delete(params, "block_id")
	r := Final()
	if at != nil {
		r = *at
	}
	for k, v := range r.queryParams() {
		params[k] = v
	}
}

// BlockID identifies a block by height or hash for methods whose
// params take a bare block_id.
type BlockID struct {
	Height *uint64
	Hash   *string
}

// HeightID is a block reference by height.
func HeightID(h uint64) BlockID { return BlockID{Height: &h} }

// HashID is a block reference by base58 hash.
func HashID(hash string) BlockID { return BlockID{Hash: &hash} }

// value renders the block_id parameter (uint64 or base58 string).
func (id BlockID) value() any {
	if id.Height != nil {
		return *id.Height
	}
	if id.Hash != nil {
		return *id.Hash
	}
	return nil
}

// CallFunctionRequest describes a contract view call.
type CallFunctionRequest struct {
	AccountID  string
	MethodName string
	// Args is JSON-marshaled and base64-encoded by the client.
	Args any
	At   *BlockRef
}

// CallFunctionRawRequest is a view call with pre-encoded base64 args.
type CallFunctionRawRequest struct {
	AccountID  string
	MethodName string
	ArgsBase64 string
	At         *BlockRef
}

// ViewStateRequest describes a contract storage scan.
type ViewStateRequest struct {
	AccountID string
	// PrefixBase64 selects the key prefix; "" scans the whole state
	// (expensive). Required by the API.
	PrefixBase64   string
	AfterKeyBase64 *string
	Limit          *uint32
	IncludeProof   bool
	At             *BlockRef
}

// AccessKeyListOptions paginates view_access_key_list.
type AccessKeyListOptions struct {
	AfterKey *string
	Limit    *uint32
	At       *BlockRef
}

// LightClientProofRequest describes a light client proof query.
// Exactly one of TransactionHash or ReceiptID must be set.
type LightClientProofRequest struct {
	Type            ProofType
	TransactionHash string
	ReceiptID       string
	// SenderID is the transaction sender (for transaction proofs) or
	// the receipt receiver (for receipt proofs).
	SenderID        string
	LightClientHead string
}

// StateChangesQuery describes a `changes` request. Build it with one
// of the constructors.
type StateChangesQuery struct {
	params map[string]any
}

// AccountChanges queries account-level changes for the given accounts.
func AccountChanges(accountIDs []string, at *BlockRef) StateChangesQuery {
	p := map[string]any{
		"changes_type": "account_changes",
		"account_ids":  accountIDs,
	}
	mergeBlockRef(p, at)
	return StateChangesQuery{params: p}
}

// DataChanges queries contract data changes under keyPrefixBase64 for
// the given accounts.
func DataChanges(accountIDs []string, keyPrefixBase64 string, at *BlockRef) StateChangesQuery {
	p := map[string]any{
		"changes_type":      "data_changes",
		"account_ids":       accountIDs,
		"key_prefix_base64": keyPrefixBase64,
	}
	mergeBlockRef(p, at)
	return StateChangesQuery{params: p}
}

// AllAccessKeyChanges queries access-key changes for the given
// accounts (all key kinds).
func AllAccessKeyChanges(accountIDs []string, at *BlockRef) StateChangesQuery {
	p := map[string]any{
		"changes_type": "all_access_key_changes",
		"account_ids":  accountIDs,
	}
	mergeBlockRef(p, at)
	return StateChangesQuery{params: p}
}
