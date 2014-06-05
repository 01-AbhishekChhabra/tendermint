package blocks

import (
    . "github.com/tendermint/tendermint/binary"
    "io"
)

/*

Signature message wire format:

    |A...|SSS...|

    A  account number, varint encoded (1+ bytes)
    S  signature of all prior bytes (32 bytes)

It usually follows the message to be signed.

*/

type Signature struct {
    Signer          AccountId
    SigBytes        ByteSlice
}

func ReadSignature(r io.Reader) *Signature {
    return nil
}

func (self *Signature) Equals(other Binary) bool {
    if o, ok := other.(*Signature); ok {
        return self.Signer.Equals(o.Signer) &&
               self.SigBytes.Equals(o.SigBytes)
    } else {
        return false
    }
}

func (self *Signature) WriteTo(w io.Writer) (n int64, err error) {
    var n_ int64
    n_, err = self.Signer.WriteTo(w)
    n += n_; if err != nil { return n, err }
    n_, err = self.SigBytes.WriteTo(w)
    n += n_; return
}

func (self *Signature) Verify(msg ByteSlice) bool {
    return false
}
