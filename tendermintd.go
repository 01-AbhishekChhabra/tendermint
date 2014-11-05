// +build tendermintd

package main

import (
	"os"
	"os/signal"

	"github.com/tendermint/tendermint/blocks"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/consensus"
	db_ "github.com/tendermint/tendermint/db"
	mempool_ "github.com/tendermint/tendermint/mempool"
	"github.com/tendermint/tendermint/p2p"
	state_ "github.com/tendermint/tendermint/state"
)

type Node struct {
	lz               []p2p.Listener
	sw               *p2p.Switch
	book             *p2p.AddrBook
	pexReactor       *p2p.PEXReactor
	mempoolReactor   *mempool_.MempoolReactor
	consensusReactor *consensus.ConsensusReactor
	privValidator    *consensus.PrivValidator
}

func NewNode() *Node {
	// Get BlockStore
	blockStoreDB := db_.NewMemDB() // TODO configurable db.
	blockStore := blocks.NewBlockStore(blockStoreDB)

	// Get State
	stateDB := db_.NewMemDB() // TODO configurable db.
	state := state_.LoadState(stateDB)
	if state == nil {
		state = state_.GenesisStateFromFile(stateDB, config.RootDir+"/genesis.json")
		state.Save()
	}

	// Get PrivAccount
	var privValidator *consensus.PrivValidator
	if _, err := os.Stat(config.RootDir + "/private.json"); err == nil {
		privAccount := state_.PrivAccountFromFile(config.RootDir + "/private.json")
		privValidatorDB := db_.NewMemDB() // TODO configurable db.
		privValidator = consensus.NewPrivValidator(privValidatorDB, privAccount)
	}

	// Get PEXReactor
	book := p2p.NewAddrBook(config.RootDir + "/addrbook.json")
	pexReactor := p2p.NewPEXReactor(book)

	// Get MempoolReactor
	mempool := mempool_.NewMempool(state)
	mempoolReactor := mempool_.NewMempoolReactor(mempool)

	// Get ConsensusReactor
	consensusReactor := consensus.NewConsensusReactor(blockStore, mempool, state)
	if privValidator != nil {
		consensusReactor.SetPrivValidator(privValidator)
	}

	sw := p2p.NewSwitch([]p2p.Reactor{pexReactor, mempoolReactor, consensusReactor})

	return &Node{
		sw:               sw,
		book:             book,
		pexReactor:       pexReactor,
		mempoolReactor:   mempoolReactor,
		consensusReactor: consensusReactor,
		privValidator:    privValidator,
	}
}

func (n *Node) Start() {
	log.Info("Starting node")
	for _, l := range n.lz {
		go n.inboundConnectionRoutine(l)
	}
	n.book.Start()
	n.sw.Start()
}

func (n *Node) Stop() {
	log.Info("Stopping node")
	// TODO: gracefully disconnect from peers.
	n.sw.Stop()
	n.book.Stop()
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
		// PEXReactor's pexRoutine will handle that.
	}

	// cleanup
}

//-----------------------------------------------------------------------------

func main() {

	// Parse config flags
	config.ParseFlags()

	// Create & start node
	n := NewNode()
	l := p2p.NewDefaultListener("tcp", config.Config.LAddr)
	n.AddListener(l)
	n.Start()

	// If seedNode is provided by config, dial out.
	if config.Config.SeedNode != "" {
		peer, err := n.sw.DialPeerWithAddress(p2p.NewNetAddressString(config.Config.SeedNode))
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
