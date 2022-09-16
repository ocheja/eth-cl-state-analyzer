package fork_state

import (
	"fmt"

	"github.com/attestantio/go-eth2-client/http"
	"github.com/attestantio/go-eth2-client/spec"
	"github.com/cortze/eth2-state-analyzer/pkg/utils"
)

// This Wrapper is meant to include all necessary data from the Bellatrix Fork
func NewBellatrixState(bstate spec.VersionedBeaconState, iApi *http.Service) ForkStateContentBase {

	bellatrixObj := ForkStateContentBase{
		Version:       bstate.Version,
		Balances:      bstate.Bellatrix.Balances,
		Validators:    bstate.Bellatrix.Validators,
		EpochStructs:  NewEpochData(iApi, bstate.Bellatrix.Slot),
		Epoch:         utils.GetEpochFromSlot(bstate.Bellatrix.Slot),
		Slot:          bstate.Bellatrix.Slot,
		BlockRoots:    bstate.Bellatrix.StateRoots,
		SyncCommittee: *bstate.Bellatrix.CurrentSyncCommittee,
	}

	bellatrixObj.Setup()

	fmt.Printf("AttVals1: %d", bellatrixObj.NumAttestingVals)
	fmt.Printf("ActiveVals1: %d", bellatrixObj.NumActiveVals)
	ProcessAttestations(&bellatrixObj, bstate.Bellatrix.PreviousEpochParticipation)
	fmt.Printf("AttVals2: %d", bellatrixObj.NumAttestingVals)
	fmt.Printf("ActiveVals2: %d", bellatrixObj.NumActiveVals)

	return bellatrixObj
}
