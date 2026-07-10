package schedular

import (
	"context"
	"log"

	"github.com/robfig/cron/v3"
	"github.com/sh3lwan/jobhunter/internal/repository"
	"github.com/sh3lwan/jobhunter/internal/services"
)

// StartSchedular runs the background reconciliation loops:
//   - every 2 minutes: request embeddings for jobs that have none yet
//     (covers jobs inserted by jobscrapper or any other writer to the shared DB)
//   - every 2 minutes: LLM-rerank the strongest matches that lack a rerank
//   - every 10 minutes: materialize match scores for any cv/job pairs that
//     were missed by the per-message trigger in the Kafka consumer
func StartSchedular(q *repository.Queries, embeddingSvc *services.EmbeddingService, rerankSvc *services.RerankService, ctx context.Context) {
	log.Println("Scheduler started")

	c := cron.New()

	// Serialize rerank runs — each does multiple sequential LLM calls and a
	// slow tick must not overlap the next one.
	rerankBusy := make(chan struct{}, 1)

	c.AddFunc("*/2 * * * *", func() {
		select {
		case rerankBusy <- struct{}{}:
			defer func() { <-rerankBusy }()
		default:
			return
		}

		done, err := rerankSvc.RerankPending(ctx, 6)

		if err != nil {
			log.Printf("Scheduler - RerankPending: %v", err)
			return
		}

		if done > 0 {
			log.Printf("Scheduler - reranked %d matches", done)
		}
	})

	c.AddFunc("*/2 * * * *", func() {
		sent, err := embeddingSvc.RequestMissingJobEmbeddings(ctx, 50)

		if err != nil {
			log.Printf("Scheduler - RequestMissingJobEmbeddings: %v", err)
			return
		}

		if sent > 0 {
			log.Printf("Scheduler - requested embeddings for %d jobs", sent)
		}
	})

	c.AddFunc("*/10 * * * *", func() {
		count, err := q.InsertAllMissingCvJobPairs(ctx)

		if err != nil {
			log.Printf("Scheduler - InsertAllMissingCvJobPairs: %v", err)
			return
		}

		if count > 0 {
			log.Printf("Scheduler - computed %d new cv-job match pairs", count)
		}
	})

	c.Start()

	go func() {
		<-ctx.Done()
		c.Stop()
	}()
}
