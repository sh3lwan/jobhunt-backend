package schedular

import (
	"context"
	"fmt"

	"github.com/robfig/cron/v3"
	"github.com/sh3lwan/jobhunter/internal/repository"
)

func StartSchedular(q *repository.Queries, ctx context.Context) {
	fmt.Println("Cron Job Started...")
	c := cron.New()

	//	c.AddFunc("*/30 * * * *", func() {
	//		// scrappe remotive jobs + save to db
	//		rmtv := services.NewRemotiveService()
	//
	//		cvService := services.NewCVService(q, nil)
	//
	//		skills, err := cvService.GetSkills(ctx)
	//
	//		jobs, err := rmtv.CollectJobs(q, ctx, skills)
	//
	//		if err != nil {
	//			fmt.Printf("Error @ Schedular - Remoative Collect: %v", err.Error())
	//		}
	//
	//		dbjobService := services.NewDBJobService(q)
	//		err = dbjobService.SaveJobs(q, ctx, jobs)
	//
	//		if err != nil {
	//			fmt.Println("Error @ Schedular: ", err.Error())
	//		}
	//	})

	//	c.AddFunc("*/5 * * * *", func() {
	//
	//		//mbeddingService := services.NewEmbeddingService(nil)
	//
	//		//count, err := q.InsertAllMissingCvJobPairs(ctx)
	//
	//		//	if err != nil {
	//		//		log.Printf("Error @ Schedular - InsertAllMissingCvJobPairs: %v", err.Error())
	//		//	}
	//
	//		//	log.Printf("Missing CV-Job Pairs Affected: %d", count)
	//	})

	c.Start()
}
