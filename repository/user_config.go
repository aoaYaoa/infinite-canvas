package repository

import "github.com/tigerowo/infinite-canvas/model"

func GetUserConfig(userID string) (model.UserConfig, bool, error) {
	db, err := DB()
	if err != nil {
		return model.UserConfig{}, false, err
	}
	var config model.UserConfig
	err = db.First(&config, "user_id = ?", userID).Error
	if err != nil {
		return model.UserConfig{}, false, nil
	}
	return config, true, nil
}

func SaveUserConfig(config model.UserConfig) (model.UserConfig, error) {
	db, err := DB()
	if err != nil {
		return config, err
	}
	return config, db.Save(&config).Error
}
