package analyzer

import (
	"fmt"
	"math"
	"sort"
	"strconv"

	"github.com/pkg/errors"
	"github.com/prysmaticlabs/go-bitfield"

	api "github.com/attestantio/go-eth2-client/api/v1"
	"github.com/attestantio/go-eth2-client/spec"
	"github.com/attestantio/go-eth2-client/spec/phase0"
)

const (
	participationRate   = 0.945 // about to calculate participation rate
	baseRewardFactor    = 64
	baseRewardsPerEpoch = 4
)

func GetValidatorBalance(bstate *spec.VersionedBeaconState, valIdx uint64) (uint64, error) {
	var balance uint64
	var err error
	switch bstate.Version {
	case spec.DataVersionPhase0:
		if uint64(len(bstate.Phase0.Balances)) < valIdx {
			err = fmt.Errorf("phase0 - validator index %d wasn't activated in slot %d", valIdx, bstate.Phase0.Slot)
		}
		balance = bstate.Phase0.Balances[valIdx]

	case spec.DataVersionAltair:
		if uint64(len(bstate.Altair.Balances)) < valIdx {
			err = fmt.Errorf("altair - validator index %d wasn't activated in slot %d", valIdx, bstate.Phase0.Slot)
		}
		balance = bstate.Altair.Balances[valIdx]

	case spec.DataVersionBellatrix:
		if uint64(len(bstate.Bellatrix.Balances)) < valIdx {
			err = fmt.Errorf("bellatrix - validator index %d wasn't activated in slot %d", valIdx, bstate.Phase0.Slot)
		}
		balance = bstate.Bellatrix.Balances[valIdx]
	default:

	}
	return balance, err
}

func GetParticipationRate(bstate *spec.VersionedBeaconState, s *StateAnalyzer, m map[string]bitfield.Bitlist) (uint64, error) {

	// participationRate := 0.85

	switch bstate.Version {
	case spec.DataVersionPhase0:
		currentSlot := bstate.Phase0.Slot
		currentEpoch := currentSlot / 32
		totalAttPreviousEpoch := 0
		totalAttCurrentEpoch := 0
		totalAttestingVals := 0

		previousAttestatons := bstate.Phase0.PreviousEpochAttestations
		// currentAttestations := bstate.Phase0.CurrentEpochAttestations
		doubleVotes := 0
		vals := bstate.Phase0.Validators

		// TODO: check validator active, slashed or exiting
		for _, item := range vals {
			if item.ActivationEligibilityEpoch < phase0.Epoch(currentEpoch) {
				totalAttestingVals += 1
			}
		}
		keys := make([]string, 0)
		for _, item := range previousAttestatons {
			slot := item.Data.Slot
			committeeIndex := item.Data.Index
			mapKey := strconv.Itoa(int(slot)) + "_" + strconv.Itoa(int(committeeIndex))

			resultBits := bitfield.NewBitlist(0)

			if val, ok := m[mapKey]; ok {
				// the committeeIndex for the given slot already had an aggregation
				// TODO: check error
				allZero, err := val.And(item.AggregationBits) // if the same position of the aggregations has a 1 in the same position, there was a double vote
				if err != nil {
					fmt.Println(err)
				}
				resultBitstmp, err := val.Or(item.AggregationBits) // to join all aggregation we do Or operation

				if allZero.Count() > 0 {
					// there was a double vote
					doubleVotes += int(allZero.Count())
				}
				if err == nil {
					resultBits = resultBitstmp
				} else {
					fmt.Println(err)
				}

			} else {
				// we had not received any aggregation for this committeeIndex at the given slot
				resultBits = item.AggregationBits
				keys = append(keys, mapKey)
			}
			m[mapKey] = resultBits
			attPreviousEpoch := int(item.AggregationBits.Count())
			totalAttPreviousEpoch += attPreviousEpoch // we are counting bits set to 1 aggregation by aggregation
			// if we do the And at the same committee we can catch the double votes
			// doing that the number of votes is less than the number of validators

		}

		sort.Strings(keys)
		numOfCommittees := 0
		numOfBits := uint64(0)
		numOf1Bits := 0

		for _, key := range keys {

			numOfBits += m[key].Len()
			numOf1Bits += int(m[key].Count())
			numOfCommittees += 1
		}

		fmt.Println("Current Epoch: ", currentEpoch)
		fmt.Println("Using Block at: ", currentSlot)
		fmt.Println("Attestations in the current Epoch: ", totalAttCurrentEpoch)
		fmt.Println("Total number of Validators: ", totalAttestingVals)

	case spec.DataVersionAltair:
		participationRate := bstate.Altair.PreviousEpochParticipation
		fmt.Println(participationRate)

	case spec.DataVersionBellatrix:
		participationRate := bstate.Bellatrix.PreviousEpochParticipation
		fmt.Println(participationRate)
	default:

	}

	return 0, nil
}

// https://kb.beaconcha.in/rewards-and-penalties
// https://consensys.net/blog/codefi/rewards-and-penalties-on-ethereum-20-phase-0/
// TODO: -would be nice to incorporate top the max value wheather there were 2-3 consecutive missed blocks afterwards
func GetMaxReward(valIdx uint64, totValStatus *map[phase0.ValidatorIndex]*api.Validator, totalActiveBalance uint64) (uint64, error) {
	// First iteration just taking 31/8*BaseReward as Max value
	// BaseReward = ( effectiveBalance * (BaseRewardFactor)/(BaseRewardsPerEpoch * sqrt(activeBalance)) )

	idx := phase0.ValidatorIndex(valIdx)

	valStatus, ok := (*totValStatus)[idx]
	if !ok {
		return 0, errors.New("")
	}
	// apply formula
	//baseReward := GetBaseReward(valStatus.Validator.EffectiveBalance, totalActiveBalance)
	maxReward := ((31.0 / 8.0) * participationRate * (float64(uint64(valStatus.Validator.EffectiveBalance)) * baseRewardFactor))
	maxReward = maxReward / (baseRewardsPerEpoch * math.Sqrt(float64(totalActiveBalance)))
	return uint64(maxReward), nil
}

// directly calculated on the MaxReward fucntion
func GetBaseReward(valEffectiveBalance phase0.Gwei, totalActiveBalance uint64) uint64 {
	// BaseReward = ( effectiveBalance * (BaseRewardFactor)/(BaseRewardsPerEpoch * sqrt(activeBalance)) )
	var baseReward uint64

	sqrt := math.Sqrt(float64(totalActiveBalance))

	denom := baseRewardsPerEpoch * sqrt

	bsRewrd := (float64(uint64(valEffectiveBalance)) * baseRewardFactor) / denom

	baseReward = uint64(bsRewrd)
	return baseReward
}
