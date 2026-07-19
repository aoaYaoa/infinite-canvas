package repository

import (
	"errors"
	"time"

	"github.com/tigerowo/infinite-canvas/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func userConfigTimestamp() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

func GetUserConfig(userID string) (model.UserConfig, bool, error) {
	db, err := DB()
	if err != nil {
		return model.UserConfig{}, false, err
	}

	var config model.UserConfig
	err = db.First(&config, "user_id = ?", userID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.UserConfig{}, false, nil
	}
	if err != nil {
		return model.UserConfig{}, false, err
	}
	return config, true, nil
}

func SaveUserConfig(config model.UserConfig) (model.UserConfig, error) {
	db, err := DB()
	if err != nil {
		return config, err
	}

	config.UpdatedAt = userConfigTimestamp()
	return config, db.Omit("CanvasData").Save(&config).Error
}

func EnsureUserConfig(userID string) error {
	db, err := DB()
	if err != nil {
		return err
	}

	current := userConfigTimestamp()
	return db.Clauses(clause.OnConflict{DoNothing: true}).
		Create(&model.UserConfig{
			UserID:    userID,
			CreatedAt: current,
			UpdatedAt: current,
		}).Error
}

func CompareAndSwapUserCanvasData(
	userID string,
	expectedUpdatedAt string,
	expectedCanvasData string,
	nextCanvasData string,
) (bool, error) {
	db, err := DB()
	if err != nil {
		return false, err
	}
	return compareAndSwapUserCanvasData(
		db,
		userID,
		expectedUpdatedAt,
		expectedCanvasData,
		nextCanvasData,
	)
}

func compareAndSwapUserCanvasData(
	db *gorm.DB,
	userID string,
	expectedUpdatedAt string,
	expectedCanvasData string,
	nextCanvasData string,
) (bool, error) {
	result := db.Model(&model.UserConfig{}).
		Where(
			"user_id = ? AND COALESCE(updated_at, '') = ? AND COALESCE(canvas_data, '') = ?",
			userID,
			expectedUpdatedAt,
			expectedCanvasData,
		).
		Updates(map[string]any{
			"canvas_data": nextCanvasData,
			"updated_at":  userConfigTimestamp(),
		})

	return result.RowsAffected == 1, result.Error
}
