package state

import (
	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/blocks"
	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/crypto"
	"io"
)

const (
	AccountDetailStatusNominal   = byte(0x00)
	AccountDetailStatusBonded    = byte(0x01)
	AccountDetailStatusUnbonding = byte(0x02)
)

type Account struct {
	Id     uint64 // Numeric id of account, incrementing.
	PubKey []byte
}

func ReadAccount(r io.Reader, n *int64, err *error) Account {
	return Account{
		Id:     ReadUInt64(r, n, err),
		PubKey: ReadByteSlice(r, n, err),
	}
}

func (account Account) WriteTo(w io.Writer) (n int64, err error) {
	WriteUInt64(w, account.Id, &n, &err)
	WriteByteSlice(w, account.PubKey, &n, &err)
	return
}

func (account Account) VerifyBytes(msg []byte, sig Signature) bool {
	if sig.SignerId != account.Id {
		panic("account.id doesn't match sig.signerid")
	}
	v1 := &crypto.Verify{
		Message:   msg,
		PubKey:    account.PubKey,
		Signature: sig.Bytes,
	}
	ok := crypto.VerifyBatch([]*crypto.Verify{v1})
	return ok
}

func (account Account) Verify(o Signable) bool {
	sig := o.GetSignature()
	o.SetSignature(Signature{}) // clear
	msg := BinaryBytes(o)
	o.SetSignature(sig) // restore
	return account.VerifyBytes(msg, sig)
}

//-----------------------------------------------------------------------------

type AccountDetail struct {
	Account
	Sequence uint64
	Balance  uint64
	Status   byte
}

func ReadAccountDetail(r io.Reader, n *int64, err *error) *AccountDetail {
	return &AccountDetail{
		Account:  ReadAccount(r, n, err),
		Sequence: ReadUInt64(r, n, err),
		Balance:  ReadUInt64(r, n, err),
		Status:   ReadByte(r, n, err),
	}
}

func (accDet AccountDetail) WriteTo(w io.Writer) (n int64, err error) {
	WriteBinary(w, accDet.Account, &n, &err)
	WriteUInt64(w, accDet.Sequence, &n, &err)
	WriteUInt64(w, accDet.Balance, &n, &err)
	WriteByte(w, accDet.Status, &n, &err)
	return
}

//-----------------------------------------------------------------------------

type PrivAccount struct {
	Account
	PrivKey []byte
}

// Generates a new account with private key.
// The Account.Id is empty since it isn't in the blockchain.
func GenPrivAccount() *PrivAccount {
	privKey := CRandBytes(32)
	pubKey := crypto.MakePubKey(privKey)
	return &PrivAccount{
		Account: Account{
			Id:     uint64(0),
			PubKey: pubKey,
		},
		PrivKey: privKey,
	}
}

func (pa *PrivAccount) SignBytes(msg []byte) Signature {
	signature := crypto.SignMessage(msg, pa.PrivKey, pa.PubKey)
	sig := Signature{
		SignerId: pa.Id,
		Bytes:    signature,
	}
	return sig
}

func (pa *PrivAccount) Sign(o Signable) {
	o.SetSignature(Signature{}) // clear
	msg := BinaryBytes(o)
	sig := pa.SignBytes(msg)
	o.SetSignature(sig)
}
