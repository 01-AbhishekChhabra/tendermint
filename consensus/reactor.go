package consensus

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/blocks"
	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/mempool"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/state"
)

const (
	StateCh = byte(0x20)
	DataCh  = byte(0x21)
	VoteCh  = byte(0x22)

	peerStateKey = "ConsensusReactor.peerState"

	peerGossipSleepDuration = 50 * time.Millisecond // Time to sleep if there's nothing to send.
	hasVotesThreshold       = 50                    // After this many new votes we'll send a HasVotesMessage.
)

//-----------------------------------------------------------------------------

type ConsensusReactor struct {
	sw      *p2p.Switch
	started uint32
	stopped uint32
	quit    chan struct{}

	conS *ConsensusState
}

func NewConsensusReactor(blockStore *BlockStore, mempool *mempool.Mempool, state *state.State) *ConsensusReactor {
	conS := NewConsensusState(state, blockStore, mempool)
	conR := &ConsensusReactor{
		quit: make(chan struct{}),
		conS: conS,
	}
	return conR
}

// Implements Reactor
func (conR *ConsensusReactor) Start(sw *p2p.Switch) {
	if atomic.CompareAndSwapUint32(&conR.started, 0, 1) {
		log.Info("Starting ConsensusReactor")
		conR.sw = sw
		conR.conS.Start()
		go conR.broadcastNewRoundStepRoutine()
	}
}

// Implements Reactor
func (conR *ConsensusReactor) Stop() {
	if atomic.CompareAndSwapUint32(&conR.stopped, 0, 1) {
		log.Info("Stopping ConsensusReactor")
		conR.conS.Stop()
		close(conR.quit)
	}
}

func (conR *ConsensusReactor) IsStopped() bool {
	return atomic.LoadUint32(&conR.stopped) == 1
}

// Implements Reactor
func (conR *ConsensusReactor) GetChannels() []*p2p.ChannelDescriptor {
	// TODO optimize
	return []*p2p.ChannelDescriptor{
		&p2p.ChannelDescriptor{
			Id:       StateCh,
			Priority: 5,
		},
		&p2p.ChannelDescriptor{
			Id:       DataCh,
			Priority: 5,
		},
		&p2p.ChannelDescriptor{
			Id:       VoteCh,
			Priority: 5,
		},
	}
}

// Implements Reactor
func (conR *ConsensusReactor) AddPeer(peer *p2p.Peer) {
	// Create peerState for peer
	peerState := NewPeerState(peer)
	peer.Data.Set(peerStateKey, peerState)

	// Begin gossip routines for this peer.
	go conR.gossipDataRoutine(peer, peerState)
	go conR.gossipVotesRoutine(peer, peerState)
}

// Implements Reactor
func (conR *ConsensusReactor) RemovePeer(peer *p2p.Peer, reason interface{}) {
	//peer.Data.Get(peerStateKey).(*PeerState).Disconnect()
}

// Implements Reactor
func (conR *ConsensusReactor) Receive(chId byte, peer *p2p.Peer, msgBytes []byte) {

	// Get round state
	rs := conR.conS.GetRoundState()
	ps := peer.Data.Get(peerStateKey).(*PeerState)
	_, msg_ := decodeMessage(msgBytes)
	voteAddCounter := 0
	var err error = nil

	log.Debug("[%X][%v] Receive: %v", chId, peer, msg_)

	switch chId {
	case StateCh:
		switch msg_.(type) {
		case *NewRoundStepMessage:
			msg := msg_.(*NewRoundStepMessage)
			ps.ApplyNewRoundStepMessage(msg, rs)

		case *HasVotesMessage:
			msg := msg_.(*HasVotesMessage)
			ps.ApplyHasVotesMessage(msg)

		default:
			// Ignore unknown message
		}

	case DataCh:
		switch msg_.(type) {
		case *Proposal:
			proposal := msg_.(*Proposal)
			ps.SetHasProposal(proposal)
			err = conR.conS.SetProposal(proposal)

		case *PartMessage:
			msg := msg_.(*PartMessage)
			if msg.Type == partTypeProposalBlock {
				ps.SetHasProposalBlockPart(msg.Height, msg.Round, msg.Part.Index)
				_, err = conR.conS.AddProposalBlockPart(msg.Height, msg.Round, msg.Part)
			} else if msg.Type == partTypeProposalPOL {
				ps.SetHasProposalPOLPart(msg.Height, msg.Round, msg.Part.Index)
				_, err = conR.conS.AddProposalPOLPart(msg.Height, msg.Round, msg.Part)
			} else {
				// Ignore unknown part type
			}

		default:
			// Ignore unknown message
		}

	case VoteCh:
		switch msg_.(type) {
		case *Vote:
			vote := msg_.(*Vote)
			// We can't deal with votes from another height,
			// as they have a different validator set.
			if vote.Height != rs.Height || vote.Height != ps.Height {
				return
			}
			index, val := rs.Validators.GetById(vote.SignerId)
			if val == nil {
				log.Warning("Peer gave us an invalid vote.")
				return
			}
			ps.EnsureVoteBitArrays(rs.Height, rs.Round, rs.Validators.Size())
			ps.SetHasVote(rs.Height, rs.Round, index, vote)
			added, err := conR.conS.AddVote(vote)
			if err != nil {
				log.Warning("Error attempting to add vote: %v", err)
			}
			if added {
				// Maybe send HasVotesMessage
				// TODO optimize. It would be better to just acks for each vote!
				voteAddCounter++
				if voteAddCounter%hasVotesThreshold == 0 {
					msg := &HasVotesMessage{
						Height:     rs.Height,
						Round:      rs.Round,
						Prevotes:   rs.Prevotes.BitArray(),
						Precommits: rs.Precommits.BitArray(),
						Commits:    rs.Commits.BitArray(),
					}
					conR.sw.Broadcast(StateCh, msg)
				}
			}

		default:
			// Ignore unknown message
		}
	default:
		// Ignore unknown channel
	}

	if err != nil {
		log.Warning("Error in Receive(): %v", err)
	}
}

// Sets our private validator account for signing votes.
func (conR *ConsensusReactor) SetPrivValidator(priv *PrivValidator) {
	conR.conS.SetPrivValidator(priv)
}

//--------------------------------------

// XXX We need to ensure that Proposal* etc are also set appropriately.
// Listens for changes to the ConsensusState.Step by pulling
// on conR.conS.NewStepCh().
func (conR *ConsensusReactor) broadcastNewRoundStepRoutine() {
	for {
		// Get RoundState with new Step or quit.
		var rs *RoundState
		select {
		case rs = <-conR.conS.NewStepCh():
		case <-conR.quit:
			return
		}

		// Get seconds since beginning of height.
		// Due to the condition documented, this is safe.
		timeElapsed := rs.StartTime.Sub(time.Now())

		// Broadcast NewRoundStepMessage
		msg := &NewRoundStepMessage{
			Height: rs.Height,
			Round:  rs.Round,
			Step:   rs.Step,
			SecondsSinceStartTime: uint32(timeElapsed.Seconds()),
		}
		conR.sw.Broadcast(StateCh, msg)
	}
}

func (conR *ConsensusReactor) gossipDataRoutine(peer *p2p.Peer, ps *PeerState) {

OUTER_LOOP:
	for {
		// Manage disconnects from self or peer.
		if peer.IsStopped() || conR.IsStopped() {
			log.Info("Stopping gossipDataRoutine for %v.", peer)
			return
		}
		rs := conR.conS.GetRoundState()
		prs := ps.GetRoundState()

		// Send proposal Block parts?
		// NOTE: if we or peer is at RoundStepCommit*, the round
		// won't necessarily match, but that's OK.
		if rs.ProposalBlockParts.Header().Equals(prs.ProposalBlockParts) {
			if index, ok := rs.ProposalBlockParts.BitArray().Sub(
				prs.ProposalBlockBitArray).PickRandom(); ok {
				msg := &PartMessage{
					Height: rs.Height,
					Round:  rs.Round,
					Type:   partTypeProposalBlock,
					Part:   rs.ProposalBlockParts.GetPart(uint16(index)),
				}
				peer.Send(DataCh, msg)
				ps.SetHasProposalBlockPart(rs.Height, rs.Round, uint16(index))
				continue OUTER_LOOP
			}
		}

		// If height and round doesn't match, sleep.
		if rs.Height != prs.Height || rs.Round != prs.Round {
			time.Sleep(peerGossipSleepDuration)
			continue OUTER_LOOP
		}

		// Send proposal?
		if rs.Proposal != nil && !prs.Proposal {
			msg := p2p.TypedMessage{msgTypeProposal, rs.Proposal}
			peer.Send(DataCh, msg)
			ps.SetHasProposal(rs.Proposal)
			continue OUTER_LOOP
		}

		// Send proposal POL parts?
		if rs.ProposalPOLParts.Header().Equals(prs.ProposalPOLParts) {
			if index, ok := rs.ProposalPOLParts.BitArray().Sub(
				prs.ProposalPOLBitArray).PickRandom(); ok {
				msg := &PartMessage{
					Height: rs.Height,
					Round:  rs.Round,
					Type:   partTypeProposalPOL,
					Part:   rs.ProposalPOLParts.GetPart(uint16(index)),
				}
				peer.Send(DataCh, msg)
				ps.SetHasProposalPOLPart(rs.Height, rs.Round, uint16(index))
				continue OUTER_LOOP
			}
		}

		// Nothing to do. Sleep.
		time.Sleep(peerGossipSleepDuration)
		continue OUTER_LOOP
	}
}

// XXX Need to also send commits for LastComits.
func (conR *ConsensusReactor) gossipVotesRoutine(peer *p2p.Peer, ps *PeerState) {
OUTER_LOOP:
	for {
		// Manage disconnects from self or peer.
		if peer.IsStopped() || conR.IsStopped() {
			log.Info("Stopping gossipVotesRoutine for %v.", peer)
			return
		}
		rs := conR.conS.GetRoundState()
		prs := ps.GetRoundState()

		// If height doesn't match, sleep.
		if rs.Height != prs.Height {
			time.Sleep(peerGossipSleepDuration)
			continue OUTER_LOOP
		}

		// Ensure that peer's prevote/precommit/commit bitarrays of of sufficient capacity
		ps.EnsureVoteBitArrays(rs.Height, rs.Round, rs.Validators.Size())

		trySendVote := func(voteSet *VoteSet, peerVoteSet BitArray) (sent bool) {
			// TODO: give priority to our vote.
			index, ok := voteSet.BitArray().Sub(peerVoteSet).PickRandom()
			if ok {
				vote := voteSet.GetByIndex(index)
				// NOTE: vote may be a commit.
				msg := p2p.TypedMessage{msgTypeVote, vote}
				peer.Send(VoteCh, msg)
				ps.SetHasVote(rs.Height, rs.Round, index, vote)
				return true
			}
			return false
		}

		// If there are prevotes to send...
		if rs.Round == prs.Round && prs.Step <= RoundStepPrevote {
			if trySendVote(rs.Prevotes, prs.Prevotes) {
				continue OUTER_LOOP
			}
		}

		// If there are precommits to send...
		if rs.Round == prs.Round && prs.Step <= RoundStepPrecommit {
			if trySendVote(rs.Precommits, prs.Precommits) {
				continue OUTER_LOOP
			}
		}

		// If there are any commits to send...
		if trySendVote(rs.Commits, prs.Commits) {
			continue OUTER_LOOP
		}

		// We sent nothing. Sleep...
		time.Sleep(peerGossipSleepDuration)
		continue OUTER_LOOP
	}
}

//-----------------------------------------------------------------------------

// Read only when returned by PeerState.GetRoundState().
type PeerRoundState struct {
	Height                uint32        // Height peer is at
	Round                 uint16        // Round peer is at
	Step                  RoundStep     // Step peer is at
	StartTime             time.Time     // Estimated start of round 0 at this height
	Proposal              bool          // True if peer has proposal for this round
	ProposalBlockParts    PartSetHeader //
	ProposalBlockBitArray BitArray      // True bit -> has part
	ProposalPOLParts      PartSetHeader //
	ProposalPOLBitArray   BitArray      // True bit -> has part
	Prevotes              BitArray      // All votes peer has for this round
	Precommits            BitArray      // All precommits peer has for this round
	Commits               BitArray      // All commits peer has for this height
}

//-----------------------------------------------------------------------------

var (
	ErrPeerStateHeightRegression = errors.New("Error peer state height regression")
	ErrPeerStateInvalidStartTime = errors.New("Error peer state invalid startTime")
)

type PeerState struct {
	mtx sync.Mutex
	PeerRoundState
}

func NewPeerState(peer *p2p.Peer) *PeerState {
	return &PeerState{}
}

// Returns an atomic snapshot of the PeerRoundState.
// There's no point in mutating it since it won't change PeerState.
func (ps *PeerState) GetRoundState() *PeerRoundState {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	prs := ps.PeerRoundState // copy
	return &prs
}

func (ps *PeerState) SetHasProposal(proposal *Proposal) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if ps.Height != proposal.Height || ps.Round != proposal.Round {
		return
	}
	if ps.Proposal {
		return
	}

	ps.Proposal = true
	ps.ProposalBlockParts = proposal.BlockParts
	ps.ProposalBlockBitArray = NewBitArray(uint(proposal.BlockParts.Total))
	ps.ProposalPOLParts = proposal.POLParts
	ps.ProposalPOLBitArray = NewBitArray(uint(proposal.POLParts.Total))
}

func (ps *PeerState) SetHasProposalBlockPart(height uint32, round uint16, index uint16) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if ps.Height != height || ps.Round != round {
		return
	}

	ps.ProposalBlockBitArray.SetIndex(uint(index), true)
}

func (ps *PeerState) SetHasProposalPOLPart(height uint32, round uint16, index uint16) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if ps.Height != height || ps.Round != round {
		return
	}

	ps.ProposalPOLBitArray.SetIndex(uint(index), true)
}

func (ps *PeerState) EnsureVoteBitArrays(height uint32, round uint16, numValidators uint) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if ps.Height != height || ps.Round != round {
		return
	}

	if ps.Prevotes.IsZero() {
		ps.Prevotes = NewBitArray(numValidators)
	}
	if ps.Precommits.IsZero() {
		ps.Precommits = NewBitArray(numValidators)
	}
	if ps.Commits.IsZero() {
		ps.Commits = NewBitArray(numValidators)
	}
}

func (ps *PeerState) SetHasVote(height uint32, round uint16, index uint, vote *Vote) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if ps.Height != height {
		return
	}

	switch vote.Type {
	case VoteTypePrevote:
		ps.Prevotes.SetIndex(index, true)
	case VoteTypePrecommit:
		ps.Precommits.SetIndex(index, true)
	case VoteTypeCommit:
		if vote.Round < round {
			ps.Prevotes.SetIndex(index, true)
			ps.Precommits.SetIndex(index, true)
		}
		ps.Commits.SetIndex(index, true)
	default:
		panic("Invalid vote type")
	}
}

func (ps *PeerState) ApplyNewRoundStepMessage(msg *NewRoundStepMessage, rs *RoundState) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	// Just remember these values.
	psHeight := ps.Height
	psRound := ps.Round
	//psStep := ps.Step

	startTime := time.Now().Add(-1 * time.Duration(msg.SecondsSinceStartTime) * time.Second)
	ps.Height = msg.Height
	ps.Round = msg.Round
	ps.Step = msg.Step
	ps.StartTime = startTime
	if psHeight != msg.Height || psRound != msg.Round {
		ps.Proposal = false
		ps.ProposalBlockParts = PartSetHeader{}
		ps.ProposalBlockBitArray = BitArray{}
		ps.ProposalPOLParts = PartSetHeader{}
		ps.ProposalPOLBitArray = BitArray{}
		// We'll update the BitArray capacity later.
		ps.Prevotes = BitArray{}
		ps.Precommits = BitArray{}
	}
	if psHeight != msg.Height {
		// We'll update the BitArray capacity later.
		ps.Commits = BitArray{}
	}
}

func (ps *PeerState) ApplyHasVotesMessage(msg *HasVotesMessage) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if ps.Height != msg.Height {
		return
	}

	ps.Commits = ps.Commits.Or(msg.Commits)
	if ps.Round == msg.Round {
		ps.Prevotes = ps.Prevotes.Or(msg.Prevotes)
		ps.Precommits = ps.Precommits.Or(msg.Precommits)
	} else {
		ps.Prevotes = msg.Prevotes
		ps.Precommits = msg.Precommits
	}
}

//-----------------------------------------------------------------------------
// Messages

const (
	msgTypeUnknown = byte(0x00)
	// Messages for communicating state changes
	msgTypeNewRoundStep = byte(0x01)
	msgTypeHasVotes     = byte(0x02)
	// Messages of data
	msgTypeProposal = byte(0x11)
	msgTypePart     = byte(0x12) // both block & POL
	msgTypeVote     = byte(0x13)
)

// TODO: check for unnecessary extra bytes at the end.
func decodeMessage(bz []byte) (msgType byte, msg interface{}) {
	n, err := new(int64), new(error)
	// log.Debug("decoding msg bytes: %X", bz)
	msgType = bz[0]
	r := bytes.NewReader(bz[1:])
	switch msgType {
	// Messages for communicating state changes
	case msgTypeNewRoundStep:
		msg = readNewRoundStepMessage(r, n, err)
	case msgTypeHasVotes:
		msg = readHasVotesMessage(r, n, err)
	// Messages of data
	case msgTypeProposal:
		msg = ReadProposal(r, n, err)
	case msgTypePart:
		msg = readPartMessage(r, n, err)
	case msgTypeVote:
		msg = ReadVote(r, n, err)
	default:
		msg = nil
	}
	return
}

//-------------------------------------

type NewRoundStepMessage struct {
	Height                uint32
	Round                 uint16
	Step                  RoundStep
	SecondsSinceStartTime uint32
}

func readNewRoundStepMessage(r io.Reader, n *int64, err *error) *NewRoundStepMessage {
	return &NewRoundStepMessage{
		Height: ReadUInt32(r, n, err),
		Round:  ReadUInt16(r, n, err),
		Step:   RoundStep(ReadUInt8(r, n, err)),
		SecondsSinceStartTime: ReadUInt32(r, n, err),
	}
}

func (m *NewRoundStepMessage) WriteTo(w io.Writer) (n int64, err error) {
	WriteByte(w, msgTypeNewRoundStep, &n, &err)
	WriteUInt32(w, m.Height, &n, &err)
	WriteUInt16(w, m.Round, &n, &err)
	WriteUInt8(w, uint8(m.Step), &n, &err)
	WriteUInt32(w, m.SecondsSinceStartTime, &n, &err)
	return
}

func (m *NewRoundStepMessage) String() string {
	return fmt.Sprintf("[NewRoundStep %v/%v/%X]", m.Height, m.Round, m.Step)
}

//-------------------------------------

type HasVotesMessage struct {
	Height     uint32
	Round      uint16
	Prevotes   BitArray
	Precommits BitArray
	Commits    BitArray
}

func readHasVotesMessage(r io.Reader, n *int64, err *error) *HasVotesMessage {
	return &HasVotesMessage{
		Height:     ReadUInt32(r, n, err),
		Round:      ReadUInt16(r, n, err),
		Prevotes:   ReadBitArray(r, n, err),
		Precommits: ReadBitArray(r, n, err),
		Commits:    ReadBitArray(r, n, err),
	}
}

func (m *HasVotesMessage) WriteTo(w io.Writer) (n int64, err error) {
	WriteByte(w, msgTypeHasVotes, &n, &err)
	WriteUInt32(w, m.Height, &n, &err)
	WriteUInt16(w, m.Round, &n, &err)
	WriteBinary(w, m.Prevotes, &n, &err)
	WriteBinary(w, m.Precommits, &n, &err)
	WriteBinary(w, m.Commits, &n, &err)
	return
}

func (m *HasVotesMessage) String() string {
	return fmt.Sprintf("[HasVotesMessage H:%v R:%v]", m.Height, m.Round)
}

//-------------------------------------

const (
	partTypeProposalBlock = byte(0x01)
	partTypeProposalPOL   = byte(0x02)
)

type PartMessage struct {
	Height uint32
	Round  uint16
	Type   byte
	Part   *Part
}

func readPartMessage(r io.Reader, n *int64, err *error) *PartMessage {
	return &PartMessage{
		Height: ReadUInt32(r, n, err),
		Round:  ReadUInt16(r, n, err),
		Type:   ReadByte(r, n, err),
		Part:   ReadPart(r, n, err),
	}
}

func (m *PartMessage) WriteTo(w io.Writer) (n int64, err error) {
	WriteByte(w, msgTypePart, &n, &err)
	WriteUInt32(w, m.Height, &n, &err)
	WriteUInt16(w, m.Round, &n, &err)
	WriteByte(w, m.Type, &n, &err)
	WriteBinary(w, m.Part, &n, &err)
	return
}

func (m *PartMessage) String() string {
	return fmt.Sprintf("[PartMessage H:%v R:%v T:%X]", m.Height, m.Round, m.Type)
}
