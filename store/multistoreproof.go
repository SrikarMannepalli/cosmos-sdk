package store

import (
	"bytes"
	"fmt"

	"github.com/tendermint/iavl"
	"github.com/tendermint/tendermint/crypto/merkle"
	cmn "github.com/tendermint/tendermint/libs/common"
)

// MultiStoreProof defines a collection of store proofs in a multi-store
type MultiStoreProof struct {
	StoreInfos []storeInfo
}

func NewMultiStoreProof(storeInfos []storeInfo) *MultiStoreProof {
	return &MultiStoreProof{StoreInfos: storeInfos}
}

func (proof *MultiStoreProof) ComputeRootHash() []byte {
	ci := commitInfo{
		Version:    -1, // Not needed. TODO: improve code
		StoreInfos: proof.StoreInfos,
	}
	return ci.Hash()
}

// RequireProof return whether proof is require for the subpath
func RequireProof(subpath string) bool {
	// XXX create a better convention.
	// Currently, only when query subpath is "/key", will proof be included in response.
	// If there are some changes about proof building in iavlstore.go, we must change code here to keep consistency with iavlstore.go:212
	if subpath == "/key" {
		return true
	}
	return false
}

//----------------------------------------

// Implements
type MultiStoreProofOp struct {
	// Encoded in ProofOp.Key
	key []byte

	// To encode in ProofOp.Data.
	Proof *MultiStoreProof `json:"proof"`
}

var _ merkle.ProofOperator = MultiStoreProofOp{}

const ProofOpMultiStoreProof = "multistore"

func NewMultiStoreProofOp(key []byte, proof *MultiStoreProof) MultiStoreProofOp {
	return MultiStoreProofOp{
		key:   key,
		Proof: proof,
	}
}

func MultiStoreProofOpDecoder(pop merkle.ProofOp) (merkle.ProofOperator, error) {
	if pop.Type != ProofOpMultiStoreProof {
		return nil, cmn.NewError("unexpected ProofOp.Type; got %v, want %v", pop.Type, ProofOpMultiStoreProof)
	}
	var op MultiStoreProofOp // a bit strange as we'll discard this, but it works.
	err := cdc.UnmarshalBinaryLengthPrefixed(pop.Data, &op)
	if err != nil {
		return nil, cmn.ErrorWrap(err, "decoding ProofOp.Data into MultiStoreProofOp")
	}
	return NewMultiStoreProofOp(pop.Key, op.Proof), nil
}

func (op MultiStoreProofOp) ProofOp() merkle.ProofOp {
	bz := cdc.MustMarshalBinaryLengthPrefixed(op)
	return merkle.ProofOp{
		Type: ProofOpMultiStoreProof,
		Key:  op.key,
		Data: bz,
	}
}

func (op MultiStoreProofOp) String() string {
	return fmt.Sprintf("MultiStoreProofOp{%v}", op.GetKey())
}

func (op MultiStoreProofOp) GetKey() []byte {
	return op.key
}

func (op MultiStoreProofOp) Run(args [][]byte) ([][]byte, error) {
	if len(args) != 1 {
		return nil, cmn.NewError("Value size is not 1")
	}
	value := args[0]
	root := op.Proof.ComputeRootHash()

	for _, si := range op.Proof.StoreInfos {
		if si.Name == string(op.key) {

			if bytes.Equal(value, si.Core.CommitID.Hash) {
				return [][]byte{root}, nil
			} else {
				return nil, cmn.NewError("hash mismatch for substore %v: %X vs %X", si.Name, si.Core.CommitID.Hash, value)
			}
		}
	}

	return nil, cmn.NewError("key %v not found in multistore proof", op.key)
}

//----------------------------------------

// XXX This should be managed by the rootMultiStore
// which may want to register more proof ops?
func DefaultProofRuntime() (prt *merkle.ProofRuntime) {
	prt = merkle.NewProofRuntime()
	prt.RegisterOpDecoder(merkle.ProofOpSimpleValue, merkle.SimpleValueOpDecoder)
	prt.RegisterOpDecoder(iavl.ProofOpIAVLValue, iavl.IAVLValueOpDecoder)
	prt.RegisterOpDecoder(iavl.ProofOpIAVLAbsence, iavl.IAVLAbsenceOpDecoder)
	return
}
