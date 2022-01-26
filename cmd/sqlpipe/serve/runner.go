package serve

import (
	"fmt"
	"sync"
	"time"

	"github.com/calmitchell617/sqlpipe/internal/engine"
	"github.com/calmitchell617/sqlpipe/pkg"
)

func (app *application) toDoScanner() {
	// This loops forever, looking for transfer requests to fulfill in the db
	var wg sync.WaitGroup

	for {
		wg.Wait()
		time.Sleep(time.Second)

		queuedTransfers, err := app.models.Transfers.GetQueued()
		if err != nil {
			app.logger.PrintError(err, nil)
		}

		queuedQueries, err := app.models.Queries.GetQueued()
		if err != nil {
			app.logger.PrintError(err, nil)
		}

		for _, transfer := range queuedTransfers {
			if app.numLocalActiveTransfers < 10 {
				wg.Add(1)
				pkg.Background(func() {
					defer wg.Done()
					transfer.Status = "active"
					err = app.models.Transfers.Update(transfer)
					if err != nil {
						app.logger.PrintError(fmt.Errorf("%s", err), nil)
						return
					}
					app.logger.PrintInfo(
						"now running a transfer",
						map[string]string{
							"ID":           fmt.Sprint(transfer.ID),
							"CreatedAt":    humanDate(transfer.CreatedAt),
							"SourceID":     fmt.Sprint(transfer.SourceID),
							"TargetID":     fmt.Sprint(transfer.TargetID),
							"Query":        transfer.Query,
							"TargetSchema": transfer.TargetSchema,
							"TargetTable":  transfer.TargetTable,
							"Overwrite":    fmt.Sprint(transfer.Overwrite),
							"Status":       transfer.Status,
						},
					)
					errProperties, err := engine.RunTransfer(transfer)
					if err != nil {
						app.logger.PrintError(err, errProperties)
						transfer.Status = "error"
						transfer.Error = err.Error()
						transfer.ErrorProperties = fmt.Sprint(errProperties)
						transfer.StoppedAt = time.Now()

						err = app.models.Transfers.Update(transfer)
						if err != nil {
							app.logger.PrintError(err, errProperties)
						}
						return
					}

					transfer.Status = "complete"
					transfer.StoppedAt = time.Now()
					err = app.models.Transfers.Update(transfer)
					if err != nil {
						app.logger.PrintError(err, errProperties)
					}
				})
			}
		}

		for _, query := range queuedQueries {
			pkg.Background(func() {
				query.Status = "active"
				err = app.models.Queries.Update(query)
				if err != nil {
					app.logger.PrintError(fmt.Errorf("%s", err), nil)
				}
				app.logger.PrintInfo(
					"now running a query",
					map[string]string{
						"ID":           fmt.Sprint(query.ID),
						"CreatedAt":    humanDate(query.CreatedAt),
						"ConnectionID": fmt.Sprint(query.ConnectionID),
						"Query":        query.Query,
						"Status":       query.Status,
					},
				)
				errProperties, err := engine.RunQuery(query)
				if err != nil {
					app.logger.PrintError(err, errProperties)
					query.Status = "error"
					query.Error = err.Error()
					query.ErrorProperties = fmt.Sprint(errProperties)
					query.StoppedAt = time.Now()
					err = app.models.Queries.Update(query)
					if err != nil {
						app.logger.PrintError(err, errProperties)
					}
					return
				}

				query.Status = "complete"
				query.StoppedAt = time.Now()
				err = app.models.Queries.Update(query)
				if err != nil {
					app.logger.PrintError(err, errProperties)
				}
			})
		}
	}
}
