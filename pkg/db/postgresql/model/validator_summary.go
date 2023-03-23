package model

// Postgres intregration variables
var (
	CreateValidatorSummaryTable = `
	CREATE TABLE IF NOT EXISTS t_validator_summary(
		f_val_idx INT PRIMARY KEY,
		f_active_since INT,
		f_number_epochs INT,
		f_balance_eth REAL,
		f_acc_reward INT,
		f_acc_max_reward INT,
		f_acc_sync_committee INT,
		f_acc_missing_source INT,
		f_acc_missing_target INT, 
		f_acc_missing_head INT);`

	UpsertValidatorSummary = `
	INSERT INTO t_validator_summary(
		f_val_idx,
		f_active_since,
		f_number_epochs,
		f_balance_eth,
		f_acc_reward,
		f_acc_max_reward,
		f_acc_sync_committee,
		f_acc_missing_source,
		f_acc_missing_target,
		f_acc_missing_head)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	ON CONFLICT ON CONSTRAINT t_validator_summary_pkey
		DO 
			UPDATE SET 
				f_active_since = excluded.f_active_since, 
				f_number_epochs = excluded.f_number_epochs, 
				f_balance_eth = excluded.f_balance_eth,
				f_acc_reward = excluded.f_acc_reward,
				f_acc_max_reward = excluded.f_acc_max_reward,
				f_acc_sync_committee = excluded.f_acc_sync_committee,
				f_acc_missing_source = excluded.f_acc_missing_source,
				f_acc_missing_target = excluded.f_acc_missing_target,
				f_acc_missing_head = excluded.f_acc_missing_head;
	`

	SelectValidatorSummaries = `
	SELECT *
	FROM t_validator_summary`
)

type ValidatorSummary struct {
	ValIdx           uint64  // validator index
	ActiveSince      uint64  // epoch at which the validator became active
	NumberEpochs     uint64  // numbers of epoch we measured this validator
	CurrentBalance   float32 // balance at the last measured epoch
	AccReward        int64   // Cumulative Reward
	AccMaxReward     int64   // Cumulative MaxReward
	AccSyncCommittee int64   // number of epochs as a sync committee participant
	SumMissedSource  uint64  // Number of missed source attestations
	SumMissedTarget  uint64  // Number of missed target attestations
	SumMissedHead    uint64  // Number of missed head attestations
}

func NewValidatorSummary() ValidatorSummary {
	return ValidatorSummary{}
}

func NewEmptyValidatorSummary() ValidatorSummary {
	return ValidatorSummary{}
}
