package merkle

import (
    "fmt"
)

type Binary interface {
    ByteSize()      int
    SaveTo([]byte)  int
}

type Value interface {
    Binary
}

type Key interface {
    Binary

    Equals(b Key)   bool
    Less(b Key)     bool
}

type Tree interface {
    Root()          Node

    Size()          uint64
    Height()        uint8
    Has(key Key)    bool
    Get(key Key)    Value
    Hash()          (ByteSlice, uint64)

    Put(Key, Value)
    Remove(Key)     (Value, error)
}

type Db interface {
    Get([]byte) []byte
    Put([]byte, []byte)
}

type Node interface {
    Binary

    Key()           Key
    Value()         Value
    Left(Db)        Node
    Right(Db)       Node

    Size()          uint64
    Height()        uint8
    Has(Db, Key)    bool
    Get(Db, Key)    Value
    Hash()          (ByteSlice, uint64)

    Put(Db, Key, Value) (*IAVLNode, bool)
    Remove(Db, Key) (*IAVLNode, Value, error)
}

type NodeIterator func() Node

func NotFound(key Key) error {
    return fmt.Errorf("Key was not found.")
}
