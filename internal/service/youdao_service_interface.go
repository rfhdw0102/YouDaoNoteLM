package service

import (
	"YoudaoNoteLm/internal/model/entity"
	externalYoudao "YoudaoNoteLm/internal/service/external/youdao"
)

// YoudaoService 有道云笔记服务接口

// YoudaoService 有道云笔记服务接口
type YoudaoService interface {
	// 绑定管理
	Bind(userID uint, apiKey string) error
	Unbind(userID uint) error
	GetBinding(userID uint) (*entity.YoudaoBinding, error)

	// 浏览
	ListNotes(userID uint, folderID string) ([]externalYoudao.NoteItem, error)

	// 导入
	ImportNote(userID uint, notebookID uint, fileID string) (*entity.Source, error)
	ImportNotesBatch(userID uint, notebookID uint, fileIDs []string, fileNames map[string]string) (taskID string, sourceIDs []uint, err error)
}
