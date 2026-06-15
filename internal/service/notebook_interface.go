package service

import (
	"YoudaoNoteLm/internal/model/dto/request"
	"YoudaoNoteLm/internal/model/dto/response"
)

// NotebookService 笔记本服务接口
type NotebookService interface {
	// Create 创建笔记本
	Create(userID uint, req *request.CreateNotebookRequest) (*response.NotebookResponse, error)
	// List 查询用户的所有笔记本
	List(userID uint) ([]*response.NotebookResponse, error)
	// Rename 重命名笔记本
	Rename(userID, notebookID uint, req *request.RenameNotebookRequest) error
	// Delete 删除笔记本
	Delete(userID, notebookID uint) error
}
