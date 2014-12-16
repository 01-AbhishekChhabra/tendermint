package account

import (
	"errors"
	"io"
	"reflect"

	"github.com/tendermint/go-ed25519"
	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
)

type Signature interface {
}

const (
	SignatureTypeEd25519 = byte(0x01)
)

func SignatureDecoder(r io.Reader, n *int64, err *error) interface{} {
	switch t := ReadByte(r, n, err); t {
	case SignatureTypeEd25519:
		return ReadBinary(&SignatureEd25519{}, r, n, err)
	default:
		*err = Errorf("Unknown Signature type %X", t)
		return nil
	}
}

var _ = RegisterType(&TypeInfo{
	Type:    reflect.TypeOf((*Signature)(nil)).Elem(),
	Decoder: SignatureDecoder,
})

//-------------------------------------

type SignatureEd25519 struct {
	Bytes []byte
}

func (sig SignatureEd25519) TypeByte() byte { return SignatureTypeEd25519 }

func (sig SignatureEd25519) ValidateBasic() error {
	if len(sig.Bytes) != ed25519.SignatureSize {
		return errors.New("Invalid SignatureEd25519 signature size")
	}
	return nil
}

func (sig SignatureEd25519) IsZero() bool {
	return len(sig.Bytes) == 0
}
