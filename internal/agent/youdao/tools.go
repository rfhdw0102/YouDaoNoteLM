package youdao

import (
	"context"
	"fmt"

	"YoudaoNoteLm/internal/service"
	externalYoudao "YoudaoNoteLm/internal/service/external/youdao"
	"YoudaoNoteLm/pkg/logger"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"go.uber.org/zap"
)

// ========== list_notes 工具 ==========

type ListNotesInput struct {
	FolderID string `json:"folder_id" jsonschema_description:"目录ID，为空则列出根目录"`
}

type ListNotesOutput struct {
	Items []externalYoudao.NoteItem `json:"items"`
}

func NewListNotesTool(youdaoService service.YoudaoService) (tool.InvokableTool, error) {
	return utils.InferTool("list_notes", "浏览有道云笔记目录。输入目录ID列出该目录下的笔记，为空则列出根目录",
		func(ctx context.Context, input *ListNotesInput) (*ListNotesOutput, error) {
			userID := GetUserID(ctx)
			items, err := youdaoService.ListNotes(userID, input.FolderID)
			if err != nil {
				return nil, fmt.Errorf("获取笔记列表失败: %w", err)
			}
			logger.Info("list_notes 执行成功", zap.Uint("user_id", userID), zap.Int("count", len(items)))
			return &ListNotesOutput{Items: items}, nil
		},
	)
}

// ========== read_note 工具 ==========

type ReadNoteInput struct {
	FileID string `json:"file_id" jsonschema_description:"笔记ID，从 list_notes 或 search_notes 结果中获取"`
}

type ReadNoteOutput struct {
	Content   string `json:"content"`
	RawFormat string `json:"raw_format"`
}

func NewReadNoteTool(youdaoCLI externalYoudao.CLI, youdaoService service.YoudaoService) (tool.InvokableTool, error) {
	return utils.InferTool("read_note", "读取有道云笔记内容。输入笔记ID，返回笔记的 Markdown 内容",
		func(ctx context.Context, input *ReadNoteInput) (*ReadNoteOutput, error) {
			userID := GetUserID(ctx)
			apiKey, err := getAPIKeyFromService(youdaoService, userID)
			if err != nil {
				return nil, err
			}

			result, err := youdaoCLI.Read(apiKey, input.FileID)
			if err != nil {
				return nil, fmt.Errorf("读取笔记失败: %w", err)
			}
			logger.Info("read_note 执行成功", zap.Uint("user_id", userID), zap.String("file_id", input.FileID))
			return &ReadNoteOutput{Content: result.Content, RawFormat: result.RawFormat}, nil
		},
	)
}

// ========== search_notes 工具 ==========

type SearchNotesInput struct {
	Keyword string `json:"keyword" jsonschema_description:"搜索关键词"`
}

type SearchNotesOutput struct {
	Items []externalYoudao.NoteItem `json:"items"`
}

func NewSearchNotesTool(youdaoCLI externalYoudao.CLI, youdaoService service.YoudaoService) (tool.InvokableTool, error) {
	return utils.InferTool("search_notes", "搜索有道云笔记。输入关键词，返回匹配的笔记列表",
		func(ctx context.Context, input *SearchNotesInput) (*SearchNotesOutput, error) {
			userID := GetUserID(ctx)
			apiKey, err := getAPIKeyFromService(youdaoService, userID)
			if err != nil {
				return nil, err
			}

			items, err := youdaoCLI.Search(apiKey, input.Keyword)
			if err != nil {
				return nil, fmt.Errorf("搜索笔记失败: %w", err)
			}
			logger.Info("search_notes 执行成功", zap.Uint("user_id", userID), zap.Int("count", len(items)))
			return &SearchNotesOutput{Items: items}, nil
		},
	)
}

// ========== create_note 工具 ==========

type CreateNoteInput struct {
	Title    string `json:"title" jsonschema_description:"笔记标题"`
	Content  string `json:"content" jsonschema_description:"笔记内容（Markdown格式）"`
	ParentID string `json:"parent_id" jsonschema_description:"目标目录ID，为空则保存到默认位置"`
}

type CreateNoteOutput struct {
	NoteID string `json:"note_id"`
}

func NewCreateNoteTool(youdaoCLI externalYoudao.CLI, youdaoService service.YoudaoService) (tool.InvokableTool, error) {
	return utils.InferTool("create_note", "创建有道云笔记。输入标题和Markdown内容，保存到有道云笔记",
		func(ctx context.Context, input *CreateNoteInput) (*CreateNoteOutput, error) {
			userID := GetUserID(ctx)
			apiKey, err := getAPIKeyFromService(youdaoService, userID)
			if err != nil {
				return nil, err
			}

			noteID, err := youdaoCLI.CreateNote(apiKey, input.Title, input.Content, input.ParentID)
			if err != nil {
				return nil, fmt.Errorf("创建笔记失败: %w", err)
			}
			logger.Info("create_note 执行成功", zap.Uint("user_id", userID), zap.String("note_id", noteID))
			return &CreateNoteOutput{NoteID: noteID}, nil
		},
	)
}

// ========== 辅助函数 ==========

// getAPIKeyFromService 从 YoudaoService 获取用户的 API Key
func getAPIKeyFromService(youdaoService service.YoudaoService, userID uint) (string, error) {
	binding, err := youdaoService.GetBinding(userID)
	if err != nil {
		return "", fmt.Errorf("获取绑定信息失败: %w", err)
	}
	if binding == nil || binding.Status != "active" {
		return "", fmt.Errorf("请先绑定有道云笔记账号")
	}
	return binding.APIKey, nil
}
