package model

type CanvasProjectDeletion struct {
	UserID    string `json:"userId" gorm:"primaryKey"`
	ProjectID string `json:"projectId" gorm:"primaryKey"`
	DeletedAt string `json:"deletedAt" gorm:"index:idx_canvas_project_deletions_deleted_at"`
}
