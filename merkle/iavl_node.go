package merkle

import (
	"bytes"
	"crypto/sha256"
	. "github.com/tendermint/tendermint/binary"
	"io"
)

// Node

type IAVLNode struct {
	key       []byte
	value     []byte
	size      uint64
	height    uint8
	hash      []byte
	leftHash  []byte
	rightHash []byte
	persisted bool

	// May or may not be persisted nodes, but they'll get cleared
	// when this this node is saved.
	leftCached  *IAVLNode
	rightCached *IAVLNode
}

func NewIAVLNode(key []byte, value []byte) *IAVLNode {
	return &IAVLNode{
		key:       key,
		value:     value,
		size:      1,
		persisted: false,
	}
}

func ReadIAVLNode(r io.Reader, n *int64, err *error) *IAVLNode {
	node := &IAVLNode{}

	// node header & key
	node.height = ReadUInt8(r, &n, &err)
	node.size = ReadUInt64(r, &n, &err)
	node.key = ReadByteSlice(r, &n, &err)
	if err != nil {
		panic(err)
	}

	// node value or children.
	if node.height == 0 {
		// value
		node.value = ReadByteSlice(r, &n, &err)
	} else {
		// left
		node.leftHash = ReadByteSlice(r, &n, &err)
		// right
		node.rightHash = ReadByteSlice(r, &n, &err)
	}
	if err != nil {
		panic(err)
	}
	return node
}

func (self *IAVLNode) Copy() *IAVLNode {
	if self.height == 0 {
		panic("Why are you copying a value node?")
	}
	return &IAVLNode{
		key:         self.key,
		size:        self.size,
		height:      self.height,
		hash:        nil, // Going to be mutated anyways.
		leftHash:    self.leftHash,
		rightHash:   self.rightHash,
		persisted:   self.persisted,
		leftCached:  self.leftCached,
		rightCached: self.rightCached,
	}
}

func (self *IAVLNode) Size() uint64 {
	return self.size
}

func (self *IAVLNode) Height() uint8 {
	return self.height
}

func (self *IAVLNode) has(ndb *IAVLNodeDB, key []byte) (has bool) {
	if bytes.Equal(self.key, key) {
		return true
	}
	if self.height == 0 {
		return false
	} else {
		if bytes.Compare(key, self.key) == -1 {
			return self.getLeft(ndb).has(ndb, key)
		} else {
			return self.getRight(ndb).has(ndb, key)
		}
	}
}

func (self *IAVLNode) get(ndb *IAVLNodeDB, key []byte) (value []byte) {
	if self.height == 0 {
		if bytes.Equal(self.key, key) {
			return self.value
		} else {
			return nil
		}
	} else {
		if bytes.Compare(key, self.key) == -1 {
			return self.getLeft(ndb).get(ndb, key)
		} else {
			return self.getRight(ndb).get(ndb, key)
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

func (self *IAVLNode) Save(ndb *IAVLNodeDB) []byte {
	if self.hash == nil {
		hash, _ := self.HashWithCount()
		self.hash = hash
	}
	if self.persisted {
		return self.hash
	}

	// children
	if self.leftCached != nil {
		self.leftHash = self.leftCached.Save(ndb)
		self.leftCached = nil
	}
	if self.rightCached != nil {
		self.rightHash = self.rightCached.Save(ndb)
		self.rightCached = nil
	}

	// save self
	ndb.Save(self)
	return self.hash
}

func (self *IAVLNode) set(ndb *IAVLNodeDB, key []byte, value []byte) (_ *IAVLNode, updated bool) {
	if self.height == 0 {
		if bytes.Compare(key, self.key) == -1 {
			return &IAVLNode{
				key:         self.key,
				height:      1,
				size:        2,
				leftCached:  NewIAVLNode(key, value),
				rightCached: self,
			}, false
		} else if bytes.Equal(self.key, key) {
			return NewIAVLNode(key, value), true
		} else {
			return &IAVLNode{
				key:         key,
				height:      1,
				size:        2,
				leftCached:  self,
				rightCached: NewIAVLNode(key, value),
			}, false
		}
	} else {
		self = self.Copy()
		if bytes.Compare(key, self.key) == -1 {
			self.leftCached, updated = self.getLeft(ndb).set(ndb, key, value)
			self.leftHash = nil
		} else {
			self.rightCached, updated = self.getRight(ndb).set(ndb, key, value)
			self.rightHash = nil
		}
		if updated {
			return self, updated
		} else {
			self.calcHeightAndSize(ndb)
			return self.balance(ndb), updated
		}
	}
}

// newKey: new leftmost leaf key for tree after successfully removing 'key' if changed.
// only one of newSelfHash or newSelf is returned.
func (self *IAVLNode) remove(ndb *IAVLNodeDB, key []byte) (newSelfHash []byte, newSelf *IAVLNode, newKey []byte, value []byte, err error) {
	if self.height == 0 {
		if bytes.Equal(self.key, key) {
			return nil, nil, nil, self.value, nil
		} else {
			return nil, self, nil, nil, NotFound(key)
		}
	} else {
		if bytes.Compare(key, self.key) == -1 {
			var newLeftHash []byte
			var newLeft *IAVLNode
			newLeftHash, newLeft, newKey, value, err = self.getLeft(ndb).remove(ndb, key)
			if err != nil {
				return nil, self, nil, value, err
			} else if newLeftHash == nil && newLeft == nil { // left node held value, was removed
				return self.rightHash, self.rightCached, self.key, value, nil
			}
			self = self.Copy()
			self.leftHash, self.leftCached = newLeftHash, newLeft
		} else {
			var newRightHash []byte
			var newRight *IAVLNode
			newRightHash, newRight, newKey, value, err = self.getRight(ndb).remove(ndb, key)
			if err != nil {
				return nil, self, nil, value, err
			} else if newRightHash == nil && newRight == nil { // right node held value, was removed
				return self.leftHash, self.leftCached, nil, value, nil
			}
			self = self.Copy()
			self.rightHash, self.rightCached = newRightHash, newRight
			if newKey != nil {
				self.key = newKey
				newKey = nil
			}
		}
		self.calcHeightAndSize(ndb)
		return nil, self.balance(ndb), newKey, value, err
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
		if self.leftCached != nil {
			leftHash, leftCount := self.left.HashWithCount()
			self.leftHash = leftHash
			hashCount += leftCount
		}
		WriteByteSlice(w, self.leftHash, &n, &err)
		// right
		if self.rightCached != nil {
			rightHash, rightCount := self.right.HashWithCount()
			self.rightHash = rightHash
			hashCount += rightCount
		}
		WriteByteSlice(w, self.rightHash, &n, &err)
	}
	return
}

func (self *IAVLNode) getLeft(ndb *IAVLNodeDB) *IAVLNode {
	if self.leftCached != nil {
		return self.leftCached
	} else {
		return ndb.Get(leftHash)
	}
}

func (self *IAVLNode) getRight(ndb *IAVLNodeDB) *IAVLNode {
	if self.rightCached != nil {
		return self.rightCached
	} else {
		return ndb.Get(rightHash)
	}
}

func (self *IAVLNode) rotateRight(ndb *IAVLNodeDB) *IAVLNode {
	self = self.Copy()
	sl := self.getLeft(ndb).Copy()

	slrHash, slrCached := sl.rightHash, sl.rightCached
	sl.rightHash, sl.rightCached = nil, self
	self.leftHash, self.leftCached = slrHash, slrCached

	self.calcHeightAndSize(ndb)
	sl.calcHeightAndSize(ndb)

	return sl
}

func (self *IAVLNode) rotateLeft(ndb *IAVLNodeDB) *IAVLNode {
	self = self.Copy()
	sr := self.getRight(ndb).Copy()

	srlHash, srlCached := sr.leftHash, sr.leftCached
	sr.leftHash, sr.leftCached = nil, self
	self.rightHash, self.rightCached = srlHash, srlCached

	self.calcHeightAndSize(ndb)
	sr.calcHeightAndSize(ndb)

	return sr
}

func (self *IAVLNode) calcHeightAndSize(ndb *IAVLNodeDB) {
	self.height = maxUint8(self.getLeft(ndb).Height(), self.getRight(ndb).Height()) + 1
	self.size = self.getLeft(ndb).Size() + self.getRight(ndb).Size()
}

func (self *IAVLNode) calcBalance(ndb *IAVLNodeDB) int {
	return int(self.getLeft(ndb).Height()) - int(self.getRight(ndb).Height())
}

func (self *IAVLNode) balance(ndb *IAVLNodeDB) (newSelf *IAVLNode) {
	balance := self.calcBalance(ndb)
	if balance > 1 {
		if self.getLeft(ndb).calcBalance(ndb) >= 0 {
			// Left Left Case
			return self.rotateRight(ndb)
		} else {
			// Left Right Case
			self = self.Copy()
			self.leftHash, self.leftCached = nil, self.getLeft(ndb).rotateLeft(ndb)
			//self.calcHeightAndSize()
			return self.rotateRight(ndb)
		}
	}
	if balance < -1 {
		if self.getRight(ndb).calcBalance(ndb) <= 0 {
			// Right Right Case
			return self.rotateLeft(ndb)
		} else {
			// Right Left Case
			self = self.Copy()
			self.rightHash, self.rightCached = nil, self.getRight(ndb).rotateRight(ndb)
			//self.calcHeightAndSize()
			return self.rotateLeft(ndb)
		}
	}
	// Nothing changed
	return self
}

// Only used in testing...
func (self *IAVLNode) lmd(ndb *IAVLNodeDB) *IAVLNode {
	if self.height == 0 {
		return self
	}
	return self.getLeft(ndb).lmd(ndb)
}

// Only used in testing...
func (self *IAVLNode) rmd(ndb *IAVLNodeDB) *IAVLNode {
	if self.height == 0 {
		return self
	}
	return self.getRight(ndb).rmd(ndb)
}

func (self *IAVLNode) traverse(ndb *IAVLNodeDB, cb func(*IAVLNode) bool) bool {
	stop := cb(self)
	if stop {
		return stop
	}
	if self.height > 0 {
		stop = self.getLeft(ndb).traverse(ndb, cb)
		if stop {
			return stop
		}
		stop = self.getRight(ndb).traverse(ndb, cb)
		if stop {
			return stop
		}
	}
	return false
}
