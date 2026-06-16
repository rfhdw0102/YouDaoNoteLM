package service

import (
	"YoudaoNoteLm/internal/model/entity"
	"mime/multipart"
)

// SearchResultItem 搜索结果项（用于导入时保留标题）
type SearchResultItem struct {
	Title string // 标题
	URL   string // URL
}

// ImporterService 导入服务接口
type ImporterService interface {
	ImportFile(userID, notebookID uint, file *multipart.FileHeader) (*entity.Source, error)
	// PreviewAudio 异步音频转写：上传文件后立即返回 previewID，后台执行 ASR 转写
	PreviewAudio(userID, notebookID uint, file *multipart.FileHeader) (previewID string, fileName string, err error)
	// GetAudioPreviewStatus 查询音频预览状态（前端轮询用）
	GetAudioPreviewStatus(userID uint, previewID string) (interface{}, error)
	ConfirmAudio(userID uint, previewID string, editedContent *string) (*entity.Source, error)
	// ImportSearchResults 批量导入搜索结果，返回任务 ID 和创建的 Source ID 列表
	ImportSearchResults(userID, notebookID uint, items []SearchResultItem) (taskID string, sourceIDs []uint, err error)
	GetImportTask(taskID string) (interface{}, error)
	DeleteImportTask(taskID string) error // 删除导入任务
}
