package handlers

import (
	"github.com/tendermint/netmon/Godeps/_workspace/src/github.com/tendermint/go-event-meter"
	"github.com/tendermint/netmon/Godeps/_workspace/src/github.com/tendermint/go-events"

	"github.com/tendermint/netmon/types"

	tmtypes "github.com/tendermint/netmon/Godeps/_workspace/src/github.com/tendermint/tendermint/types"
)

/*
	Each chain-validator gets an eventmeter which maintains the websocket
	Certain pre-defined events may update the netmon state: latency pongs, new blocks
	TODO: config changes for new validators and changing ip/port
*/

// implements eventmeter.EventCallbackFunc
// updates validator and possibly chain with new block
func (tn *TendermintNetwork) newBlockCallback(chainState *types.ChainState, val *types.ValidatorState) eventmeter.EventCallbackFunc {
	return func(metric *eventmeter.EventMetric, data events.EventData) {
		block := data.(tmtypes.EventDataNewBlock).Block

		// these functions are thread safe
		// we should run them concurrently

		// update height for validator
		val.NewBlock(block)

		// possibly update height and mean block time for chain
		chainState.NewBlock(block)
	}
}

// implements eventmeter.EventLatencyFunc
func (tn *TendermintNetwork) latencyCallback(chain *types.ChainState, val *types.ValidatorState) eventmeter.LatencyCallbackFunc {
	return func(latency float64) {
		oldLatency := val.UpdateLatency(latency)
		chain.UpdateLatency(oldLatency, latency)
	}
}
