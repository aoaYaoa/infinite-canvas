package service

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/tigerowo/infinite-canvas/model"
	"github.com/tigerowo/infinite-canvas/repository"
)

type UserConfigPayload struct {
	ModelConfig      json.RawMessage             `json:"modelConfig,omitempty"`
	StorageProvider  *StorageObjectProviderInput `json:"storageProvider,omitempty"`
	CanvasData       json.RawMessage             `json:"canvasData,omitempty"`
	ImageHistory     json.RawMessage             `json:"imageHistory,omitempty"`
	AssetData        json.RawMessage             `json:"assetData,omitempty"`
	SyncCapabilities map[string]bool             `json:"syncCapabilities,omitempty"`
}

type StorageObjectProviderInput struct {
	Enabled         *bool  `json:"enabled,omitempty"`
	Name            string `json:"name"`
	Type            string `json:"type"`
	Endpoint        string `json:"endpoint"`
	Region          string `json:"region"`
	Bucket          string `json:"bucket"`
	AccessKeyID     string `json:"accessKeyId"`
	SecretAccessKey string `json:"secretAccessKey"`
	PublicBaseURL   string `json:"publicBaseUrl"`
	PathPrefix      string `json:"pathPrefix"`
}

type userModelConfigInput struct {
	LocalChannels []userLocalModelChannelInput `json:"localChannels"`
}

type userLocalModelChannelInput struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	BaseURL string   `json:"baseUrl"`
	APIKey  string   `json:"apiKey"`
	Models  []string `json:"models"`
}

func SelectUserLocalModelChannelForModel(userID string, modelName string, channelID string) (model.ModelChannel, error) {
	userID = strings.TrimSpace(userID)
	modelName = strings.TrimSpace(modelName)
	channelID = strings.TrimSpace(channelID)
	if userID == "" {
		return model.ModelChannel{}, errors.New("请先登录")
	}
	if modelName == "" {
		return model.ModelChannel{}, errors.New("缺少模型名称")
	}
	if channelID == "" {
		return model.ModelChannel{}, errors.New("缺少模型渠道")
	}
	config, ok, err := repository.GetUserConfig(userID)
	if err != nil {
		return model.ModelChannel{}, err
	}
	if !ok || strings.TrimSpace(config.ModelConfig) == "" {
		return model.ModelChannel{}, errors.New("本地渠道不存在")
	}
	var modelConfig userModelConfigInput
	if err := json.Unmarshal([]byte(config.ModelConfig), &modelConfig); err != nil {
		return model.ModelChannel{}, err
	}
	for _, channel := range modelConfig.LocalChannels {
		if strings.TrimSpace(channel.ID) != channelID {
			continue
		}
		baseURL := strings.TrimSpace(channel.BaseURL)
		apiKey := strings.TrimSpace(channel.APIKey)
		if baseURL == "" || apiKey == "" {
			return model.ModelChannel{}, errors.New("本地渠道配置不完整")
		}
		models := userLocalChannelModels(channel.Models)
		if len(models) > 0 && !userLocalChannelHasModel(models, modelName) {
			return model.ModelChannel{}, errors.New("本地渠道不支持该模型")
		}
		return model.ModelChannel{
			ID:      channelID,
			Name:    firstVideoTaskValue(strings.TrimSpace(channel.Name), "本地直连"),
			BaseURL: baseURL,
			APIKey:  apiKey,
			Models:  models,
			Weight:  1,
			Timeout: 600,
			Enabled: true,
		}, nil
	}
	return model.ModelChannel{}, errors.New("本地渠道不存在")
}

func userLocalChannelModels(models []string) []string {
	result := make([]string, 0, len(models))
	seen := map[string]bool{}
	for _, item := range models {
		modelName := strings.TrimSpace(item)
		if modelName == "" || seen[modelName] {
			continue
		}
		result = append(result, modelName)
		seen[modelName] = true
	}
	return result
}

func userLocalChannelHasModel(models []string, modelName string) bool {
	for _, item := range models {
		if strings.EqualFold(strings.TrimSpace(item), modelName) {
			return true
		}
	}
	return false
}

func CurrentUserConfig(ctx context.Context) (UserConfigPayload, error) {
	user, ok := UserFromContext(ctx)
	if !ok || user.ID == "" {
		return UserConfigPayload{}, errors.New("请先登录")
	}
	config, ok, err := repository.GetUserConfig(user.ID)
	if err != nil {
		return UserConfigPayload{}, err
	}
	result := UserConfigPayload{
		SyncCapabilities: map[string]bool{
			"userData":  true,
			"workflows": true,
			"assets":    true,
		},
	}
	if !ok {
		return result, nil
	}
	if strings.TrimSpace(config.ModelConfig) != "" {
		result.ModelConfig = json.RawMessage(config.ModelConfig)
	}
	if strings.TrimSpace(config.StorageProvider) != "" {
		var provider StorageObjectProviderInput
		if err := json.Unmarshal([]byte(config.StorageProvider), &provider); err == nil {
			result.StorageProvider = &provider
		}
	}
	if strings.TrimSpace(config.CanvasData) != "" {
		state, err := normalizeCanvasData(
			json.RawMessage(config.CanvasData),
		)
		if err != nil {
			return UserConfigPayload{}, err
		}
		merged, err := mergeUserCanvasData(user.ID, state)
		if err != nil {
			return UserConfigPayload{}, err
		}
		data, err := json.Marshal(merged)
		if err != nil {
			return UserConfigPayload{}, err
		}
		result.CanvasData = data
	}
	if strings.TrimSpace(config.ImageHistory) != "" {
		result.ImageHistory = json.RawMessage(config.ImageHistory)
	}
	if strings.TrimSpace(config.AssetData) != "" {
		result.AssetData = json.RawMessage(config.AssetData)
	}
	return result, nil
}

func SaveCurrentUserModelConfig(ctx context.Context, raw json.RawMessage) (UserConfigPayload, error) {
	user, ok := UserFromContext(ctx)
	if !ok || user.ID == "" {
		return UserConfigPayload{}, errors.New("请先登录")
	}
	config, _, err := repository.GetUserConfig(user.ID)
	if err != nil {
		return UserConfigPayload{}, err
	}
	current := now()
	if config.UserID == "" {
		config.UserID = user.ID
		config.CreatedAt = current
	}
	config.ModelConfig = string(raw)
	config.UpdatedAt = current
	if _, err := repository.SaveUserConfig(config); err != nil {
		return UserConfigPayload{}, err
	}
	return CurrentUserConfig(ctx)
}

type canvasDataState struct {
	Projects []json.RawMessage `json:"projects"`
}

type canvasProjectVersion struct {
	Raw       json.RawMessage
	ID        string
	UpdatedAt string
}

func normalizeCanvasData(raw json.RawMessage) (canvasDataState, error) {
	state := canvasDataState{
		Projects: []json.RawMessage{},
	}
	if strings.TrimSpace(string(raw)) == "" {
		return state, nil
	}
	if err := json.Unmarshal(raw, &state); err != nil {
		return state, err
	}
	if state.Projects == nil {
		state.Projects = []json.RawMessage{}
	}
	return state, nil
}

func mergeCanvasData(
	deletedProjectIDs map[string]bool,
	states ...canvasDataState,
) canvasDataState {
	projects := map[string]canvasProjectVersion{}
	for _, state := range states {
		for _, raw := range state.Projects {
			var metadata struct {
				ID        string `json:"id"`
				UpdatedAt string `json:"updatedAt"`
			}
			if json.Unmarshal(raw, &metadata) != nil {
				continue
			}
			metadata.ID = strings.TrimSpace(metadata.ID)
			if metadata.ID == "" || deletedProjectIDs[metadata.ID] {
				continue
			}
			current, exists := projects[metadata.ID]
			if !exists || metadata.UpdatedAt >= current.UpdatedAt {
				projects[metadata.ID] = canvasProjectVersion{
					Raw:       raw,
					ID:        metadata.ID,
					UpdatedAt: metadata.UpdatedAt,
				}
			}
		}
	}
	versions := make([]canvasProjectVersion, 0, len(projects))
	for _, project := range projects {
		versions = append(versions, project)
	}
	sort.Slice(versions, func(i, j int) bool {
		if versions[i].UpdatedAt == versions[j].UpdatedAt {
			return versions[i].ID < versions[j].ID
		}
		return versions[i].UpdatedAt > versions[j].UpdatedAt
	})
	result := canvasDataState{
		Projects: make([]json.RawMessage, 0, len(versions)),
	}
	for _, project := range versions {
		result.Projects = append(result.Projects, project.Raw)
	}
	return result
}

func mergeUserCanvasData(
	userID string,
	states ...canvasDataState,
) (canvasDataState, error) {
	deletedProjectIDs, err := repository.UserDeletedCanvasProjectIDs(
		userID,
	)
	if err != nil {
		return canvasDataState{}, err
	}
	return mergeCanvasData(deletedProjectIDs, states...), nil
}

func CurrentUserCanvasData(ctx context.Context) (json.RawMessage, error) {
	config, err := CurrentUserConfig(ctx)
	if err != nil {
		return nil, err
	}
	if len(config.CanvasData) == 0 {
		return json.RawMessage(`{"projects":[]}`), nil
	}
	return json.RawMessage(config.CanvasData), nil
}

func SaveCurrentUserCanvasData(ctx context.Context, raw json.RawMessage) (json.RawMessage, error) {
	user, ok := UserFromContext(ctx)
	if !ok || user.ID == "" {
		return nil, errors.New("请先登录")
	}
	incoming, err := normalizeCanvasData(raw)
	if err != nil {
		return nil, errors.New("画布数据格式无效")
	}

	for attempt := 0; attempt < 16; attempt++ {
		config, exists, err := repository.GetUserConfig(user.ID)
		if err != nil {
			return nil, err
		}
		if !exists {
			if err := repository.EnsureUserConfig(user.ID); err != nil {
				return nil, err
			}
			continue
		}
		stored, err := normalizeCanvasData(json.RawMessage(config.CanvasData))
		if err != nil {
			return nil, err
		}
		merged, err := mergeUserCanvasData(user.ID, stored, incoming)
		if err != nil {
			return nil, err
		}
		encoded, err := json.Marshal(merged)
		if err != nil {
			return nil, err
		}
		updated, err := repository.CompareAndSwapUserCanvasData(
			user.ID,
			config.UpdatedAt,
			config.CanvasData,
			string(encoded),
		)
		if err != nil {
			return nil, err
		}
		if updated {
			return json.RawMessage(encoded), nil
		}
	}
	return nil, errors.New("画布数据同步冲突，请重试")
}

func DeleteCurrentUserCanvasProjects(
	ctx context.Context,
	projectIDs []string,
) error {
	user, ok := UserFromContext(ctx)
	if !ok || user.ID == "" {
		return errors.New("请先登录")
	}
	ids := make([]string, 0, len(projectIDs))
	seen := map[string]bool{}
	for _, projectID := range projectIDs {
		projectID = strings.TrimSpace(projectID)
		if projectID == "" || seen[projectID] {
			continue
		}
		seen[projectID] = true
		ids = append(ids, projectID)
	}
	if len(ids) == 0 {
		return errors.New("画布项目参数无效")
	}
	deletedAt := time.Now().UTC().Format(time.RFC3339Nano)
	for attempt := 0; attempt < 16; attempt++ {
		config, exists, err := repository.GetUserConfig(user.ID)
		if err != nil {
			return err
		}
		if !exists {
			if err := repository.EnsureUserConfig(user.ID); err != nil {
				return err
			}
			continue
		}
		stored, err := normalizeCanvasData(
			json.RawMessage(config.CanvasData),
		)
		if err != nil {
			return err
		}
		deletedProjectIDs, err := repository.UserDeletedCanvasProjectIDs(
			user.ID,
		)
		if err != nil {
			return err
		}
		for _, projectID := range ids {
			deletedProjectIDs[projectID] = true
		}
		encoded, err := json.Marshal(
			mergeCanvasData(deletedProjectIDs, stored),
		)
		if err != nil {
			return err
		}
		updated, err := repository.SoftDeleteAndCompareAndSwapUserCanvasData(
			user.ID,
			ids,
			deletedAt,
			config.UpdatedAt,
			config.CanvasData,
			string(encoded),
		)
		if err != nil {
			return err
		}
		if updated {
			return nil
		}
	}
	return errors.New("画布数据同步冲突，请重试")
}

func CurrentUserImageHistory(ctx context.Context) (json.RawMessage, error) {
	config, err := currentUserConfig(ctx)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(config.ImageHistory) == "" {
		return json.RawMessage(`{"logs":[],"categories":[]}`), nil
	}
	return json.RawMessage(config.ImageHistory), nil
}

func SaveCurrentUserImageHistory(ctx context.Context, raw json.RawMessage) (json.RawMessage, error) {
	config, err := saveCurrentUserConfigField(ctx, func(config *model.UserConfig) {
		config.ImageHistory = string(raw)
	})
	if err != nil {
		return nil, err
	}
	return json.RawMessage(config.ImageHistory), nil
}

func CurrentUserAssetData(ctx context.Context) (json.RawMessage, error) {
	config, err := currentUserConfig(ctx)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(config.AssetData) == "" {
		return json.RawMessage(`{"assets":[]}`), nil
	}
	return json.RawMessage(config.AssetData), nil
}

func SaveCurrentUserAssetData(ctx context.Context, raw json.RawMessage) (json.RawMessage, error) {
	config, err := saveCurrentUserConfigField(ctx, func(config *model.UserConfig) {
		config.AssetData = string(raw)
	})
	if err != nil {
		return nil, err
	}
	return json.RawMessage(config.AssetData), nil
}

func currentUserConfig(ctx context.Context) (model.UserConfig, error) {
	user, ok := UserFromContext(ctx)
	if !ok || user.ID == "" {
		return model.UserConfig{}, errors.New("请先登录")
	}
	config, _, err := repository.GetUserConfig(user.ID)
	if err != nil {
		return model.UserConfig{}, err
	}
	if config.UserID == "" {
		config.UserID = user.ID
	}
	return config, nil
}

func saveCurrentUserConfigField(ctx context.Context, patch func(config *model.UserConfig)) (model.UserConfig, error) {
	user, ok := UserFromContext(ctx)
	if !ok || user.ID == "" {
		return model.UserConfig{}, errors.New("请先登录")
	}
	config, _, err := repository.GetUserConfig(user.ID)
	if err != nil {
		return model.UserConfig{}, err
	}
	current := now()
	if config.UserID == "" {
		config.UserID = user.ID
		config.CreatedAt = current
	}
	patch(&config)
	config.UpdatedAt = current
	return repository.SaveUserConfig(config)
}
