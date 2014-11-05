package p2p

import (
	"errors"
	"net"
	"sync/atomic"
	"time"

	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
)

type Reactor interface {
	Start(sw *Switch)
	Stop()
	GetChannels() []*ChannelDescriptor
	AddPeer(peer *Peer)
	RemovePeer(peer *Peer, reason interface{})
	Receive(chId byte, peer *Peer, msgBytes []byte)
}

//-----------------------------------------------------------------------------

/*
All communication amongst peers are multiplexed by "channels".
(Not the same as Go "channels")

To send a message, serialize it into a ByteSlice and send it to each peer.
For best performance, re-use the same immutable ByteSlice to each peer.
You can also use a TypedBytes{} struct for convenience.
You can find all connected and active peers by iterating over ".Peers().List()".
".Broadcast()" is provided for convenience, but by iterating over
the peers manually the caller can decide which subset receives a message.

Inbound messages are received by calling ".Receive()".
The receiver is responsible for decoding the message bytes, which may be preceded
by a single type byte if a TypedBytes{} was used.
*/
type Switch struct {
	reactors     []Reactor
	chDescs      []*ChannelDescriptor
	reactorsByCh map[byte]Reactor
	peers        *PeerSet
	dialing      *CMap
	listeners    *CMap // listenerName -> chan interface{}
	quit         chan struct{}
	started      uint32
	stopped      uint32
}

var (
	ErrSwitchStopped       = errors.New("Switch already stopped")
	ErrSwitchDuplicatePeer = errors.New("Duplicate peer")
)

const (
	peerDialTimeoutSeconds = 30
)

func NewSwitch(reactors []Reactor) *Switch {

	// Validate the reactors. no two reactors can share the same channel.
	chDescs := []*ChannelDescriptor{}
	reactorsByCh := make(map[byte]Reactor)
	for _, reactor := range reactors {
		reactorChannels := reactor.GetChannels()
		for _, chDesc := range reactorChannels {
			chId := chDesc.Id
			if reactorsByCh[chId] != nil {
				Panicf("Channel %X has multiple reactors %v & %v", chId, reactorsByCh[chId], reactor)
			}
			chDescs = append(chDescs, chDesc)
			reactorsByCh[chId] = reactor
		}
	}

	sw := &Switch{
		reactors:     reactors,
		chDescs:      chDescs,
		reactorsByCh: reactorsByCh,
		peers:        NewPeerSet(),
		dialing:      NewCMap(),
		listeners:    NewCMap(),
		quit:         make(chan struct{}),
		stopped:      0,
	}

	return sw
}

func (sw *Switch) Start() {
	if atomic.CompareAndSwapUint32(&sw.started, 0, 1) {
		log.Info("Starting switch")
		for _, reactor := range sw.reactors {
			reactor.Start(sw)
		}
	}
}

func (sw *Switch) Stop() {
	if atomic.CompareAndSwapUint32(&sw.stopped, 0, 1) {
		log.Info("Stopping switch")
		close(sw.quit)
		// Stop each peer.
		for _, peer := range sw.peers.List() {
			peer.stop()
		}
		sw.peers = NewPeerSet()
		// Stop all reactors.
		for _, reactor := range sw.reactors {
			reactor.Stop()
		}
	}
}

func (sw *Switch) Reactors() []Reactor {
	return sw.reactors
}

func (sw *Switch) AddPeerWithConnection(conn net.Conn, outbound bool) (*Peer, error) {
	if atomic.LoadUint32(&sw.stopped) == 1 {
		return nil, ErrSwitchStopped
	}

	peer := newPeer(conn, outbound, sw.reactorsByCh, sw.chDescs, sw.StopPeerForError)

	// Add the peer to .peers
	if sw.peers.Add(peer) {
		log.Info("+ %v", peer)
	} else {
		log.Info("Ignoring duplicate: %v", peer)
		return nil, ErrSwitchDuplicatePeer
	}

	// Start the peer
	go peer.start()

	// Notify listeners.
	sw.doAddPeer(peer)

	return peer, nil
}

func (sw *Switch) DialPeerWithAddress(addr *NetAddress) (*Peer, error) {
	if atomic.LoadUint32(&sw.stopped) == 1 {
		return nil, ErrSwitchStopped
	}

	log.Info("Dialing peer @ %v", addr)
	sw.dialing.Set(addr.String(), addr)
	conn, err := addr.DialTimeout(peerDialTimeoutSeconds * time.Second)
	sw.dialing.Delete(addr.String())
	if err != nil {
		return nil, err
	}
	peer, err := sw.AddPeerWithConnection(conn, true)
	if err != nil {
		return nil, err
	}
	return peer, nil
}

func (sw *Switch) IsDialing(addr *NetAddress) bool {
	return sw.dialing.Has(addr.String())
}

// XXX: This is wrong, we can't just ignore failures on TrySend.
func (sw *Switch) Broadcast(chId byte, msg Binary) (numSuccess, numFailure int) {
	if atomic.LoadUint32(&sw.stopped) == 1 {
		return
	}

	log.Debug("[%X] Broadcast: %v", chId, msg)
	for _, peer := range sw.peers.List() {
		// XXX XXX Change.
		// success := peer.TrySend(chId, msg)
		success := peer.Send(chId, msg)
		log.Debug("[%X] for peer %v success: %v", chId, peer, success)
		if success {
			numSuccess += 1
		} else {
			numFailure += 1
		}
	}
	return

}

// Returns the count of outbound/inbound and outbound-dialing peers.
func (sw *Switch) NumPeers() (outbound, inbound, dialing int) {
	peers := sw.peers.List()
	for _, peer := range peers {
		if peer.outbound {
			outbound++
		} else {
			inbound++
		}
	}
	dialing = sw.dialing.Size()
	return
}

func (sw *Switch) Peers() IPeerSet {
	return sw.peers
}

// Disconnect from a peer due to external error.
// TODO: make record depending on reason.
func (sw *Switch) StopPeerForError(peer *Peer, reason interface{}) {
	log.Info("- %v !! reason: %v", peer, reason)
	sw.peers.Remove(peer)
	peer.stop()

	// Notify listeners
	sw.doRemovePeer(peer, reason)
}

// Disconnect from a peer gracefully.
// TODO: handle graceful disconnects.
func (sw *Switch) StopPeerGracefully(peer *Peer) {
	log.Info("- %v", peer)
	sw.peers.Remove(peer)
	peer.stop()

	// Notify listeners
	sw.doRemovePeer(peer, nil)
}

func (sw *Switch) doAddPeer(peer *Peer) {
	for _, reactor := range sw.reactors {
		reactor.AddPeer(peer)
	}
}

func (sw *Switch) doRemovePeer(peer *Peer, reason interface{}) {
	for _, reactor := range sw.reactors {
		reactor.RemovePeer(peer, reason)
	}
}

//-----------------------------------------------------------------------------

type SwitchEventNewPeer struct {
	Peer *Peer
}

type SwitchEventDonePeer struct {
	Peer  *Peer
	Error interface{}
}
