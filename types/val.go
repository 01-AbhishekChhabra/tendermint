package types

import (
	"fmt"
	"sync"

	"github.com/tendermint/go-crypto"
	"github.com/tendermint/go-event-meter"
	client "github.com/tendermint/go-rpc/client"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	tmtypes "github.com/tendermint/tendermint/types"
)

//------------------------------------------------
// validator types
//------------------------------------------------

//------------------------------------------------
// simple validator set and validator (just crypto, no network)

// validator set (independent of chains)
type ValidatorSet struct {
	ID         string       `json:"id"`
	Validators []*Validator `json:"validators"`
}

func (vs *ValidatorSet) Validator(valID string) (*Validator, error) {
	for _, v := range vs.Validators {
		if v.ID == valID {
			return v, nil
		}
	}
	return nil, fmt.Errorf("Unknwon validator %s", valID)
}

// validator (independent of chain)
type Validator struct {
	ID     string        `json:"id"`
	PubKey crypto.PubKey `json:"pub_key"`
	Chains []string      `json:"chains,omitempty"` // TODO: put this elsewhere (?)
}

//------------------------------------------------
// Live validator on a chain

// Validator on a chain
// Returned over RPC but also used to manage state
// Responsible for communication with the validator
type ValidatorState struct {
	Config *ValidatorConfig `json:"config"`
	Status *ValidatorStatus `json:"status"`

	// Currently we get IPs and dial,
	// but should reverse so the nodes dial the netmon,
	// both for node privacy and easier reconfig (validators changing ip/port)
	em     *eventmeter.EventMeter // holds a ws connection to the val
	client *client.ClientURI      // rpc client
}

// Start a new event meter, including the websocket connection
// Also create the http rpc client for convenienve
func (vs *ValidatorState) Start() error {
	// we need the lock because RPCAddr can be updated concurrently
	vs.Config.mtx.Lock()
	rpcAddr := vs.Config.RPCAddr
	vs.Config.mtx.Unlock()

	em := eventmeter.NewEventMeter(fmt.Sprintf("ws://%s/websocket", rpcAddr), ctypes.UnmarshalEvent)
	if _, err := em.Start(); err != nil {
		return err
	}
	vs.em = em
	vs.client = client.NewClientURI(fmt.Sprintf("http://%s", rpcAddr))
	return nil
}

func (vs *ValidatorState) Stop() {
	vs.em.Stop()
}

func (vs *ValidatorState) EventMeter() *eventmeter.EventMeter {
	return vs.em
}

func (vs *ValidatorState) NewBlock(block *tmtypes.Block) {
	vs.Status.mtx.Lock()
	defer vs.Status.mtx.Unlock()
	vs.Status.BlockHeight = block.Header.Height
}

func (vs *ValidatorState) UpdateLatency(latency float64) float64 {
	vs.Status.mtx.Lock()
	defer vs.Status.mtx.Unlock()
	old := vs.Status.Latency
	vs.Status.Latency = latency
	return old
}

func (vs *ValidatorState) SetOnline(isOnline bool) {
	vs.Status.mtx.Lock()
	defer vs.Status.mtx.Unlock()
	vs.Status.Online = isOnline
}

// Return the validators pubkey. If it's not yet set, get it from the node
// TODO: proof that it's the node's key
// XXX: Is this necessary? Why would it not be set
func (vs *ValidatorState) PubKey() crypto.PubKey {
	if vs.Config.Validator.PubKey != nil {
		return vs.Config.Validator.PubKey
	}

	var result ctypes.TMResult
	_, err := vs.client.Call("status", nil, &result)
	if err != nil {
		log.Error("Error getting validator pubkey", "addr", vs.Config.RPCAddr, "val", vs.Config.Validator.ID, "error", err)
		return nil
	}
	status := result.(*ctypes.ResultStatus)
	vs.Config.Validator.PubKey = status.PubKey
	return vs.Config.Validator.PubKey
}

type ValidatorConfig struct {
	mtx       sync.Mutex
	Validator *Validator `json:"validator"`
	P2PAddr   string     `json:"p2p_addr"`
	RPCAddr   string     `json:"rpc_addr"`
	Index     int        `json:"index,omitempty"`
}

// TODO: update p2p address

func (vc *ValidatorConfig) UpdateRPCAddress(rpcAddr string) {
	vc.mtx.Lock()
	defer vc.mtx.Unlock()
	vc.RPCAddr = rpcAddr
}

type ValidatorStatus struct {
	mtx         sync.Mutex
	Online      bool    `json:"online"`
	Latency     float64 `json:"latency" wire:"unsafe"`
	BlockHeight int     `json:"block_height"`
}
