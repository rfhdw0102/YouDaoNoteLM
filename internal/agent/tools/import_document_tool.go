package tools

import (
	"context"
	"fmt"

	"YoudaoNoteLm/internal/service"
	"YoudaoNoteLm/pkg/logger"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"go.uber.org/zap"
)

// contextKey 用于从 context 传递用户信息
type contextKey string

const (
	userIDKey     contextKey = "import_tool_user_id"
	notebookIDKey contextKey = "import_tool_notebook_id"
)

// WithUserID 将 userID 注入 context
func WithUserID(ctx context.Context, userID uint) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

// GetUserID 从 context 获取 userID
func GetUserID(ctx context.Context) uint {
	if v, ok := ctx.Value(userIDKey).(uint); ok {
		return v
	}
	return 0
}

// WithNotebookID 将 notebookID 注入 context
func WithNotebookID(ctx context.Context, notebookID uint) context.Context {
	return context.WithValue(ctx, notebookIDKey, notebookID)
}

// GetNotebookID 从 context 获取 notebookID
func GetNotebookID(ctx context.Context) uint {
	if v, ok := ctx.Value(notebookIDKey).(uint); ok {
		return v
	}
	return 0
}

// ImportDocumentInput import_document 工具输入
type ImportDocumentInput struct {
	SourceType  string `json:"source_type" jsonschema:"enum=youdao|url|file,description=来源类型"`
	FileID      string `json:"file_id,omitempty" jsonschema_description:"有道笔记ID（source_type=youdao 时必填）"`
	URL         string `json:"url,omitempty" jsonschema_description:"网页URL（source_type=url 时必填）"`
	FileName    string `json:"file_name,omitempty" jsonschema_description:"文件名（source_type=file 时必填）"`
	FileContent string `json:"file_content,omitempty" jsonschema_description:"文件内容base64（source_type=file 时必填）"`
	NotebookID  uint   `json:"notebook_id,omitempty" jsonschema_description:"目标笔记本ID（youdao/url 来源时必填）"`
}

// ImportDocumentOutput import_document 工具输出
type ImportDocumentOutput struct {
	SourceID uint   `json:"source_id"`
	Name     string `json:"name"`
	TaskID   string `json:"task_id,omitempty"`
	Count    int    `json:"count,omitempty"`
}

// NewImportDocumentTool 创建统一的 import_document 工具
func NewImportDocumentTool(
	youdaoService service.YoudaoService,
	importerService service.ImporterService,
) (tool.InvokableTool, error) {
	return utils.InferTool("import_document", "导入文档到资料库。支持三种来源：youdao（有道云笔记）、url（网页）、file（文件上传）。导入后会自动进行结构化处理，使内容更适合搜索和阅读",
		func(ctx context.Context, input *ImportDocumentInput) (*ImportDocumentOutput, error) {
			userID := GetUserID(ctx)
			if userID == 0 {
				return nil, fmt.Errorf("未获取到用户信息")
			}

			switch input.SourceType {
			case "youdao":
				return importYoudao(ctx, youdaoService, userID, input)
			case "url":
				return importURL(ctx, importerService, userID, input)
			case "file":
				return importFile(ctx, importerService, userID, input)
			default:
				return nil, fmt.Errorf("不支持的来源类型: %s", input.SourceType)
			}
		},
	)
}

func importYoudao(ctx context.Context, youdaoService service.YoudaoService, userID uint, input *ImportDocumentInput) (*ImportDocumentOutput, error) {
	if youdaoService == nil {
		return nil, fmt.Errorf("当前 Agent 未配置有道笔记导入能力，不支持 source_type=youdao")
	}
	if input.FileID == "" {
		return nil, fmt.Errorf("source_type=youdao 时 file_id 不能为空")
	}
	if input.NotebookID == 0 {
		return nil, fmt.Errorf("source_type=youdao 时 notebook_id 不能为空")
	}

	source, err := youdaoService.ImportNote(userID, input.NotebookID, input.FileID)
	if err != nil {
		return nil, fmt.Errorf("导入有道笔记失败: %w", err)
	}

	logger.Info("import_document: 有道笔记导入成功",
		zap.Uint("source_id", source.ID),
		zap.String("name", source.Name),
	)

	return &ImportDocumentOutput{
		SourceID: source.ID,
		Name:     source.Name,
	}, nil
}

func importURL(ctx context.Context, importer service.ImporterService, userID uint, input *ImportDocumentInput) (*ImportDocumentOutput, error) {
	if input.URL == "" {
		return nil, fmt.Errorf("source_type=url 时 url 不能为空")
	}
	if input.NotebookID == 0 {
		return nil, fmt.Errorf("source_type=url 时 notebook_id 不能为空")
	}

	items := []service.SearchResultItem{
		{Title: input.URL, URL: input.URL},
	}

	taskID, sourceIDs, err := importer.ImportSearchResults(userID, input.NotebookID, items)
	if err != nil {
		return nil, fmt.Errorf("导入网页失败: %w", err)
	}

	logger.Info("import_document: 网页导入已启动",
		zap.String("task_id", taskID),
		zap.Int("count", len(sourceIDs)),
	)

	return &ImportDocumentOutput{
		TaskID: taskID,
		Count:  len(sourceIDs),
	}, nil
}

func importFile(ctx context.Context, importer service.ImporterService, userID uint, input *ImportDocumentInput) (*ImportDocumentOutput, error) {
	// 文件上传需要 multipart.FileHeader，Agent 场景下无法直接提供
	// 返回错误提示用户通过 API 上传
	return nil, fmt.Errorf("文件上传请通过 API 接口处理，Agent 暂不支持直接上传文件")
}
