package state

import (
	"github.com/attestantio/go-eth2-client/spec/phase0"
	"github.com/cortze/eth-cl-state-analyzer/pkg/db/postgresql/model"
)

type ValidatorSummaryMetrics struct {
	Summaries []model.ValidatorSummary
}

func (v *ValidatorSummaryMetrics) UpdateSingleValidatorSummary(
	valIdx uint64,
	validator phase0.Validator,
	rewards model.ValidatorRewards) {

	item := v.Summaries[valIdx]

	item.ActiveSince = uint64(validator.ActivationEpoch)
	item.AccReward = item.AccReward + rewards.Reward
	item.AccMaxReward = item.AccMaxReward + int64(rewards.MaxReward)
	item.CurrentBalance = rewards.ValidatorBalance
	item.NumberEpochs += 1

	if rewards.InSyncCommittee {
		item.AccSyncCommittee = item.AccSyncCommittee + 1
	}

	if rewards.MissingSource {
		item.SumMissedSource = item.SumMissedSource + 1
	}
	if rewards.MissingTarget {
		item.SumMissedTarget = item.SumMissedTarget + 1
	}
	if rewards.MissingHead {
		item.SumMissedHead = item.SumMissedHead + 1
	}

	v.Summaries[valIdx] = item
}

func (v *ValidatorSummaryMetrics) AddMissingIndexes(newLen uint64) {
	for i := len(v.Summaries); i < int(newLen); i++ {
		v.Summaries = append(v.Summaries, model.ValidatorSummary{ValIdx: uint64(i)})
	}
}
