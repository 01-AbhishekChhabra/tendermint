package merkle

import (
	"bytes"
	"crypto/sha256"
	. "github.com/tendermint/tendermint/binary"
	"io"
)

// Node

type IAVLNode struct {
	key    []byte
	value  []byte
	size   uint64
	height uint8
	hash   []byte
	left   *IAVLNode
	right  *IAVLNode

	// volatile
	flags byte
}

const (
	IAVLNODE_FLAG_PERSISTED   = byte(0x01)
	IAVLNODE_FLAG_PLACEHOLDER = byte(0x02)
)

func NewIAVLNode(key []byte, value []byte) *IAVLNode {
	return &IAVLNode{
		key:   key,
		value: value,
		size:  1,
	}
}

func (self *IAVLNode) Copy() *IAVLNode {
	if self.height == 0 {
		panic("Why are you copying a value node?")
	}
	return &IAVLNode{
		key:    self.key,
		size:   self.size,
		height: self.height,
		left:   self.left,
		right:  self.right,
		hash:   nil,
		flags:  byte(0),
	}
}

func (self *IAVLNode) Size() uint64 {
	return self.size
}

func (self *IAVLNode) Height() uint8 {
	return self.height
}

func (self *IAVLNode) has(db Db, key []byte) (has bool) {
	if bytes.Equal(self.key, key) {
		return true
	}
	if self.height == 0 {
		return false
	} else {
		if bytes.Compare(key, self.key) == -1 {
			return self.leftFilled(db).has(db, key)
		} else {
			return self.rightFilled(db).has(db, key)
		}
	}
}

func (self *IAVLNode) get(db Db, key []byte) (value []byte) {
	if self.height == 0 {
		if bytes.Equal(self.key, key) {
			return self.value
		} else {
			return nil
		}
	} else {
		if bytes.Compare(key, self.key) == -1 {
			return self.leftFilled(db).get(db, key)
		} else {
			return self.rightFilled(db).get(db, key)
		}
	}
}

func (self *IAVLNode) HashWithCount() ([]byte, uint64) {
	if self.hash != nil {
		return self.hash, 0
	}

	hasher := sha256.New()
	_, hashCount, err := self.saveToCountHashes(hasher)
	if err != nil {
		panic(err)
	}
	self.hash = hasher.Sum(nil)

	return self.hash, hashCount + 1
}

func (self *IAVLNode) Save(db Db) {
	if self.hash == nil {
		panic("savee.hash can't be nil")
	}
	if self.flags&IAVLNODE_FLAG_PERSISTED > 0 ||
		self.flags&IAVLNODE_FLAG_PLACEHOLDER > 0 {
		return
	}

	// children
	if self.height > 0 {
		self.left.Save(db)
		self.right.Save(db)
	}

	// save self
	buf := bytes.NewBuffer(nil)
	_, err := self.WriteTo(buf)
	if err != nil {
		panic(err)
	}
	db.Set([]byte(self.hash), buf.Bytes())

	self.flags |= IAVLNODE_FLAG_PERSISTED
}

func (self *IAVLNode) set(db Db, key []byte, value []byte) (_ *IAVLNode, updated bool) {
	if self.height == 0 {
		if bytes.Compare(key, self.key) == -1 {
			return &IAVLNode{
				key:    self.key,
				height: 1,
				size:   2,
				left:   NewIAVLNode(key, value),
				right:  self,
			}, false
		} else if bytes.Equal(self.key, key) {
			return NewIAVLNode(key, value), true
		} else {
			return &IAVLNode{
				key:    key,
				height: 1,
				size:   2,
				left:   self,
				right:  NewIAVLNode(key, value),
			}, false
		}
	} else {
		self = self.Copy()
		if bytes.Compare(key, self.key) == -1 {
			self.left, updated = self.leftFilled(db).set(db, key, value)
		} else {
			self.right, updated = self.rightFilled(db).set(db, key, value)
		}
		if updated {
			return self, updated
		} else {
			self.calcHeightAndSize(db)
			return self.balance(db), updated
		}
	}
}

// newKey: new leftmost leaf key for tree after successfully removing 'key' if changed.
func (self *IAVLNode) remove(db Db, key []byte) (newSelf *IAVLNode, newKey []byte, value []byte, err error) {
	if self.height == 0 {
		if bytes.Equal(self.key, key) {
			return nil, nil, self.value, nil
		} else {
			return self, nil, nil, NotFound(key)
		}
	} else {
		if bytes.Compare(key, self.key) == -1 {
			var newLeft *IAVLNode
			newLeft, newKey, value, err = self.leftFilled(db).remove(db, key)
			if err != nil {
				return self, nil, value, err
			} else if newLeft == nil { // left node held value, was removed
				return self.right, self.key, value, nil
			}
			self = self.Copy()
			self.left = newLeft
		} else {
			var newRight *IAVLNode
			newRight, newKey, value, err = self.rightFilled(db).remove(db, key)
			if err != nil {
				return self, nil, value, err
			} else if newRight == nil { // right node held value, was removed
				return self.left, nil, value, nil
			}
			self = self.Copy()
			self.right = newRight
			if newKey != nil {
				self.key = newKey
				newKey = nil
			}
		}
		self.calcHeightAndSize(db)
		return self.balance(db), newKey, value, err
	}
}

func (self *IAVLNode) WriteTo(w io.Writer) (n int64, err error) {
	n, _, err = self.saveToCountHashes(w)
	return
}

func (self *IAVLNode) saveToCountHashes(w io.Writer) (n int64, hashCount uint64, err error) {
	// height & size & key
	WriteUInt8(w, self.height, &n, &err)
	WriteUInt64(w, self.size, &n, &err)
	WriteByteSlice(w, self.key, &n, &err)
	if err != nil {
		return
	}

	// value or children
	if self.height == 0 {
		// value
		WriteByteSlice(w, self.value, &n, &err)
	} else {
		// left
		leftHash, leftCount := self.left.HashWithCount()
		hashCount += leftCount
		WriteByteSlice(w, leftHash, &n, &err)
		// right
		rightHash, rightCount := self.right.HashWithCount()
		hashCount += rightCount
		WriteByteSlice(w, rightHash, &n, &err)
	}
	return
}

// Given a placeholder node which has only the hash set,
// load the rest of the data from db.
// Not threadsafe.
func (self *IAVLNode) fill(db Db) {
	if self.hash == nil {
		panic("placeholder.hash can't be nil")
	}
	buf := db.Get(self.hash)
	r := bytes.NewReader(buf)
	var n int64
	var err error

	// node header & key
	self.height = ReadUInt8(r, &n, &err)
	self.size = ReadUInt64(r, &n, &err)
	self.key = ReadByteSlice(r, &n, &err)
	if err != nil {
		panic(err)
	}

	// node value or children.
	if self.height == 0 {
		// value
		self.value = ReadByteSlice(r, &n, &err)
	} else {
		// left
		leftHash := ReadByteSlice(r, &n, &err)
		self.left = &IAVLNode{
			hash:  leftHash,
			flags: IAVLNODE_FLAG_PERSISTED | IAVLNODE_FLAG_PLACEHOLDER,
		}
		// right
		rightHash := ReadByteSlice(r, &n, &err)
		self.right = &IAVLNode{
			hash:  rightHash,
			flags: IAVLNODE_FLAG_PERSISTED | IAVLNODE_FLAG_PLACEHOLDER,
		}
		if r.Len() != 0 {
			panic("buf not all consumed")
		}
	}
	if err != nil {
		panic(err)
	}
	self.flags &= ^IAVLNODE_FLAG_PLACEHOLDER
}

func (self *IAVLNode) leftFilled(db Db) *IAVLNode {
	if self.left.flags&IAVLNODE_FLAG_PLACEHOLDER > 0 {
		self.left.fill(db)
	}
	return self.left
}

func (self *IAVLNode) rightFilled(db Db) *IAVLNode {
	if self.right.flags&IAVLNODE_FLAG_PLACEHOLDER > 0 {
		self.right.fill(db)
	}
	return self.right
}

func (self *IAVLNode) rotateRight(db Db) *IAVLNode {
	self = self.Copy()
	sl := self.leftFilled(db).Copy()
	slr := sl.right

	sl.right = self
	self.left = slr

	self.calcHeightAndSize(db)
	sl.calcHeightAndSize(db)

	return sl
}

func (self *IAVLNode) rotateLeft(db Db) *IAVLNode {
	self = self.Copy()
	sr := self.rightFilled(db).Copy()
	srl := sr.left

	sr.left = self
	self.right = srl

	self.calcHeightAndSize(db)
	sr.calcHeightAndSize(db)

	return sr
}

func (self *IAVLNode) calcHeightAndSize(db Db) {
	self.height = maxUint8(self.leftFilled(db).Height(), self.rightFilled(db).Height()) + 1
	self.size = self.leftFilled(db).Size() + self.rightFilled(db).Size()
}

func (self *IAVLNode) calcBalance(db Db) int {
	return int(self.leftFilled(db).Height()) - int(self.rightFilled(db).Height())
}

func (self *IAVLNode) balance(db Db) (newSelf *IAVLNode) {
	balance := self.calcBalance(db)
	if balance > 1 {
		if self.leftFilled(db).calcBalance(db) >= 0 {
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
	if balance < -1 {
		if self.rightFilled(db).calcBalance(db) <= 0 {
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

func (self *IAVLNode) lmd(db Db) *IAVLNode {
	if self.height == 0 {
		return self
	}
	return self.leftFilled(db).lmd(db)
}

func (self *IAVLNode) rmd(db Db) *IAVLNode {
	if self.height == 0 {
		return self
	}
	return self.rightFilled(db).rmd(db)
}

func (self *IAVLNode) traverse(db Db, cb func(*IAVLNode) bool) bool {
	stop := cb(self)
	if stop {
		return stop
	}
	if self.height > 0 {
		stop = self.leftFilled(db).traverse(db, cb)
		if stop {
			return stop
		}
		stop = self.rightFilled(db).traverse(db, cb)
		if stop {
			return stop
		}
	}
	return false
}
