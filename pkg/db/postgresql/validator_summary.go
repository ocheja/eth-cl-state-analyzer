package postgresql

import (
	"github.com/cortze/eth-cl-state-analyzer/pkg/db/postgresql/model"
	"github.com/pkg/errors"
)

func (p *PostgresDBService) createValidatorSummaryTable() error {
	// create the tables
	_, err := p.psqlPool.Exec(p.ctx, model.CreateValidatorSummaryTable)
	if err != nil {
		return errors.Wrap(err, "error creating validator summary table")
	}
	return nil
}

// in case the table did not exist
func (p *PostgresDBService) ObtainValidatorSummaries() ([]model.ValidatorSummary, error) {

	rows, err := p.psqlPool.Query(p.ctx, model.SelectValidatorSummaries)
	if err != nil {
		return nil, errors.Wrap(err, "error obtaining validator summaries from database")
	}
	tmpValIdx := 0
	tmpActiveSince := 0
	tmpNumberEpochs := 0
	tmpCurrentBalance := float32(0.0)
	tmpAccReward := 0
	tmpAccMaxReward := 0
	tmpAccSyncCommittee := 0
	tmpSumMissedSource := 0
	tmpSumMissedTarget := 0
	tmpSumMissedHead := 0

	validators := make([]model.ValidatorSummary, 0)

	for rows.Next() {
		err := rows.Scan(
			&tmpValIdx,
			&tmpActiveSince,
			&tmpNumberEpochs,
			&tmpCurrentBalance,
			&tmpAccReward,
			&tmpAccMaxReward,
			&tmpAccSyncCommittee,
			&tmpSumMissedSource,
			&tmpSumMissedTarget,
			&tmpSumMissedHead)
		if err != nil {
			return nil, errors.Wrap(err, "error obtaining single validator summary")
		}
		validators = append(validators, model.ValidatorSummary{
			ValIdx:           uint64(tmpValIdx),
			ActiveSince:      uint64(tmpActiveSince),
			NumberEpochs:     uint64(tmpNumberEpochs),
			CurrentBalance:   tmpCurrentBalance,
			AccReward:        int64(tmpAccReward),
			AccMaxReward:     int64(tmpAccMaxReward),
			AccSyncCommittee: int64(tmpAccSyncCommittee),
			SumMissedSource:  uint64(tmpSumMissedSource),
			SumMissedTarget:  uint64(tmpSumMissedTarget),
			SumMissedHead:    uint64(tmpSumMissedHead),
		})

	}
	err = rows.Err()
	if err != nil {
		return nil, errors.Wrap(err, "error finalizing validator summary read")
	}

	result := make([]model.ValidatorSummary, len(validators))

	for _, item := range validators {
		result[item.ValIdx] = item
	}

	return result, err
}
