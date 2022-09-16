package fork_metrics

import (
	"fmt"

	"github.com/attestantio/go-eth2-client/http"
	"github.com/attestantio/go-eth2-client/spec"
	"github.com/cortze/eth2-state-analyzer/pkg/fork_metrics/fork_state"
)

type StateMetricsBase struct {
	CurrentState fork_state.ForkStateContentBase
	PrevState    fork_state.ForkStateContentBase
	NextState    fork_state.ForkStateContentBase
}

func (p StateMetricsBase) PrevEpochReward(valIdx uint64) int64 {
	return int64(p.CurrentState.Balances[valIdx] - p.PrevState.Balances[valIdx])
}

func (p StateMetricsBase) GetAttSlot(valIdx uint64) int64 {

	return int64(p.PrevState.EpochStructs.ValidatorAttSlot[valIdx])
}

func (p StateMetricsBase) GetAttInclusionSlot(valIdx uint64) int64 {

	for i, item := range p.CurrentState.ValAttestationInclusion[valIdx].AttestedSlot {
		// we are looking for a vote to the previous epoch
		if item >= p.PrevState.Slot+1-fork_state.SLOTS_PER_EPOCH &&
			item <= p.PrevState.Slot {
			return int64(p.CurrentState.ValAttestationInclusion[valIdx].InclusionSlot[i])
		}
	}
	return -1
}

type StateMetrics interface {
	GetMetricsBase() StateMetricsBase
	GetMaxReward(valIdx uint64) (ValidatorSepRewards, error)
}

func StateMetricsByForkVersion(nextBstate fork_state.ForkStateContentBase, bstate fork_state.ForkStateContentBase, prevBstate fork_state.ForkStateContentBase, iApi *http.Service) (StateMetrics, error) {
	switch bstate.Version {

	case spec.DataVersionPhase0:
		return NewPhase0Spec(nextBstate, bstate, prevBstate), nil

	case spec.DataVersionAltair:
		return NewAltairSpec(nextBstate, bstate, prevBstate), nil

	case spec.DataVersionBellatrix:
		return NewAltairSpec(nextBstate, bstate, prevBstate), nil
	default:
		return nil, fmt.Errorf("could not figure out the Beacon State Fork Version: %s", bstate.Version)
	}
}

type ValidatorSepRewards struct {
	Attestation     float64
	InclusionDelay  float64
	FlagIndex       float64
	SyncCommittee   float64
	MaxReward       float64
	BaseReward      float64
	InSyncCommittee bool
	ProposerSlot    int64
}
