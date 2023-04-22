package analyzer

import (
	"sync"
	"time"

	"github.com/attestantio/go-eth2-client/spec/phase0"
	"github.com/cortze/eth-cl-state-analyzer/pkg/spec"
	"github.com/cortze/eth-cl-state-analyzer/pkg/utils"
	"github.com/sirupsen/logrus"
)

func (s *StateAnalyzer) runWorker(wlog *logrus.Entry, wgWorkers *sync.WaitGroup) {
	defer wgWorkers.Done()
	ticker := time.NewTicker(utils.RoutineFlushTimeout)

loop:
	for {

		select {
		case valTask := <-s.valTaskChan:

			wlog.Debugf("task received for val %d - %d in epoch %d", valTask.ValIdxs[0], valTask.ValIdxs[len(valTask.ValIdxs)-1], valTask.StateMetricsObj.GetMetricsBase().CurrentState.Epoch)
			// Proccess State
			snapshot := time.Now()

			// batch metrics
			summaryMet := spec.PoolSummary{
				PoolName: valTask.PoolName,
				Epoch:    valTask.StateMetricsObj.GetMetricsBase().NextState.Epoch,
			}

			// process each validator
			for _, valIdx := range valTask.ValIdxs {

				if int(valIdx) >= len(valTask.StateMetricsObj.GetMetricsBase().NextState.Validators) {
					continue // validator is not in the chain yet
				}
				// get max reward at given epoch using the formulas
				maxRewards, err := valTask.StateMetricsObj.GetMaxReward(valIdx)

				if err != nil {
					log.Errorf("Error obtaining max reward: ", err.Error())
					continue
				}

				if valTask.Finalized {
					// Only update validator last status on Finalized
					// We will always receive higher epochs
					s.dbClient.Persist(spec.ValidatorLastStatus{
						ValIdx:         phase0.ValidatorIndex(valIdx),
						Epoch:          valTask.StateMetricsObj.GetMetricsBase().CurrentState.Epoch,
						CurrentBalance: valTask.StateMetricsObj.GetMetricsBase().NextState.Balances[valIdx],
						CurrentStatus:  maxRewards.Status,
					})
				}
				if s.metrics.ValidatorRewards { // only if flag is activated
					s.dbClient.Persist(maxRewards)
				}

				if s.metrics.PoolSummary && valTask.PoolName != "" {
					summaryMet.AddValidator(maxRewards)
				}

			}

			if s.metrics.PoolSummary && summaryMet.PoolName != "" {
				// only send summary batch in case pools were introduced by the user and we have a name to identify it

				wlog.Debugf("Sending pool summary batch (%s) to be stored...", summaryMet.PoolName)
				s.dbClient.Persist(summaryMet)
			}
			wlog.Debugf("Validator group processed, worker freed for next group. Took %f seconds", time.Since(snapshot).Seconds())

		case <-s.ctx.Done():
			log.Info("context has died, closing state worker routine")
			break loop
		case <-ticker.C:
			if s.processerFinished && len(s.valTaskChan) == 0 {
				break loop
			}
		}

	}
	wlog.Infof("Validator worker finished, no more tasks to process")
}
