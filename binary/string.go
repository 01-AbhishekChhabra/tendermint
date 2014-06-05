package binary

import "io"
import "bytes"

type String string
type ByteSlice []byte

// String

func (self String) Equals(other Binary) bool {
    return self == other
}

func (self String) Less(other Binary) bool {
    if o, ok := other.(String); ok {
        return self < o
    } else {
        panic("Cannot compare unequal types")
    }
}

func (self String) ByteSize() int {
    return len(self)+4
}

func (self String) WriteTo(w io.Writer) (n int64, err error) {
    var n_ int
    _, err = UInt32(len(self)).WriteTo(w)
    if err != nil { return n, err }
    n_, err = w.Write([]byte(self))
    return int64(n_+4), err
}

func ReadString(r io.Reader) String {
    length := int(ReadUInt32(r))
    bytes := make([]byte, length)
    _, err := io.ReadFull(r, bytes)
    if err != nil { panic(err) }
    return String(bytes)
}


// ByteSlice

func (self ByteSlice) Equals(other Binary) bool {
    if o, ok := other.(ByteSlice); ok {
        return bytes.Equal(self, o)
    } else {
        return false
    }
}

func (self ByteSlice) Less(other Binary) bool {
    if o, ok := other.(ByteSlice); ok {
        return bytes.Compare(self, o) < 0 // -1 if a < b
    } else {
        panic("Cannot compare unequal types")
    }
}

func (self ByteSlice) ByteSize() int {
    return len(self)+4
}

func (self ByteSlice) WriteTo(w io.Writer) (n int64, err error) {
    var n_ int
    _, err = UInt32(len(self)).WriteTo(w)
    if err != nil { return n, err }
    n_, err = w.Write([]byte(self))
    return int64(n_+4), err
}

func ReadByteSlice(r io.Reader) ByteSlice {
    length := int(ReadUInt32(r))
    bytes := make([]byte, length)
    _, err := io.ReadFull(r, bytes)
    if err != nil { panic(err) }
    return ByteSlice(bytes)
}
