package state

import (
	"math"
	"sync"

	"github.com/attestantio/go-eth2-client/spec/altair"
	"github.com/attestantio/go-eth2-client/spec/phase0"
	"github.com/cortze/eth2-state-analyzer/pkg/db/postgresql"
	"github.com/cortze/eth2-state-analyzer/pkg/db/postgresql/model"
	"github.com/cortze/eth2-state-analyzer/pkg/state_metrics"
	"github.com/cortze/eth2-state-analyzer/pkg/state_metrics/fork_state"
	"github.com/jackc/pgx/v4"
)

func (s *StateAnalyzer) runProcessState(wgProcess *sync.WaitGroup, downloadFinishedFlag *bool) {
	defer wgProcess.Done()

	epochBatch := pgx.Batch{}
	log.Info("Launching Beacon State Pre-Processer")
loop:
	for {
		// in case the downloads have finished, and there are no more tasks to execute
		if *downloadFinishedFlag && len(s.EpochTaskChan) == 0 {
			log.Warn("the task channel has been closed, finishing epoch routine")
			if epochBatch.Len() == 0 {
				log.Debugf("Sending last epoch batch to be stored...")
				s.dbClient.WriteChan <- epochBatch
				epochBatch = pgx.Batch{}
			}

			break loop
		}

		select {
		case <-s.ctx.Done():
			log.Info("context has died, closing state processer routine")
			return

		case task, ok := <-s.EpochTaskChan:

			// check if the channel has been closed
			if !ok {
				log.Warn("the task channel has been closed, finishing epoch routine")
				return
			}
			log.Infof("epoch task received for slot %d, analyzing...", task.State.Slot)

			// returns the state in a custom struct for Phase0, Altair of Bellatrix
			stateMetrics, err := state_metrics.StateMetricsByForkVersion(task.NextState, task.State, task.PrevState, s.cli.Api)

			if err != nil {
				log.Errorf(err.Error())
				continue
			}

			log.Debugf("Creating validator batches for slot %d...", task.State.Slot)

			if len(task.ValIdxs) == 0 {
				// in case no validators provided, do all the active ones in the next epoch, take into account proposer and sync committee rewards
				task.ValIdxs = stateMetrics.GetMetricsBase().NextState.GetAllVals()
			} else {
				finalValidxs := make([]uint64, 0)
				for _, item := range task.ValIdxs {
					// check that validator number does not exceed the number of validators
					if int(item) >= len(stateMetrics.GetMetricsBase().NextState.Validators) {
						continue
					}
					// in case no validators provided, do all the active ones in the next epoch, take into account proposer and sync committee rewards
					if fork_state.IsActive(*stateMetrics.GetMetricsBase().NextState.Validators[item], phase0.Epoch(stateMetrics.GetMetricsBase().PrevState.Epoch)) {
						finalValidxs = append(finalValidxs, item)
					}
				}
				task.ValIdxs = finalValidxs
			}
			if task.NextState.Slot <= s.FinalSlot || task.Finalized {

				stepSize := int(math.Min(float64(MAX_VAL_BATCH_SIZE), float64(len(task.ValIdxs)/s.validatorWorkerNum)))
				stepSize = int(math.Max(float64(1), float64(stepSize))) // in case it is 0, at least set to 1
				for i := 0; i < len(task.ValIdxs); i += stepSize {
					endIndex := int(math.Min(float64(len(task.ValIdxs)), float64(i+stepSize)))
					// subslice does not include the endIndex
					valTask := &ValTask{
						ValIdxs:         task.ValIdxs[i:endIndex],
						StateMetricsObj: stateMetrics,
					}
					s.ValTaskChan <- valTask
				}
			}
			if task.PrevState.Slot >= s.InitSlot || task.Finalized { // only write epoch metrics inside the defined range

				log.Debugf("Writing epoch metrics to DB for slot %d...", task.State.Slot)
				// create a model to be inserted into the db, we only insert previous epoch metrics

				missedBlocks := stateMetrics.GetMetricsBase().PrevState.MissedBlocks
				log.Infof("Missed First Array: %+v", missedBlocks)
				// take into accoutn epoch transition
				nextMissedBlock := stateMetrics.GetMetricsBase().CurrentState.TrackPrevMissingBlock()
				if nextMissedBlock != 0 {
					missedBlocks = append(missedBlocks, nextMissedBlock)
				}
				epochDBRow := model.NewEpochMetrics(
					stateMetrics.GetMetricsBase().PrevState.Epoch,
					stateMetrics.GetMetricsBase().PrevState.Slot,
					uint64(len(stateMetrics.GetMetricsBase().CurrentState.PrevAttestations)),
					uint64(stateMetrics.GetMetricsBase().CurrentState.NumAttestingVals),
					uint64(stateMetrics.GetMetricsBase().PrevState.NumActiveVals),
					uint64(stateMetrics.GetMetricsBase().PrevState.TotalActiveRealBalance),
					uint64(stateMetrics.GetMetricsBase().CurrentState.AttestingBalance[altair.TimelyTargetFlagIndex]), // as per BEaconcha.in
					uint64(stateMetrics.GetMetricsBase().PrevState.TotalActiveBalance),
					uint64(stateMetrics.GetMetricsBase().CurrentState.GetMissingFlagCount(int(altair.TimelySourceFlagIndex))),
					uint64(stateMetrics.GetMetricsBase().CurrentState.GetMissingFlagCount(int(altair.TimelyTargetFlagIndex))),
					uint64(stateMetrics.GetMetricsBase().CurrentState.GetMissingFlagCount(int(altair.TimelyHeadFlagIndex))),
					missedBlocks)

				epochBatch.Queue(model.UpsertEpoch,
					epochDBRow.Epoch,
					epochDBRow.Slot,
					epochDBRow.PrevNumAttestations,
					epochDBRow.PrevNumAttValidators,
					epochDBRow.PrevNumValidators,
					epochDBRow.TotalBalance,
					epochDBRow.AttEffectiveBalance,
					epochDBRow.TotalEffectiveBalance,
					epochDBRow.MissingSource,
					epochDBRow.MissingTarget,
					epochDBRow.MissingHead)

				// Proposer Duties
				log.Infof("Missed Full Array: %+v", missedBlocks)
				for _, duty := range stateMetrics.GetMetricsBase().PrevState.EpochStructs.ProposerDuties {
					newDuty := model.NewProposerDuties(uint64(duty.ValidatorIndex), uint64(duty.Slot), true)
					for _, item := range missedBlocks {

						if newDuty.ProposerSlot == item { // we found the proposer slot in the missed blocks

							newDuty.Proposed = false
						}

					}
					log.Infof("Duty %d Proposed: %t", newDuty.ProposerSlot, newDuty.Proposed)
					epochBatch.Queue(model.InsertProposerDuty,
						newDuty.ValIdx,
						newDuty.ProposerSlot,
						newDuty.Proposed)
				}
			}

			// Flush the database batches
			if epochBatch.Len() >= postgresql.MAX_EPOCH_BATCH_QUEUE {
				s.dbClient.WriteChan <- epochBatch
				epochBatch = pgx.Batch{}
			}
		default:
		}

	}
	log.Infof("Pre process routine finished...")
}
