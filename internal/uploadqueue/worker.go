package uploadqueue

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"

	"buh/internal/importer"
)

type Worker struct {
	repo     *Repo
	importer *importer.Importer
}

func NewWorker(repo *Repo, imp *importer.Importer) *Worker {
	return &Worker{repo: repo, importer: imp}
}

// Run processes jobs from the queue until ctx is canceled.
// When ctx is canceled no new jobs are claimed, but the current job runs to completion.
func (w *Worker) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		job, ok, err := w.repo.Claim(context.Background())
		if err != nil {
			log.Printf("uploadqueue worker: claim error: %v", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
			}
			continue
		}
		if !ok {
			select {
			case <-ctx.Done():
				return
			case <-time.After(500 * time.Millisecond):
			}
			continue
		}

		w.process(job)
	}
}

func (w *Worker) process(job Job) {
	slipResults, er, err := w.importer.ProcessFileData(context.Background(), job.AccountantID, job.Filename, job.Data)
	if err != nil {
		if failErr := w.repo.Fail(context.Background(), job.ID, err.Error()); failErr != nil {
			log.Printf("uploadqueue worker: fail %s: %v", job.Filename, failErr)
		}
		return
	}

	result := JobResult{
		EntrepreneurName:  er.Entrepreneur.Name,
		IsNewEntrepreneur: er.IsNew,
	}
	if er.Entrepreneur.ID != uuid.Nil {
		result.EntrepreneurID = er.Entrepreneur.ID
	}
	for _, sr := range slipResults {
		switch sr.Status {
		case importer.StatusCreated:
			result.SlipsCreated++
		case importer.StatusUpdated:
			result.SlipsUpdated++
		}
	}

	if err := w.repo.Complete(context.Background(), job, result); err != nil {
		log.Printf("uploadqueue worker: complete %s: %v", job.Filename, err)
	}
}
