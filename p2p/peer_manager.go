package p2p

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"sync/atomic"
	"time"

	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
)

var pexErrInvalidMessage = errors.New("Invalid PEX message")

const (
	PexCh                    = byte(0x00)
	ensurePeersPeriodSeconds = 30
	minNumOutboundPeers      = 10
	maxNumPeers              = 50
)

/*
PeerManager handles PEX (peer exchange) and ensures that an
adequate number of peers are connected to the switch.
*/
type PeerManager struct {
	sw       *Switch
	swEvents chan interface{}
	quit     chan struct{}
	started  uint32
	stopped  uint32

	book *AddrBook
}

func NewPeerManager(sw *Switch, book *AddrBook) *PeerManager {
	swEvents := make(chan interface{})
	sw.AddEventListener("PeerManager.swEvents", swEvents)
	pm := &PeerManager{
		sw:       sw,
		swEvents: swEvents,
		quit:     make(chan struct{}),
		book:     book,
	}
	return pm
}

func (pm *PeerManager) Start() {
	if atomic.CompareAndSwapUint32(&pm.started, 0, 1) {
		log.Info("Starting PeerManager")
		go pm.switchEventsRoutine()
		go pm.requestRoutine()
		go pm.ensurePeersRoutine()
	}
}

func (pm *PeerManager) Stop() {
	if atomic.CompareAndSwapUint32(&pm.stopped, 0, 1) {
		log.Info("Stopping PeerManager")
		close(pm.quit)
		close(pm.swEvents)
	}
}

// Asks peer for more addresses.
func (pm *PeerManager) RequestPEX(peer *Peer) {
	peer.TrySend(PexCh, &pexRequestMessage{})
}

func (pm *PeerManager) SendAddrs(peer *Peer, addrs []*NetAddress) {
	peer.Send(PexCh, &pexAddrsMessage{Addrs: addrs})
}

// For new outbound peers, announce our listener addresses if any,
// and if .book needs more addresses, ask for them.
func (pm *PeerManager) switchEventsRoutine() {
	for {
		swEvent, ok := <-pm.swEvents
		if !ok {
			break
		}
		switch swEvent.(type) {
		case SwitchEventNewPeer:
			event := swEvent.(SwitchEventNewPeer)
			if event.Peer.IsOutbound() {
				pm.SendAddrs(event.Peer, pm.book.OurAddresses())
				if pm.book.NeedMoreAddrs() {
					pm.RequestPEX(event.Peer)
				}
			}
		case SwitchEventDonePeer:
			// TODO
		}
	}
}

// Ensures that sufficient peers are connected. (continuous)
func (pm *PeerManager) ensurePeersRoutine() {
	// fire once immediately.
	pm.ensurePeers()
	// fire periodically
	timer := NewRepeatTimer(ensurePeersPeriodSeconds * time.Second)
FOR_LOOP:
	for {
		select {
		case <-timer.Ch:
			pm.ensurePeers()
		case <-pm.quit:
			break FOR_LOOP
		}
	}

	// Cleanup
	timer.Stop()
}

// Ensures that sufficient peers are connected. (once)
func (pm *PeerManager) ensurePeers() {
	numOutPeers, _, numDialing := pm.sw.NumPeers()
	numToDial := minNumOutboundPeers - (numOutPeers + numDialing)
	if numToDial <= 0 {
		return
	}
	toDial := NewCMap()

	// Try to pick numToDial addresses to dial.
	// TODO: improve logic.
	for i := 0; i < numToDial; i++ {
		newBias := MinInt(numOutPeers, 8)*10 + 10
		var picked *NetAddress
		// Try to fetch a new peer 3 times.
		// This caps the maximum number of tries to 3 * numToDial.
		for j := 0; i < 3; j++ {
			picked = pm.book.PickAddress(newBias)
			if picked == nil {
				return
			}
			if toDial.Has(picked.String()) ||
				pm.sw.IsDialing(picked) ||
				pm.sw.Peers().Has(picked.String()) {
				continue
			} else {
				break
			}
		}
		if picked == nil {
			continue
		}
		toDial.Set(picked.String(), picked)
	}

	// Dial picked addresses
	for _, item := range toDial.Values() {
		picked := item.(*NetAddress)
		go func() {
			_, err := pm.sw.DialPeerWithAddress(picked)
			if err != nil {
				pm.book.MarkAttempt(picked)
			}
		}()
	}
}

// Handles incoming PEX messages.
func (pm *PeerManager) requestRoutine() {

	for {
		inMsg, ok := pm.sw.Receive(PexCh) // {Peer, Time, Packet}
		if !ok {
			// Client has stopped
			break
		}

		// decode message
		msg := decodeMessage(inMsg.Bytes)
		log.Info("requestRoutine received %v", msg)

		switch msg.(type) {
		case *pexRequestMessage:
			// inMsg.MConn.Peer requested some peers.
			// TODO: prevent abuse.
			addrs := pm.book.GetSelection()
			msg := &pexAddrsMessage{Addrs: addrs}
			queued := inMsg.MConn.Peer.TrySend(PexCh, msg)
			if !queued {
				// ignore
			}
		case *pexAddrsMessage:
			// We received some peer addresses from inMsg.MConn.Peer.
			// TODO: prevent abuse.
			// (We don't want to get spammed with bad peers)
			srcAddr := inMsg.MConn.RemoteAddress
			for _, addr := range msg.(*pexAddrsMessage).Addrs {
				pm.book.AddAddress(addr, srcAddr)
			}
		default:
			// Ignore unknown message.
			// pm.sw.StopPeerForError(inMsg.MConn.Peer, pexErrInvalidMessage)
		}
	}

	// Cleanup

}

//-----------------------------------------------------------------------------

/* Messages */

const (
	msgTypeUnknown = byte(0x00)
	msgTypeRequest = byte(0x01)
	msgTypeAddrs   = byte(0x02)
)

// TODO: check for unnecessary extra bytes at the end.
func decodeMessage(bz []byte) (msg interface{}) {
	var n int64
	var err error
	// log.Debug("decoding msg bytes: %X", bz)
	switch bz[0] {
	case msgTypeRequest:
		return &pexRequestMessage{}
	case msgTypeAddrs:
		return readPexAddrsMessage(bytes.NewReader(bz[1:]), &n, &err)
	default:
		return nil
	}
}

/*
A pexRequestMessage requests additional peer addresses.
*/
type pexRequestMessage struct {
}

func (m *pexRequestMessage) WriteTo(w io.Writer) (n int64, err error) {
	WriteByte(w, msgTypeRequest, &n, &err)
	return
}

func (m *pexRequestMessage) String() string {
	return "[pexRequest]"
}

/*
A message with announced peer addresses.
*/
type pexAddrsMessage struct {
	Addrs []*NetAddress
}

func readPexAddrsMessage(r io.Reader, n *int64, err *error) *pexAddrsMessage {
	numAddrs := int(ReadUInt32(r, n, err))
	addrs := []*NetAddress{}
	for i := 0; i < numAddrs; i++ {
		addr := ReadNetAddress(r, n, err)
		addrs = append(addrs, addr)
	}
	return &pexAddrsMessage{
		Addrs: addrs,
	}
}

func (m *pexAddrsMessage) WriteTo(w io.Writer) (n int64, err error) {
	WriteByte(w, msgTypeAddrs, &n, &err)
	WriteUInt32(w, uint32(len(m.Addrs)), &n, &err)
	for _, addr := range m.Addrs {
		WriteBinary(w, addr, &n, &err)
	}
	return
}

func (m *pexAddrsMessage) String() string {
	return fmt.Sprintf("[pexAddrs %v]", m.Addrs)
}
