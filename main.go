package main

import (
	"os"
	"os/signal"

	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/consensus"
	"github.com/tendermint/tendermint/p2p"
)

type Node struct {
	lz   []p2p.Listener
	sw   *p2p.Switch
	book *p2p.AddrBook
	pmgr *p2p.PeerManager
}

func NewNode() *Node {
	// Define channels for our app
	chDescs := []*p2p.ChannelDescriptor{
		// PEX
		&p2p.ChannelDescriptor{
			Id:                p2p.PexCh,
			SendQueueCapacity: 2,
			RecvQueueCapacity: 2,
			RecvBufferSize:    1024,
			DefaultPriority:   1,
		},
		// CONSENSUS
		&p2p.ChannelDescriptor{
			Id:                consensus.ProposalCh,
			SendQueueCapacity: 2,
			RecvQueueCapacity: 10,
			RecvBufferSize:    10240,
			DefaultPriority:   5,
		},
		&p2p.ChannelDescriptor{
			Id:                consensus.KnownPartsCh,
			SendQueueCapacity: 2,
			RecvQueueCapacity: 10,
			RecvBufferSize:    1024,
			DefaultPriority:   5,
		},
		&p2p.ChannelDescriptor{
			Id:                consensus.VoteCh,
			SendQueueCapacity: 100,
			RecvQueueCapacity: 1000,
			RecvBufferSize:    10240,
			DefaultPriority:   5,
		},
		// TODO: MEMPOOL
	}
	sw := p2p.NewSwitch(chDescs)
	book := p2p.NewAddrBook(config.RootDir + "/addrbook.json")
	pmgr := p2p.NewPeerManager(sw, book)

	return &Node{
		sw:   sw,
		book: book,
		pmgr: pmgr,
	}
}

func (n *Node) Start() {
	log.Info("Starting node")
	for _, l := range n.lz {
		go n.inboundConnectionRoutine(l)
	}
	n.sw.Start()
	n.book.Start()
	n.pmgr.Start()
}

func (n *Node) Stop() {
	log.Info("Stopping node")
	// TODO: gracefully disconnect from peers.
	n.sw.Stop()
	n.book.Stop()
	n.pmgr.Stop()
}

// Add a Listener to accept inbound peer connections.
func (n *Node) AddListener(l p2p.Listener) {
	log.Info("Added %v", l)
	n.lz = append(n.lz, l)
	n.book.AddOurAddress(l.ExternalAddress())
}

func (n *Node) inboundConnectionRoutine(l p2p.Listener) {
	for {
		inConn, ok := <-l.Connections()
		if !ok {
			break
		}
		// New inbound connection!
		peer, err := n.sw.AddPeerWithConnection(inConn, false)
		if err != nil {
			log.Info("Ignoring error from inbound connection: %v\n%v",
				peer, err)
			continue
		}
		// NOTE: We don't yet have the external address of the
		// remote (if they have a listener at all).
		// PeerManager's pexRoutine will handle that.
	}

	// cleanup
}

//-----------------------------------------------------------------------------

func main() {

	// Create & start node
	n := NewNode()
	l := p2p.NewDefaultListener("tcp", config.Config.LAddr)
	n.AddListener(l)
	n.Start()

	// Seed?
	if config.Config.Seed != "" {
		peer, err := n.sw.DialPeerWithAddress(p2p.NewNetAddressString(config.Config.Seed))
		if err != nil {
			log.Error("Error dialing seed: %v", err)
			//n.book.MarkAttempt(addr)
			return
		} else {
			log.Info("Connected to seed: %v", peer)
		}
	}

	// Sleep forever and then...
	trapSignal(func() {
		n.Stop()
	})
}

func trapSignal(cb func()) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for sig := range c {
			log.Info("captured %v, exiting..", sig)
			cb()
			os.Exit(1)
		}
	}()
	select {}
}
