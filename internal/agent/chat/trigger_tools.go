package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"YoudaoNoteLm/internal/repository"
	bizerrors "YoudaoNoteLm/pkg/errors"
	"YoudaoNoteLm/pkg/logger"

	"go.uber.org/zap"
)

// GenerateFunc 生成执行函数（解耦 service 包，避免循环依赖）。
// service 包在创建 ChatAgent 时把 GenerationService.Generate 包装成此类型传入。
// 返回生成的 content（markdown 格式）。
type GenerateFunc func(ctx context.Context, userID, notebookID uint, markdown, genType, prompt string, sourceIDs []uint) (string, error)

// ============ context 注入：eventCh + bgEventCh + WaitGroup ============

type eventChKey struct{}
type bgEventChKey struct{}
type bgWaitGroupKey struct{}

func withEventCh(ctx context.Context, ch chan<- StreamEvent) context.Context {
	return context.WithValue(ctx, eventChKey{}, ch)
}

func getEventCh(ctx context.Context) chan<- StreamEvent {
	ch, _ := ctx.Value(eventChKey{}).(chan<- StreamEvent)
	return ch
}

func withBgEventCh(ctx context.Context, ch chan<- StreamEvent) context.Context {
	return context.WithValue(ctx, bgEventChKey{}, ch)
}

func getBgEventCh(ctx context.Context) chan<- StreamEvent {
	ch, _ := ctx.Value(bgEventChKey{}).(chan<- StreamEvent)
	return ch
}

// WithBgWaitGroup 注入服务层的后台任务 WaitGroup（导出给 service 层调用）。
// 触发工具用 GetBgWaitGroup 读取并注册，服务 goroutine 在主 agent 完成后 Wait 再 close eventCh。
func WithBgWaitGroup(ctx context.Context, wg *sync.WaitGroup) context.Context {
	return context.WithValue(ctx, bgWaitGroupKey{}, wg)
}

// GetBgWaitGroup 读取服务层的后台任务 WaitGroup。
func GetBgWaitGroup(ctx context.Context) *sync.WaitGroup {
	wg, _ := ctx.Value(bgWaitGroupKey{}).(*sync.WaitGroup)
	return wg
}

// sendEventSafely 向 eventCh 发送事件，防止 channel 关闭后 panic
func sendEventSafely(ctx context.Context, eventCh chan<- StreamEvent, event StreamEvent) {
	if eventCh == nil {
		return
	}
	defer func() { recover() }()
	select {
	case eventCh <- event:
	case <-ctx.Done():
	}
}

// ============ triggerSearchTool：异步触发搜索子 agent ============

// triggerSearchTool 搜索触发型 tool：LLM 调用后异步执行搜索子 agent，立即返回不阻塞。
// 直接消费 SearchAgent.ExecuteStream 的事件流，映射为前端 search_started/search_results 事件。
type triggerSearchTool struct {
	executor   SearchAgentExecutor
	userID     uint
	notebookID uint
}

func newTriggerSearchTool(executor SearchAgentExecutor, userID, notebookID uint) *triggerSearchTool {
	return &triggerSearchTool{executor: executor, userID: userID, notebookID: notebookID}
}

func (t *triggerSearchTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "SearchAgent",
		Desc: "网络搜索并分析。当需要网络信息、最新信息、资料外的知识时使用。调用后搜索结果将显示在左侧搜索面板，无需等待即可继续对话。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"query": {
				Desc:     "搜索关键词",
				Required: true,
				Type:     schema.String,
			},
		}),
	}, nil
}

func (t *triggerSearchTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var args struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
		return "", fmt.Errorf("解析搜索参数失败: %w", err)
	}
	if strings.TrimSpace(args.Query) == "" {
		return "", fmt.Errorf("搜索关键词不能为空")
	}
	return t.RunQuery(ctx, args.Query)
}

func (t *triggerSearchTool) RunQuery(ctx context.Context, query string) (string, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return "", fmt.Errorf("搜索关键词不能为空")
	}

	bgEventCh := getBgEventCh(ctx)
	bgWg := GetBgWaitGroup(ctx)
	runCtx := context.WithoutCancel(ctx)
	runCtx, cancel := context.WithTimeout(runCtx, 3*time.Minute)
	eventCh := t.executor.ExecuteStream(runCtx, t.userID, t.notebookID, query)

	firstEvt, ok := <-eventCh
	if !ok {
		cancel()
		return "搜索未能启动，请稍后重试。", nil
	}
	if firstEvt.Type == "error" {
		cancel()
		if firstEvt.Error != "" {
			return firstEvt.Error, nil
		}
		return "搜索未能启动，请稍后重试。", nil
	}

	if bgWg != nil {
		bgWg.Add(1)
	}
	go func() {
		if bgWg != nil {
			defer bgWg.Done()
		}
		defer cancel()

		var fullContent string
		startSent := false
		handleEvent := func(evt *SearchEvent) bool {
			switch evt.Type {
			case "started":
				if !startSent {
					sendEventSafely(runCtx, bgEventCh, StreamEvent{Type: EventSearchStarted})
					startSent = true
				}
				return true
			case "search_round":
				if !startSent {
					sendEventSafely(runCtx, bgEventCh, StreamEvent{Type: EventSearchStarted})
					startSent = true
				}
			case "content":
				fullContent += evt.Content
			case "error":
				if evt.ErrorCode == bizerrors.CodeConflict {
					return false
				}
				sendEventSafely(runCtx, bgEventCh, StreamEvent{
					Type:    EventError,
					Content: evt.Error,
				})
				return false
			}
			return true
		}
		if !handleEvent(firstEvt) {
			return
		}
		for evt := range eventCh {
			if !handleEvent(evt) {
				return
			}
		}

		results, summary := parseSearchResults(fullContent)
		if len(results) > 0 {
			if cb := getSearchRefCallback(runCtx); cb != nil {
				cb(results)
			}
			sendEventSafely(runCtx, bgEventCh, StreamEvent{
				Type: EventSearchResults,
				Data: map[string]interface{}{
					"results": results,
					"summary": summary,
				},
			})
		} else {
			sendEventSafely(runCtx, bgEventCh, StreamEvent{
				Type: EventSearchResults,
				Data: map[string]interface{}{
					"results": []SearchAgentResultItem{},
					"summary": "搜索失败或无结果，请重试",
				},
			})
		}
	}()

	return "搜索已开始，结果将显示在左侧搜索面板，您可以继续对话。", nil
}

func explicitSearchQuery(content string) (string, bool) {
	text := strings.TrimSpace(content)
	prefixes := []string{
		"帮我搜索一下", "帮我搜一下", "帮我搜索", "帮我搜",
		"搜索一下", "搜一下", "搜索", "搜", "查一下", "查",
	}
	for _, prefix := range prefixes {
		if !strings.HasPrefix(text, prefix) {
			continue
		}
		query := strings.TrimSpace(strings.TrimLeft(strings.TrimPrefix(text, prefix), "：:"))
		return query, query != ""
	}
	return "", false
}

// parseSearchResults 从搜索 Agent 的最终回复中提取 JSON 结果。
func parseSearchResults(content string) ([]SearchAgentResultItem, string) {
	var bestResults []SearchAgentResultItem
	var bestSummary string
	for offset := 0; offset < len(content); {
		rel := strings.IndexByte(content[offset:], '{')
		if rel < 0 {
			break
		}
		start := offset + rel
		var parsed struct {
			Results []SearchAgentResultItem `json:"results"`
			Summary string                  `json:"summary"`
		}
		if err := json.NewDecoder(strings.NewReader(content[start:])).Decode(&parsed); err == nil && len(parsed.Results) > 0 {
			bestResults = parsed.Results
			bestSummary = parsed.Summary
		}
		offset = start + 1
	}
	if len(bestResults) == 0 {
		logger.Debug("[triggerSearch] 未从输出中找到有效搜索结果 JSON")
	}
	return bestResults, bestSummary
}

// ============ triggerGenerationTool：异步触发生成服务 ============

// triggerGenerationTool 生成触发型 tool：LLM 调用后异步执行生成，立即返回不阻塞。
// 生成结果通过 generation_result 事件发到前端，作为 Note 添加到 NotesPanel。
type triggerGenerationTool struct {
	generateFn GenerateFunc
	sourceRepo repository.SourceRepository
	userID     uint
	notebookID uint
	sourceIDs  []uint
}

func newTriggerGenerationTool(generateFn GenerateFunc, sourceRepo repository.SourceRepository, userID, notebookID uint, sourceIDs []uint) *triggerGenerationTool {
	return &triggerGenerationTool{
		generateFn: generateFn,
		sourceRepo: sourceRepo,
		userID:     userID,
		notebookID: notebookID,
		sourceIDs:  sourceIDs,
	}
}

func (t *triggerGenerationTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "generate_content",
		Desc: "基于选定的资料生成思维导图/PPT/测验/笔记。调用后生成结果将显示在右侧笔记面板，无需等待即可继续对话。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"type": {
				Desc:     "生成类型：mindmap(思维导图) / ppt(演示文稿) / quiz(测验) / note(笔记)",
				Required: true,
				Type:     schema.String,
			},
			"prompt": {
				Desc:     "生成要求或额外提示（可选）",
				Required: false,
				Type:     schema.String,
			},
		}),
	}, nil
}

func (t *triggerGenerationTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var args struct {
		Type   string `json:"type"`
		Prompt string `json:"prompt"`
	}
	if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
		return "", fmt.Errorf("解析生成参数失败: %w", err)
	}

	// 校验生成类型
	validTypes := map[string]bool{"mindmap": true, "ppt": true, "quiz": true, "note": true}
	if !validTypes[args.Type] {
		return "", fmt.Errorf("不支持的生成类型: %s（支持: mindmap/ppt/quiz/note）", args.Type)
	}

	// 必须有选中的资料
	if len(t.sourceIDs) == 0 {
		return "生成失败：请先在左侧选中至少一份资料。", nil
	}

	bgEventCh := getBgEventCh(ctx)
	bgWg := GetBgWaitGroup(ctx)

	if bgWg != nil {
		bgWg.Add(1)
	}
	go func() {
		if bgWg != nil {
			defer bgWg.Done()
		}
		runCtx := context.WithoutCancel(ctx)
		runCtx, cancel := context.WithTimeout(runCtx, 5*time.Minute)
		defer cancel()

		// 通知前端生成开始（发到 bgEventCh，不阻塞主 agent 的 eventCh）
		sendEventSafely(runCtx, bgEventCh, StreamEvent{Type: EventGenerationStarted, Data: args.Type})

		// 获取选中资料的 markdown 内容
		sources, err := t.sourceRepo.FindByIDs(t.sourceIDs)
		if err != nil {
			logger.Error("[triggerGeneration] 获取资料失败", zap.Error(err))
			sendEventSafely(runCtx, bgEventCh, StreamEvent{Type: EventError, Content: "获取资料内容失败"})
			return
		}

		var mdBuilder strings.Builder
		for _, s := range sources {
			if s.MarkdownContent != "" {
				mdBuilder.WriteString(fmt.Sprintf("# %s\n\n%s\n\n---\n\n", s.Name, s.MarkdownContent))
			}
		}
		markdown := mdBuilder.String()
		if markdown == "" {
			sendEventSafely(runCtx, bgEventCh, StreamEvent{Type: EventError, Content: "资料内容为空，无法生成"})
			return
		}

		// 调用生成函数
		content, err := t.generateFn(runCtx, t.userID, t.notebookID, markdown, args.Type, args.Prompt, t.sourceIDs)
		if err != nil {
			logger.Error("[triggerGeneration] 生成失败", zap.Error(err))
			sendEventSafely(runCtx, bgEventCh, StreamEvent{Type: EventError, Content: "生成失败: " + err.Error()})
			return
		}

		// 发送生成结果
		sendEventSafely(runCtx, bgEventCh, StreamEvent{
			Type: EventGenerationResult,
			Data: map[string]interface{}{
				"type":    args.Type,
				"content": content,
			},
		})
	}()

	return "生成已开始，结果将显示在右侧笔记面板，您可以继续对话。", nil
}
