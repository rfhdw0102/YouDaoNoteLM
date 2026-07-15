// generation_interface.go 定义生成模块的对外公共契约。
//
// 本文件集中定义所有对外导出的类型和接口，是 generation_compat.go 重新导出的来源。
// 重构内部实现时需保持此文件的类型/接口签名稳定。
//
// 主要类型：
//   - GenerationType：生成类型（mindmap/ppt/quiz/note）
//   - GenerationRequest / GenerationResponse：同步生成请求/响应
//   - GenerationTask / GenerationTaskEvent / GenerationTaskStatus：异步任务相关
//   - GenerationTaskStore / GenerationTaskService / GenerationTaskQueue：任务调度接口
//   - GenerationService / GenerationModel / GenerationPrompt：生成服务接口
//   - GenerationMemoryScope / GenerationMemoryEntry / GenerationMemoryStore：会话记忆
//   - GenerationExportRequest / GenerationExportResult：内容导出
package generation

import "context"

// 表示内容生成类型。
type GenerationType string

const (
	GenerationTypeMindmap GenerationType = "mindmap"
	GenerationTypePPT     GenerationType = "ppt"
	GenerationTypeQuiz    GenerationType = "quiz"
	GenerationTypeNote    GenerationType = "note"
)

// 生成模块的内部请求。
type GenerationRequest struct {
	UserID       uint           `json:"user_id,omitempty"`
	NotebookID   uint           `json:"notebook_id,omitempty"`
	Markdown     string         `json:"markdown"`
	Type         GenerationType `json:"type"`
	Prompt       string         `json:"prompt,omitempty"`
	Options      map[string]any `json:"options,omitempty"`
	SourceIDs    []uint         `json:"source_ids,omitempty"`
	UseWeb       bool           `json:"use_web,omitempty"`
	AllowDegrade bool           `json:"allow_degrade,omitempty"`
}

// 记录生成过程使用的本地引用。
type GenerationReference struct {
	SourceID    uint    `json:"source_id"`
	SourceName  string  `json:"source_name,omitempty"`
	Content     string  `json:"content"`
	Score       float32 `json:"score,omitempty"`
	Heading     string  `json:"heading,omitempty"`
	ChapterPath string  `json:"chapter_path,omitempty"`
}

// 各类生成器的统一输出。
type GenerationResponse struct {
	Type          GenerationType        `json:"type"`
	Content       string                `json:"content"`
	References    []GenerationReference `json:"references,omitempty"`
	SearchResults []SearchResult        `json:"search_results,omitempty"`
	Meta          map[string]any        `json:"meta,omitempty"`
}

// 导出生成内容的请求。
type GenerationExportRequest struct {
	Type     GenerationType `json:"type"`
	Content  string         `json:"content"`
	Title    string         `json:"title,omitempty"`
	Template string         `json:"template,omitempty"`
}

// 导出文件的二进制结果。
type GenerationExportResult struct {
	Filename    string
	ContentType string
	Data        []byte
}

// 表示异步生成任务状态。
type GenerationTaskStatus string

const (
	GenerationTaskStatusPending   GenerationTaskStatus = "pending"
	GenerationTaskStatusRunning   GenerationTaskStatus = "running"
	GenerationTaskStatusCompleted GenerationTaskStatus = "completed"
	GenerationTaskStatusFailed    GenerationTaskStatus = "failed"
	GenerationTaskStatusCancelled GenerationTaskStatus = "cancelled"
)

// 记录异步生成任务的状态和结果。
type GenerationTask struct {
	TaskID     string                 `json:"task_id"`
	UserID     uint                   `json:"user_id"`
	NotebookID uint                   `json:"notebook_id,omitempty"`
	Type       GenerationType         `json:"type"`
	Status     GenerationTaskStatus   `json:"status"`
	Result     *GenerationResponse    `json:"result,omitempty"`
	Error      string                 `json:"error,omitempty"`
	Meta       map[string]interface{} `json:"meta,omitempty"`
	CreatedAt  int64                  `json:"created_at"`
	UpdatedAt  int64                  `json:"updated_at"`
	Sequence   int64                  `json:"sequence,omitempty"`
}

const GenerationTaskEventTask = "task"

// 任务状态推送事件。
type GenerationTaskEvent struct {
	Event string          `json:"event"`
	Task  *GenerationTask `json:"task,omitempty"`
}

type GenerationTaskListFilter struct {
	UserID     uint
	NotebookID uint
	Limit      int
}

// 抽象任务持久化能力。
type GenerationTaskStore interface {
	Save(ctx context.Context, task *GenerationTask) error
	Get(ctx context.Context, taskID string) (*GenerationTask, error)
	List(ctx context.Context, filter GenerationTaskListFilter) ([]*GenerationTask, error)
}

// 管理生成任务提交、查询、取消和订阅。
type GenerationTaskService interface {
	Submit(ctx context.Context, req *GenerationRequest) (*GenerationTask, error)
	GetTask(ctx context.Context, userID uint, taskID string) (*GenerationTask, error)
	ListTasks(ctx context.Context, userID, notebookID uint, limit int) ([]*GenerationTask, error)
	CancelTask(ctx context.Context, userID uint, taskID string) error
	SubscribeTasks(ctx context.Context, userID, notebookID uint) (<-chan GenerationTaskEvent, func(), error)
}

// 传给模型的提示词载荷。
type GenerationPrompt struct {
	AgentName    string
	System       string
	User         string
	Context      string
	OutputFormat string
	MaxTokens    int
}

// 抽象底层模型生成能力。
type GenerationModel interface {
	Generate(ctx context.Context, prompt GenerationPrompt) (string, error)
}

// 生成模块的统一服务入口。
type GenerationService interface {
	Generate(ctx context.Context, req *GenerationRequest) (*GenerationResponse, error)
	Export(ctx context.Context, req *GenerationExportRequest) (*GenerationExportResult, error)
}
