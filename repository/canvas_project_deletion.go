package repository

import (
	"errors"
	"strings"

	"github.com/tigerowo/infinite-canvas/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var errUserCanvasDataChanged = errors.New("user canvas data changed")

func saveUserCanvasProjectDeletions(
	db *gorm.DB,
	userID string,
	projectIDs []string,
	deletedAt string,
) error {
	projectIDs = uniqueTrimmedValues(projectIDs...)
	if len(projectIDs) == 0 {
		return nil
	}

	records := make([]model.CanvasProjectDeletion, 0, len(projectIDs))
	for _, projectID := range projectIDs {
		records = append(records, model.CanvasProjectDeletion{
			UserID:    strings.TrimSpace(userID),
			ProjectID: projectID,
			DeletedAt: deletedAt,
		})
	}

	return db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "user_id"},
			{Name: "project_id"},
		},
		DoUpdates: clause.AssignmentColumns([]string{"deleted_at"}),
	}).Create(&records).Error
}

func SoftDeleteAndCompareAndSwapUserCanvasData(
	userID string,
	projectIDs []string,
	deletedAt string,
	expectedUpdatedAt string,
	expectedCanvasData string,
	nextCanvasData string,
) (bool, error) {
	db, err := DB()
	if err != nil {
		return false, err
	}
	updated := false
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := saveUserCanvasProjectDeletions(
			tx,
			userID,
			projectIDs,
			deletedAt,
		); err != nil {
			return err
		}
		var err error
		updated, err = compareAndSwapUserCanvasData(
			tx,
			userID,
			expectedUpdatedAt,
			expectedCanvasData,
			nextCanvasData,
		)
		if err != nil {
			return err
		}
		if !updated {
			return errUserCanvasDataChanged
		}
		return nil
	})
	if errors.Is(err, errUserCanvasDataChanged) {
		return false, nil
	}
	return updated, err
}

func UserDeletedCanvasProjectIDs(
	userID string,
) (map[string]bool, error) {
	db, err := DB()
	if err != nil {
		return nil, err
	}
	var ids []string
	if err := db.Model(&model.CanvasProjectDeletion{}).
		Where("user_id = ?", strings.TrimSpace(userID)).
		Pluck("project_id", &ids).Error; err != nil {
		return nil, err
	}
	deleted := make(map[string]bool, len(ids))
	for _, id := range ids {
		deleted[id] = true
	}
	return deleted, nil
}

func CleanupDeletedCanvasProjects(before string) error {
	db, err := DB()
	if err != nil {
		return err
	}
	return db.Where("deleted_at < ?", before).
		Delete(&model.CanvasProjectDeletion{}).Error
}
