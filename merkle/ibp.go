package merkle

import (
    "crypto/sha256"
)

// Immutable B+ Tree (wraps the Node root)

type IBPTree struct {
    db      Db
    root    *IBPNode
}

func NewIBPTree(db Db) *IBPTree {
    return &IBPTree{db:db, root:nil}
}

func NewIBPTreeFromHash(db Db, hash ByteSlice) *IBPTree {
    root := &IBPNode{
        hash:   hash,
        flags:  IBPNODE_FLAG_PERSISTED | IBPNODE_FLAG_PLACEHOLDER,
    }
    root.fill(db)
    return &IBPTree{db:db, root:root}
}

func (self *IBPTree) Root() Node {
    return self.root
}

func (self *IBPTree) Size() uint64 {
    return self.root.Size()
}

func (self *IBPTree) Height() uint8 {
    return self.root.Height()
}

func (self *IBPTree) Has(key Key) bool {
    return self.root.has(self.db, key)
}

func (self *IBPTree) Put(key Key, value Value) (updated bool) {
    self.root, updated = self.root.put(self.db, key, value)
    return updated
}

func (self *IBPTree) Hash() (ByteSlice, uint64) {
    return self.root.Hash()
}

func (self *IBPTree) Save() {
    if self.root.hash == nil {
        self.root.Hash()
    }
    self.root.Save(self.db)
}

func (self *IBPTree) Get(key Key) (value Value) {
    return self.root.get(self.db, key)
}

func (self *IBPTree) Remove(key Key) (value Value, err error) {
    newRoot, value, err := self.root.remove(self.db, key)
    if err != nil {
        return nil, err
    }
    self.root = newRoot
    return value, nil
}

func (self *IBPTree) Iterator() NodeIterator {
    /*
    pop := func(stack []*IBPNode) ([]*IBPNode, *IBPNode) {
        if len(stack) <= 0 {
            return stack, nil
        } else {
            return stack[0:len(stack)-1], stack[len(stack)-1]
        }
    }

    stack := make([]*IBPNode, 0, 10)
    var cur *IBPNode = self.root
    var itr NodeIterator
    itr = func()(tn Node) {
        if len(stack) > 0 || cur != nil {
            for cur != nil {
                stack = append(stack, cur)
                cur = cur.leftFilled(self.db)
            }
            stack, cur = pop(stack)
            tn = cur
            cur = cur.rightFilled(self.db)
            return tn
        } else {
            return nil
        }
    }
    return itr
    */
    return nil
}


// Node

type IBPNode struct {
    key     Key
    value   Value
    size    uint64
    height  uint8
    hash    ByteSlice
    left    *IBPNode
    right   *IBPNode

    // volatile
    flags   byte
}

const (
    IBPNODE_FLAG_PERSISTED =   byte(0x01)
    IBPNODE_FLAG_PLACEHOLDER = byte(0x02)

    IBPNODE_DESC_HAS_VALUE =   byte(0x01)
    IBPNODE_DESC_HAS_LEFT =    byte(0x02)
    IBPNODE_DESC_HAS_RIGHT =   byte(0x04)
)

func (self *IBPNode) Copy() *IBPNode {
    if self == nil {
        return nil
    }
    return &IBPNode{
        key:    self.key,
        value:  self.value,
        size:   self.size,
        height: self.height,
        left:   self.left,
        right:  self.right,
        hash:   nil,
        flags:  byte(0),
    }
}

func (self *IBPNode) Equals(other Binary) bool {
    if o, ok := other.(*IBPNode); ok {
        return self.hash.Equals(o.hash)
    } else {
        return false
    }
}

func (self *IBPNode) Key() Key {
    return self.key
}

func (self *IBPNode) Value() Value {
    return self.value
}

func (self *IBPNode) Size() uint64 {
    if self == nil { return 0 }
    return self.size
}

func (self *IBPNode) Height() uint8 {
    if self == nil { return 0 }
    return self.height
}

func (self *IBPNode) has(db Db, key Key) (has bool) {
    if self == nil { return false }
    if self.key.Equals(key) {
        return true
    } else if key.Less(self.key) {
        return self.leftFilled(db).has(db, key)
    } else {
        return self.rightFilled(db).has(db, key)
    }
}

func (self *IBPNode) get(db Db, key Key) (value Value) {
    if self == nil { return nil }
    if self.key.Equals(key) {
        return self.value
    } else if key.Less(self.key) {
        return self.leftFilled(db).get(db, key)
    } else {
        return self.rightFilled(db).get(db, key)
    }
}

func (self *IBPNode) Hash() (ByteSlice, uint64) {
    if self == nil { return nil, 0 }
    if self.hash != nil {
        return self.hash, 0
    }

    size := self.ByteSize()
    buf := make([]byte, size, size)
    hasher := sha256.New()
    _, hashCount := self.saveToCountHashes(buf)
    hasher.Write(buf)
    self.hash = hasher.Sum(nil)

    return self.hash, hashCount+1
}

func (self *IBPNode) Save(db Db) {
    if self == nil {
        return
    } else if self.hash == nil {
        panic("savee.hash can't be nil")
    }
    if self.flags & IBPNODE_FLAG_PERSISTED > 0 ||
       self.flags & IBPNODE_FLAG_PLACEHOLDER > 0 {
        return
    }

    // save self
    buf := make([]byte, self.ByteSize(), self.ByteSize())
    self.SaveTo(buf)
    db.Put([]byte(self.hash), buf)

    // save left
    self.left.Save(db)

    // save right
    self.right.Save(db)

    self.flags |= IBPNODE_FLAG_PERSISTED
}

func (self *IBPNode) put(db Db, key Key, value Value) (_ *IBPNode, updated bool) {
    if self == nil {
        return &IBPNode{key: key, value: value, height: 1, size: 1, hash: nil}, false
    }

    self = self.Copy()

    if self.key.Equals(key) {
        self.value = value
        return self, true
    }

    if key.Less(self.key) {
        self.left, updated = self.leftFilled(db).put(db, key, value)
    } else {
        self.right, updated = self.rightFilled(db).put(db, key, value)
    }
    if updated {
        return self, updated
    } else {
        self.calcHeightAndSize(db)
        return self.balance(db), updated
    }
}

func (self *IBPNode) remove(db Db, key Key) (newSelf *IBPNode, value Value, err error) {
    if self == nil { return nil, nil, NotFound(key) }

    if self.key.Equals(key) {
        if self.left != nil && self.right != nil {
            if self.leftFilled(db).Size() < self.rightFilled(db).Size() {
                self, newSelf = self.popNode(db, self.rightFilled(db).lmd(db))
            } else {
                self, newSelf = self.popNode(db, self.leftFilled(db).rmd(db))
            }
            newSelf.left = self.left
            newSelf.right = self.right
            newSelf.calcHeightAndSize(db)
            return newSelf, self.value, nil
        } else if self.left == nil {
            return self.rightFilled(db), self.value, nil
        } else if self.right == nil {
            return self.leftFilled(db), self.value, nil
        } else {
            return nil, self.value, nil
        }
    }

    if key.Less(self.key) {
        if self.left == nil {
            return self, nil, NotFound(key)
        }
        var newLeft *IBPNode
        newLeft, value, err = self.leftFilled(db).remove(db, key)
        if newLeft == self.leftFilled(db) { // not found
            return self, nil, err
        } else if err != nil { // some other error
            return self, value, err
        }
        self = self.Copy()
        self.left = newLeft
    } else {
        if self.right == nil {
            return self, nil, NotFound(key)
        }
        var newRight *IBPNode
        newRight, value, err = self.rightFilled(db).remove(db, key)
        if newRight == self.rightFilled(db) { // not found
            return self, nil, err
        } else if err != nil { // some other error
            return self, value, err
        }
        self = self.Copy()
        self.right = newRight
    }
    self.calcHeightAndSize(db)
    return self.balance(db), value, err
}

func (self *IBPNode) ByteSize() int {
    // 1 byte node descriptor
    // 1 byte node neight
    // 8 bytes node size
    size := 10
    // key
    size += 1 // type info
    size += self.key.ByteSize()
    // value
    if self.value != nil {
        size += 1 // type info
        size += self.value.ByteSize()
    } else {
        size += 1
    }
    // children
    if self.left != nil {
        size += HASH_BYTE_SIZE
    }
    if self.right != nil {
        size += HASH_BYTE_SIZE
    }
    return size
}

func (self *IBPNode) SaveTo(buf []byte) int {
    written, _ := self.saveToCountHashes(buf)
    return written
}

func (self *IBPNode) saveToCountHashes(buf []byte) (int, uint64) {
    cur := 0
    hashCount := uint64(0)

    // node descriptor
    nodeDesc := byte(0)
    if self.value != nil { nodeDesc |= IBPNODE_DESC_HAS_VALUE }
    if self.left != nil  { nodeDesc |= IBPNODE_DESC_HAS_LEFT }
    if self.right != nil { nodeDesc |= IBPNODE_DESC_HAS_RIGHT }
    cur += UInt8(nodeDesc).SaveTo(buf[cur:])

    // node height & size
    cur += UInt8(self.height).SaveTo(buf[cur:])
    cur += UInt64(self.size).SaveTo(buf[cur:])

    // node key
    buf[cur] = GetBinaryType(self.key)
    cur += 1
    cur += self.key.SaveTo(buf[cur:])

    // node value
    if self.value != nil {
        buf[cur] = GetBinaryType(self.value)
        cur += 1
        cur += self.value.SaveTo(buf[cur:])
    }

    // left child
    if self.left != nil {
        leftHash, leftCount := self.left.Hash()
        hashCount += leftCount
        cur += leftHash.SaveTo(buf[cur:])
    }

    // right child
    if self.right != nil {
        rightHash, rightCount := self.right.Hash()
        hashCount += rightCount
        cur += rightHash.SaveTo(buf[cur:])
    }

    return cur, hashCount
}

// Given a placeholder node which has only the hash set,
// load the rest of the data from db.
// Not threadsafe.
func (self *IBPNode) fill(db Db) {
    if self == nil {
        panic("placeholder can't be nil")
    } else if self.hash == nil {
        panic("placeholder.hash can't be nil")
    }
    buf := db.Get(self.hash)
    cur := 0
    // node header
    nodeDesc := byte(LoadUInt8(buf))
    self.height = uint8(LoadUInt8(buf[1:]))
    self.size = uint64(LoadUInt64(buf[2:]))
    // key
    key, cur := LoadBinary(buf, 10)
    self.key = key.(Key)
    // value
    if nodeDesc & IBPNODE_DESC_HAS_VALUE > 0 {
        self.value, cur = LoadBinary(buf, cur)
    }
    // children
    if nodeDesc & IBPNODE_DESC_HAS_LEFT > 0 {
        var leftHash ByteSlice
        leftHash, cur = LoadByteSlice(buf, cur)
        self.left = &IBPNode{
            hash:   leftHash,
            flags:  IBPNODE_FLAG_PERSISTED | IBPNODE_FLAG_PLACEHOLDER,
        }
    }
    if nodeDesc & IBPNODE_DESC_HAS_RIGHT > 0 {
        var rightHash ByteSlice
        rightHash, cur = LoadByteSlice(buf, cur)
        self.right = &IBPNode{
            hash:   rightHash,
            flags:  IBPNODE_FLAG_PERSISTED | IBPNODE_FLAG_PLACEHOLDER,
        }
    }
    if cur != len(buf) {
        panic("buf not all consumed")
    }
    self.flags &= ^IBPNODE_FLAG_PLACEHOLDER
}

func (self *IBPNode) leftFilled(db Db) *IBPNode {
    if self.left == nil {
        return nil
    }
    if self.left.flags & IBPNODE_FLAG_PLACEHOLDER > 0 {
        self.left.fill(db)
    }
    return self.left
}

func (self *IBPNode) rightFilled(db Db) *IBPNode {
    if self.right == nil {
        return nil
    }
    if self.right.flags & IBPNODE_FLAG_PLACEHOLDER > 0 {
        self.right.fill(db)
    }
    return self.right
}

// Returns a new tree (unless node is the root) & a copy of the popped node.
// Can only pop nodes that have one or no children.
func (self *IBPNode) popNode(db Db, node *IBPNode) (newSelf, new_node *IBPNode) {
    if self == nil {
        panic("self can't be nil")
    } else if node == nil {
        panic("node can't be nil")
    } else if node.left != nil && node.right != nil {
        panic("node hnot have both left and right")
    }

    if self == node {

        var n *IBPNode
        if node.left != nil {
            n = node.leftFilled(db)
        } else if node.right != nil {
            n = node.rightFilled(db)
        } else {
            n = nil
        }
        node = node.Copy()
        node.left = nil
        node.right = nil
        node.calcHeightAndSize(db)
        return n, node

    } else {

        self = self.Copy()
        if node.key.Less(self.key) {
            self.left, node = self.leftFilled(db).popNode(db, node)
        } else {
            self.right, node = self.rightFilled(db).popNode(db, node)
        }
        self.calcHeightAndSize(db)
        return self, node

    }
}

func (self *IBPNode) rotateRight(db Db) *IBPNode {
    self = self.Copy()
    sl :=  self.leftFilled(db).Copy()
    slr := sl.right

    sl.right = self
    self.left = slr

    self.calcHeightAndSize(db)
    sl.calcHeightAndSize(db)

    return sl
}

func (self *IBPNode) rotateLeft(db Db) *IBPNode {
    self = self.Copy()
    sr :=  self.rightFilled(db).Copy()
    srl := sr.left

    sr.left = self
    self.right = srl

    self.calcHeightAndSize(db)
    sr.calcHeightAndSize(db)

    return sr
}

func (self *IBPNode) calcHeightAndSize(db Db) {
    self.height = maxUint8(self.leftFilled(db).Height(), self.rightFilled(db).Height()) + 1
    self.size = self.leftFilled(db).Size() + self.rightFilled(db).Size() + 1
}

func (self *IBPNode) calcBalance(db Db) int {
    if self == nil {
        return 0
    }
    return int(self.leftFilled(db).Height()) - int(self.rightFilled(db).Height())
}

func (self *IBPNode) balance(db Db) (newSelf *IBPNode) {
    balance := self.calcBalance(db)
    if (balance > 1) {
        if (self.leftFilled(db).calcBalance(db) >= 0) {
            // Left Left Case
            return self.rotateRight(db)
        } else {
            // Left Right Case
            self = self.Copy()
            self.left = self.leftFilled(db).rotateLeft(db)
            //self.calcHeightAndSize()
            return self.rotateRight(db)
        }
    }
    if (balance < -1) {
        if (self.rightFilled(db).calcBalance(db) <= 0) {
            // Right Right Case
            return self.rotateLeft(db)
        } else {
            // Right Left Case
            self = self.Copy()
            self.right = self.rightFilled(db).rotateRight(db)
            //self.calcHeightAndSize()
            return self.rotateLeft(db)
        }
    }
    // Nothing changed
    return self
}

func (self *IBPNode) _md(side func(*IBPNode)*IBPNode) (*IBPNode) {
    if self == nil {
        return nil
    } else if side(self) != nil {
        return side(self)._md(side)
    } else {
        return self
    }
}

func (self *IBPNode) lmd(db Db) (*IBPNode) {
    return self._md(func(node *IBPNode)*IBPNode { return node.leftFilled(db) })
}

func (self *IBPNode) rmd(db Db) (*IBPNode) {
    return self._md(func(node *IBPNode)*IBPNode { return node.rightFilled(db) })
}
