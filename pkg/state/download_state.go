package state

import (
	"fmt"
	"sync"
	"time"

	"github.com/cortze/eth2-state-analyzer/pkg/state_metrics/fork_state"
)

func (s *StateAnalyzer) runDownloadStates(wgDownload *sync.WaitGroup) {
	defer wgDownload.Done()
	log.Info("Launching Beacon State Requester")
	// loop over the list of slots that we need to analyze

	// We need three consecutive states to compute max rewards easier
	prevBState := fork_state.ForkStateContentBase{}
	bstate := fork_state.ForkStateContentBase{}
	nextBstate := fork_state.ForkStateContentBase{}
	ticker := time.NewTicker(minReqTime)
	for _, slot := range s.SlotRanges {

		select {
		case <-s.ctx.Done():
			log.Info("context has died, closing state requester routine")
			close(s.EpochTaskChan)
			return

		default:
			if s.finishDownload {
				log.Info("sudden shutdown detected, state downloader routine")
				close(s.EpochTaskChan)
				return
			}
			ticker.Reset(minReqTime)
			firstIteration := true
			secondIteration := true
			// make the state query
			log.Infof("requesting Beacon State from endpoint: slot %d", slot)

			// We need three states to calculate both, rewards and maxRewards

			if bstate.AttestingBalance != nil { // in case we already had a bstate (only false the first time)
				prevBState = bstate
				firstIteration = false
			}
			if nextBstate.AttestingBalance != nil { // in case we already had a nextBstate (only false the first and second time)
				bstate = nextBstate
				secondIteration = false
			}
			newState, err := s.cli.Api.BeaconState(s.ctx, fmt.Sprintf("%d", slot))
			if newState == nil {
				log.Errorf("Unable to retrieve Beacon State from the beacon node, closing requester routine. Nil State")
				return
			}
			if err != nil {
				// close the channel (to tell other routines to stop processing and end)
				log.Errorf("Unable to retrieve Beacon State from the beacon node, closing requester routine. %s", err.Error())
				return
			}
			if firstIteration {
				bstate, err = fork_state.GetCustomState(*newState, s.cli.Api)
				if err != nil {
					// close the channel (to tell other routines to stop processing and end)
					log.Errorf("Unable to retrieve Beacon State from the beacon node, closing requester routine. %s", err.Error())
					return
				}
			} else {
				nextBstate, err = fork_state.GetCustomState(*newState, s.cli.Api)
				if err != nil {
					// close the channel (to tell other routines to stop processing and end)
					log.Errorf("Unable to retrieve Beacon State from the beacon node, closing requester routine. %s", err.Error())
					return
				}
			}

			if !firstIteration && !secondIteration {
				// only execute tasks if it is not the first or second iteration iteration ==> we have three states

				epochTask := &EpochTask{
					ValIdxs:   s.ValidatorIndexes,
					NextState: nextBstate,
					State:     bstate,
					PrevState: prevBState,
					Finalized: false,
				}

				log.Debugf("sending task for slot: %d", epochTask.State.Slot)
				s.EpochTaskChan <- epochTask
			}
			// check if the min Request time has been completed (to avoid spaming the API)
			<-ticker.C

		}

	}

	log.Infof("All states for the slot ranges has been successfully retrieved, clossing go routine")
}

func (s *StateAnalyzer) runDownloadStatesFinalized(wgDownload *sync.WaitGroup) {
	defer wgDownload.Done()
	log.Info("Launching Beacon State Finalized Requester")
	// loop over the list of slots that we need to analyze
	prevBState := fork_state.ForkStateContentBase{}
	bstate := fork_state.ForkStateContentBase{}
	nextBstate := fork_state.ForkStateContentBase{}
	finalizedSlot := 0
	// tick every epoch, 384 seconds
	epochTicker := time.After(384 * time.Second)
	ticker := time.NewTicker(minReqTime)
	for {

		select {
		default:

			if s.finishDownload {
				log.Info("sudden shutdown detected, state downloader routine")
				close(s.EpochTaskChan)
				return
			}
		case <-s.ctx.Done():
			log.Info("context has died, closing state requester routine")
			close(s.EpochTaskChan)
			return

		case <-epochTicker:
			epochTicker = time.After(384 * time.Second)
			ticker.Reset(minReqTime)
			firstIteration := true
			secondIteration := true
			// make the state query
			log.Infof("requesting Beacon State from endpoint: finalized")
			if bstate.AttestingBalance != nil { // in case we already had a bstate (only false the first time)
				prevBState = bstate
				firstIteration = false
			}
			if nextBstate.AttestingBalance != nil { // in case we already had a nextBstate (only false the first time)
				bstate = nextBstate
				secondIteration = false
			}
			header, err := s.cli.Api.BeaconBlockHeader(s.ctx, "finalized")
			if err != nil {
				log.Errorf("Unable to retrieve Beacon State from the beacon node, closing finalized requester routine. %s", err.Error())
				continue
			}
			if int(header.Header.Message.Slot) == finalizedSlot {
				log.Infof("No new finalized state yet")
				continue
			}

			finalizedSlot = int(header.Header.Message.Slot)
			reqState := ((int(finalizedSlot/32) + 2) * 32) - 1
			log.Infof("New finalized state at slot: %d", finalizedSlot)
			newState, err := s.cli.Api.BeaconState(s.ctx, fmt.Sprintf("%d", reqState))
			if newState == nil {
				log.Errorf("Unable to retrieve Finalized Beacon State from the beacon node, closing requester routine. Nil State")
				continue
			}
			if err != nil {
				// close the channel (to tell other routines to stop processing and end)
				log.Errorf("Unable to retrieve Finalized Beacon State from the beacon node, closing requester routine. %s", err.Error())
				continue
			}
			if firstIteration {

				bstate, err = fork_state.GetCustomState(*newState, s.cli.Api)
				if err != nil {
					// close the channel (to tell other routines to stop processing and end)
					log.Errorf("Unable to retrieve Beacon State from the beacon node, closing requester routine. %s", err.Error())
					return
				}
			} else {
				nextBstate, err = fork_state.GetCustomState(*newState, s.cli.Api)
				if err != nil {
					// close the channel (to tell other routines to stop processing and end)
					log.Errorf("Unable to retrieve Beacon State from the beacon node, closing requester routine. %s", err.Error())
					return
				}
			}
			if !firstIteration && !secondIteration {
				// only execute tasks if it is not the first iteration or second iteration

				// we now only compose one single task that contains a list of validator indexes
				epochTask := &EpochTask{
					ValIdxs:   s.ValidatorIndexes,
					NextState: nextBstate,
					State:     bstate,
					PrevState: prevBState,
					Finalized: true,
				}

				log.Debugf("sending task for slot: %d", epochTask.State.Slot)
				s.EpochTaskChan <- epochTask
			}
			<-ticker.C
			// check if the min Request time has been completed (to avoid spaming the API)

		}

	}
}
