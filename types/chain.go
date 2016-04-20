package types

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/rcrowley/go-metrics"
	. "github.com/tendermint/go-common"
	tmtypes "github.com/tendermint/tendermint/types"
)

// waitign more than this many seconds for a block means we're unhealthy
const newBlockTimeoutSeconds = 5

//------------------------------------------------
// blockchain types
// NOTE: mintnet duplicates some types from here and val.go
//------------------------------------------------

// Known chain and validator set IDs (from which anything else can be found)
// Returned by the Status RPC
type ChainAndValidatorSetIDs struct {
	ChainIDs        []string `json:"chain_ids"`
	ValidatorSetIDs []string `json:"validator_set_ids"`
}

//------------------------------------------------
// chain state

// Main chain state
// Returned over RPC; also used to manage state
type ChainState struct {
	Config *BlockchainConfig `json:"config"`
	Status *BlockchainStatus `json:"status"`
}

func (cs *ChainState) NewBlock(block *tmtypes.Header) {
	cs.Status.NewBlock(block)
}

func (cs *ChainState) UpdateLatency(oldLatency, newLatency float64) {
	cs.Status.UpdateLatency(oldLatency, newLatency)
}

func (cs *ChainState) SetOnline(val *ValidatorState, isOnline bool) {
	cs.Status.SetOnline(val, isOnline)
}

//------------------------------------------------
// Blockchain Config: id, validator config

// Chain Config
type BlockchainConfig struct {
	// should be fixed for life of chain
	ID       string `json:"id"`
	ValSetID string `json:"val_set_id"` // NOTE: do we really commit to one val set per chain?

	// handles live validator states (latency, last block, etc)
	// and validator set changes
	mtx        sync.Mutex
	Validators []*ValidatorState `json:"validators"` // TODO: this should be ValidatorConfig and the state in BlockchainStatus
	valIDMap   map[string]int    // map IDs to indices
}

// So we can fetch validator by id rather than index
func (bc *BlockchainConfig) PopulateValIDMap() {
	bc.mtx.Lock()
	defer bc.mtx.Unlock()
	bc.valIDMap = make(map[string]int)
	for i, v := range bc.Validators {
		bc.valIDMap[v.Config.Validator.ID] = i
	}
}

func (bc *BlockchainConfig) GetValidatorByID(valID string) (*ValidatorState, error) {
	bc.mtx.Lock()
	defer bc.mtx.Unlock()
	valIndex, ok := bc.valIDMap[valID]
	if !ok {
		return nil, fmt.Errorf("Unknown validator %s", valID)
	}
	return bc.Validators[valIndex], nil
}

//------------------------------------------------
// BlockchainStatus

// Basic blockchain metrics
type BlockchainStatus struct {
	mtx sync.Mutex

	// Blockchain Info
	Height         int     `json:"height"` // latest height we've got
	BlockchainSize int64   `json:"blockchain_size"`
	MeanBlockTime  float64 `json:"mean_block_time" wire:"unsafe"` // ms (avg over last minute)
	TxThroughput   float64 `json:"tx_throughput" wire:"unsafe"`   // tx/s (avg over last minute)

	blockTimeMeter    metrics.Meter
	txThroughputMeter metrics.Meter

	// Network Info
	NumValidators    int `json:"num_validators"`
	ActiveValidators int `json:"active_validators"`
	//ActiveNodes      int     `json:"active_nodes"`
	MeanLatency float64 `json:"mean_latency" wire:"unsafe"` // ms

	// Health
	FullHealth bool `json:"full_health"` // all validators online, synced, making blocks
	Healthy    bool `json:"healthy"`     // we're making blocks

	// Uptime
	UptimeData *UptimeData `json:"uptime_data"`

	// What else can we get / do we want?
	// TODO: charts for block time, latency (websockets/event-meter ?)

	// for benchmark runs
	benchResults *BenchmarkResults
}

func (bc *BlockchainStatus) BenchmarkTxs(results chan *BenchmarkResults, nTxs int, args []string) {
	log.Notice("Running benchmark", "ntxs", nTxs)
	bc.benchResults = &BenchmarkResults{
		StartTime: time.Now(),
		nTxs:      nTxs,
		results:   results,
	}

	if len(args) > 0 {
		// TODO: capture output to file
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		go cmd.Run()
	}
}

func (bc *BlockchainStatus) BenchmarkBlocks(results chan *BenchmarkResults, nBlocks int, args []string) {
	log.Notice("Running benchmark", "nblocks", nBlocks)
	bc.benchResults = &BenchmarkResults{
		StartTime: time.Now(),
		nBlocks:   nBlocks,
		results:   results,
	}

	if len(args) > 0 {
		// TODO: capture output to file
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		go cmd.Run()
	}
}

type Block struct {
	Time   time.Time `json:time"`
	Height int       `json:"height"`
	NumTxs int       `json:"num_txs"`
}

type BenchmarkResults struct {
	StartTime      time.Time `json:"start_time"`
	StartBlock     int       `json:"start_block"`
	TotalTime      float64   `json:"total_time"` // seconds
	Blocks         []*Block  `json:"blocks"`
	NumBlocks      int       `json:"num_blocks"`
	NumTxs         int       `json:"num_txs`
	MeanLatency    float64   `json:"latency"`    // seconds per block
	MeanThroughput float64   `json:"throughput"` // txs per second

	// either we wait for n blocks or n txs
	nBlocks int
	nTxs    int

	done    bool
	results chan *BenchmarkResults
}

// Return the total time to commit all txs, in seconds
func (br *BenchmarkResults) ElapsedTime() float64 {
	return float64(br.Blocks[br.NumBlocks-1].Time.Sub(br.StartTime)) / float64(1000000000)
}

// Return the avg seconds/block
func (br *BenchmarkResults) Latency() float64 {
	return br.ElapsedTime() / float64(br.NumBlocks)
}

// Return the avg txs/second
func (br *BenchmarkResults) Throughput() float64 {
	return float64(br.NumTxs) / br.ElapsedTime()
}

func (br *BenchmarkResults) Done() {
	log.Info("Done benchmark", "num blocks", br.NumBlocks, "block len", len(br.Blocks))
	br.done = true
	br.TotalTime = br.ElapsedTime()
	br.MeanThroughput = br.Throughput()
	br.MeanLatency = br.Latency()
	br.results <- br
}

type UptimeData struct {
	StartTime time.Time `json:"start_time"`
	Uptime    float64   `json:"uptime" wire:"unsafe"` // Percentage of time we've been Healthy, ever

	totalDownTime time.Duration // total downtime (only updated when we come back online)
	wentDown      time.Time

	// TODO: uptime over last day, month, year
}

func NewBlockchainStatus() *BlockchainStatus {
	return &BlockchainStatus{
		blockTimeMeter:    metrics.NewMeter(),
		txThroughputMeter: metrics.NewMeter(),
		Healthy:           true,
		UptimeData: &UptimeData{
			StartTime: time.Now(),
			Uptime:    100.0,
		},
	}
}

func (s *BlockchainStatus) NewBlock(block *tmtypes.Header) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	if block.Height > s.Height {
		numTxs := block.NumTxs
		s.Height = block.Height
		s.blockTimeMeter.Mark(1)
		s.txThroughputMeter.Mark(int64(numTxs))
		s.MeanBlockTime = (1.0 / s.blockTimeMeter.Rate1()) * 1000 // 1/s to ms
		s.TxThroughput = s.txThroughputMeter.Rate1()

		log.Debug("New Block", "height", s.Height, "ntxs", numTxs)
		if s.benchResults != nil && !s.benchResults.done {
			if s.benchResults.StartBlock == 0 && numTxs > 0 {
				s.benchResults.StartBlock = s.Height
			}
			s.benchResults.Blocks = append(s.benchResults.Blocks, &Block{
				Time:   time.Now(),
				Height: s.Height,
				NumTxs: numTxs,
			})
			s.benchResults.NumTxs += numTxs
			s.benchResults.NumBlocks += 1
			if s.benchResults.nTxs > 0 && s.benchResults.NumTxs >= s.benchResults.nTxs {
				s.benchResults.Done()
			} else if s.benchResults.nBlocks > 0 && s.benchResults.NumBlocks >= s.benchResults.nBlocks {
				s.benchResults.Done()
			}
		}

		// if we're making blocks, we're healthy
		if !s.Healthy {
			s.Healthy = true
			s.UptimeData.totalDownTime += time.Since(s.UptimeData.wentDown)
		}

		// if we are connected to all validators, we're at full health
		// TODO: make sure they're all at the same height (within a block) and all proposing (and possibly validating )
		// Alternatively, just check there hasn't been a new round in numValidators rounds
		if s.ActiveValidators == s.NumValidators {
			s.FullHealth = true
		}

		// TODO: should we refactor so there's a central loop and ticker?
		go s.newBlockTimeout(s.Height)
	}
}

// we have newBlockTimeoutSeconds to make a new block, else we're unhealthy
func (s *BlockchainStatus) newBlockTimeout(height int) {
	time.Sleep(time.Second * newBlockTimeoutSeconds)

	s.mtx.Lock()
	defer s.mtx.Unlock()
	if !(s.Height > height) {
		s.Healthy = false
		s.UptimeData.wentDown = time.Now()
	}
}

// Used to calculate uptime on demand. TODO: refactor this into the central loop ...
func (s *BlockchainStatus) RealTimeUpdates() {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	since := time.Since(s.UptimeData.StartTime)
	uptime := since - s.UptimeData.totalDownTime
	if !s.Healthy {
		uptime -= time.Since(s.UptimeData.wentDown)
	}
	s.UptimeData.Uptime = float64(uptime) / float64(since)
}

func (s *BlockchainStatus) UpdateLatency(oldLatency, newLatency float64) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	// update avg validator rpc latency
	mean := s.MeanLatency * float64(s.NumValidators)
	mean = (mean - oldLatency + newLatency) / float64(s.NumValidators)
	s.MeanLatency = mean
}

// Toggle validators online/offline (updates ActiveValidators and FullHealth)
func (s *BlockchainStatus) SetOnline(val *ValidatorState, isOnline bool) {
	val.SetOnline(isOnline)

	var change int
	if isOnline {
		change = 1
	} else {
		change = -1
	}

	s.mtx.Lock()
	defer s.mtx.Unlock()

	s.ActiveValidators += change

	if s.ActiveValidators > s.NumValidators {
		panic(Fmt("got %d validators. max %ds", s.ActiveValidators, s.NumValidators))
	}

	// if we lost a connection we're no longer at full health, even if it's still online.
	// so long as we receive blocks, we'll know we're still healthy
	if s.ActiveValidators != s.NumValidators {
		s.FullHealth = false
	}
}

func TwoThirdsMaj(count, total int) bool {
	return float64(count) > (2.0/3.0)*float64(total)
}
