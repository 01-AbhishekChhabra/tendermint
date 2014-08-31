package blocks

import (
	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
	"io"
)

/* Adjustment

1. Bond         New validator posts a bond
2. Unbond       Validator leaves
3. Timeout      Validator times out
4. Dupeout      Validator dupes out (signs twice)

TODO: signing a bad checkpoint (block)
*/
type Adjustment interface {
	Type() Byte
	Binary
}

const (
	ADJ_TYPE_BOND    = Byte(0x01)
	ADJ_TYPE_UNBOND  = Byte(0x02)
	ADJ_TYPE_TIMEOUT = Byte(0x03)
	ADJ_TYPE_DUPEOUT = Byte(0x04)
)

func ReadAdjustment(r io.Reader) Adjustment {
	switch t := ReadByte(r); t {
	case ADJ_TYPE_BOND:
		return &Bond{
			Fee:       Readuint64(r),
			UnbondTo:  Readuint64(r),
			Amount:    Readuint64(r),
			Signature: ReadSignature(r),
		}
	case ADJ_TYPE_UNBOND:
		return &Unbond{
			Fee:       Readuint64(r),
			Amount:    Readuint64(r),
			Signature: ReadSignature(r),
		}
	case ADJ_TYPE_TIMEOUT:
		return &Timeout{
			AccountId: Readuint64(r),
			Penalty:   Readuint64(r),
		}
	case ADJ_TYPE_DUPEOUT:
		return &Dupeout{
			VoteA: ReadBlockVote(r),
			VoteB: ReadBlockVote(r),
		}
	default:
		Panicf("Unknown Adjustment type %x", t)
		return nil
	}
}

//-----------------------------------------------------------------------------

/* Bond < Adjustment */
type Bond struct {
	Fee      uint64
	UnbondTo uint64
	Amount   uint64
	Signature
}

func (self *Bond) Type() Byte {
	return ADJ_TYPE_BOND
}

func (self *Bond) WriteTo(w io.Writer) (n int64, err error) {
	n, err = WriteTo(self.Type(), w, n, err)
	n, err = WriteTo(UInt64(self.Fee), w, n, err)
	n, err = WriteTo(UInt64(self.UnbondTo), w, n, err)
	n, err = WriteTo(UInt64(self.Amount), w, n, err)
	n, err = WriteTo(self.Signature, w, n, err)
	return
}

//-----------------------------------------------------------------------------

/* Unbond < Adjustment */
type Unbond struct {
	Fee    uint64
	Amount uint64
	Signature
}

func (self *Unbond) Type() Byte {
	return ADJ_TYPE_UNBOND
}

func (self *Unbond) WriteTo(w io.Writer) (n int64, err error) {
	n, err = WriteTo(self.Type(), w, n, err)
	n, err = WriteTo(UInt64(self.Fee), w, n, err)
	n, err = WriteTo(UInt64(self.Amount), w, n, err)
	n, err = WriteTo(self.Signature, w, n, err)
	return
}

//-----------------------------------------------------------------------------

/* Timeout < Adjustment */
type Timeout struct {
	AccountId uint64
	Penalty   uint64
}

func (self *Timeout) Type() Byte {
	return ADJ_TYPE_TIMEOUT
}

func (self *Timeout) WriteTo(w io.Writer) (n int64, err error) {
	n, err = WriteTo(self.Type(), w, n, err)
	n, err = WriteTo(UInt64(self.AccountId), w, n, err)
	n, err = WriteTo(UInt64(self.Penalty), w, n, err)
	return
}

//-----------------------------------------------------------------------------

/*
The full vote structure is only needed when presented as evidence.
Typically only the signature is passed around, as the hash & height are implied.
*/
type BlockVote struct {
	Height    uint64
	BlockHash ByteSlice
	Signature
}

func ReadBlockVote(r io.Reader) BlockVote {
	return BlockVote{
		Height:    Readuint64(r),
		BlockHash: ReadByteSlice(r),
		Signature: ReadSignature(r),
	}
}

func (self BlockVote) WriteTo(w io.Writer) (n int64, err error) {
	n, err = WriteTo(UInt64(self.Height), w, n, err)
	n, err = WriteTo(self.BlockHash, w, n, err)
	n, err = WriteTo(self.Signature, w, n, err)
	return
}

/* Dupeout < Adjustment */
type Dupeout struct {
	VoteA BlockVote
	VoteB BlockVote
}

func (self *Dupeout) Type() Byte {
	return ADJ_TYPE_DUPEOUT
}

func (self *Dupeout) WriteTo(w io.Writer) (n int64, err error) {
	n, err = WriteTo(self.Type(), w, n, err)
	n, err = WriteTo(self.VoteA, w, n, err)
	n, err = WriteTo(self.VoteB, w, n, err)
	return
}
