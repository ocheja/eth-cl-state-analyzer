package postgresql

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Static postgres queries, for each modification in the tables, the table needs to be reseted
var (
	// wlogrus associated with the postgres db
	PsqlType = "postgres-db"
	wlog     = logrus.WithField(
		"module", PsqlType,
	)
	MAX_BATCH_QUEUE       = 700
	MAX_EPOCH_BATCH_QUEUE = 1
)

type PostgresDBService struct {
	// Control Variables
	ctx           context.Context
	cancel        context.CancelFunc
	connectionUrl string // the url might not be necessary (better to remove it?¿)
	psqlPool      *pgxpool.Pool

	WriteChan        chan pgx.Batch
	doneTasks        chan interface{}
	endProcess       int32
	FinishSignalChan chan struct{}
	workerNum        int
	// Network DB Model
}

// Connect to the PostgreSQL Database and get the multithread-proof connection
// from the given url-composed credentials
func ConnectToDB(ctx context.Context, url string, chanLength int, workerNum int) (*PostgresDBService, error) {
	mainCtx, cancel := context.WithCancel(ctx)
	// spliting the url to don't share any confidential information on wlogs
	wlog.Infof("Conneting to postgres DB %s", url)
	if strings.Contains(url, "@") {
		wlog.Debugf("Connecting to PostgresDB at %s", strings.Split(url, "@")[1])
	}
	psqlPool, err := pgxpool.Connect(mainCtx, url)
	if err != nil {
		return nil, err
	}
	if strings.Contains(url, "@") {
		wlog.Infof("PostgresDB %s succesfully connected", strings.Split(url, "@")[1])
	}
	// filter the type of network that we are filtering

	psqlDB := &PostgresDBService{
		ctx:              mainCtx,
		cancel:           cancel,
		connectionUrl:    url,
		psqlPool:         psqlPool,
		WriteChan:        make(chan pgx.Batch, chanLength),
		doneTasks:        make(chan interface{}, 1),
		endProcess:       0,
		FinishSignalChan: make(chan struct{}, 1),
		workerNum:        workerNum,
	}
	// init the psql db
	err = psqlDB.init(ctx, psqlDB.psqlPool)
	if err != nil {
		return psqlDB, errors.Wrap(err, "error initializing the tables of the psqldb")
	}
	go psqlDB.runWriters()
	return psqlDB, err
}

// Close the connection with the PostgreSQL
func (p *PostgresDBService) Close() {
}

func (p *PostgresDBService) init(ctx context.Context, pool *pgxpool.Pool) error {
	// create the tables
	err := p.createRewardsTable()
	if err != nil {
		return err
	}

	err = p.createEpochMetricsTable()
	if err != nil {
		return err
	}

	err = p.createProposerDutiesTable()
	if err != nil {
		return err
	}

	err = p.createBlockMetricsTable()
	if err != nil {
		return err
	}

	err = p.createStatusTable()
	if err != nil {
		return err
	}

	err = p.createPoolsTable()
	if err != nil {
		return err
	}

	err = p.createWithdrawalsTable()
	if err != nil {
		return err
	}

	err = p.createValidatorSummaryTable()
	if err != nil {
		return err
	}

	return nil
}

func (p *PostgresDBService) DoneTasks() {
	atomic.AddInt32(&p.endProcess, int32(1))
	wlog.Infof("Received finish signal")
}

func (p *PostgresDBService) runWriters() {
	var wgDBWriters sync.WaitGroup
	wlog.Info("Launching Beacon State Writers")
	wlog.Infof("Launching %d Beacon State Writers", p.workerNum)
	for i := 0; i < p.workerNum; i++ {
		wgDBWriters.Add(1)
		go func(dbWriterID int) {
			defer wgDBWriters.Done()
			wlogWriter := wlog.WithField("DBWriter", dbWriterID)
		loop:
			for {

				if p.endProcess >= 1 && len(p.WriteChan) == 0 {
					wlogWriter.Info("finish detected, closing persister")
					break loop
				}

				select {
				case task := <-p.WriteChan:
					wlogWriter.Debugf("Received new write task")
					err := p.ExecuteBatch(task)
					if err != nil {
						wlogWriter.Errorf("Error processing batch", err.Error())
					}

				case <-p.ctx.Done():
					wlogWriter.Info("shutdown detected, closing persister")
					break loop
				default:
				}

			}
			wlogWriter.Debugf("DB Writer finished...")

		}(i)
	}

	wgDBWriters.Wait()
	p.FinishSignalChan <- struct{}{}

}

type WriteTask struct {
	QueryID  int
	ModelObj interface{}
}

func (p PostgresDBService) ExecuteBatch(batch pgx.Batch) error {

	// for i := 0; i < batch.Len(); i++ {
	// 	wlog.Tracef("Executing SQL: %s", batch.items[i])
	// }
	snapshot := time.Now()
	tx, err := p.psqlPool.Begin(p.ctx)
	if err != nil {
		panic(err)
	}

	batchResults := tx.SendBatch(p.ctx, &batch)

	var qerr error
	var rows pgx.Rows
	for qerr == nil {
		rows, qerr = batchResults.Query()
		rows.Close()
	}
	if qerr.Error() != "no result" {
		wlog.Errorf(qerr.Error())
	}

	// p.MonitorStruct.AddDBWrite(time.Since(snapshot).Seconds())
	wlog.Debugf("Batch process time: %f, batch size: %d", time.Since(snapshot).Seconds(), batch.Len())

	return tx.Commit(p.ctx)

}
