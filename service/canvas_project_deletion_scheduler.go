package service

import (
	"log"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/tigerowo/infinite-canvas/repository"
)

const canvasProjectDeletionCleanupCron = "0 3 * * *"

var (
	canvasProjectDeletionCron     *cron.Cron
	canvasProjectDeletionCronOnce sync.Once
)

func StartCanvasProjectDeletionCleanupScheduler() {
	canvasProjectDeletionCronOnce.Do(func() {
		canvasProjectDeletionCron = cron.New()
		if _, err := canvasProjectDeletionCron.AddFunc(
			canvasProjectDeletionCleanupCron,
			cleanupExpiredCanvasProjectDeletions,
		); err != nil {
			log.Printf(
				"add canvas project deletion cleanup cron failed err=%v",
				err,
			)
			return
		}
		canvasProjectDeletionCron.Start()
	})

	cleanupExpiredCanvasProjectDeletions()
}

func cleanupExpiredCanvasProjectDeletions() {
	if err := repository.CleanupDeletedCanvasProjects(
		time.Now().UTC().
			Add(-7 * 24 * time.Hour).
			Format(time.RFC3339Nano),
	); err != nil {
		log.Printf(
			"cleanup canvas project deletions failed err=%v",
			err,
		)
	}
}
