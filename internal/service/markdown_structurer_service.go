package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"YoudaoNoteLm/internal/llm"
	"YoudaoNoteLm/internal/model/entity"
	"YoudaoNoteLm/pkg/logger"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

const structureSystemPrompt = `你是一个 markdown 结构化助手。你的任务是为缺乏结构的 markdown 文本补充标题和段落结构。

规则：
1. 识别内容的主题和子主题
2. 使用 ## 和 ### 层级标题标记主题和子主题
3. 按逻辑段落分段，段落之间用空行分隔
4. 对列表、步骤等内容使用 markdown 列表语法（- 或 1.）
5. 保留原始内容的文字不变，只添加结构标记（标题、段落分隔、列表标记）
6. 不要添加、删除或改写原始内容的文字
7. 不要添加元信息、摘要或总结
8. 直接输出结构化后的 markdown，不要加任何解释`

const structureUserPromptTemplate = `请为以下 markdown 文本补充结构（标题、段落分隔）。

来源类型：%s
原始标题：%s

---

%s`

// hasSufficientStructure 检测内容是否已有足够的结构
// 判断标准（满足任一即可）：
//  1. ≥2 个 h1/h2 标题
//  2. ≥3 个任意层级标题（h1-h4）
//  3. 有代码块 + 至少 1 个标题
//  4. 长文档有 ≥5 个列表项（说明本身是结构化内容）
func hasSufficientStructure(content string) bool {
	headings := 0   // 所有层级标题数
	h1h2 := 0       // h1 + h2 数
	codeBlocks := 0 // ``` 代码块标记数
	listItems := 0  // 列表项数（- / * / 1.）
	inCodeBlock := false

	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)

		// 代码块检测
		if strings.HasPrefix(trimmed, "```") {
			codeBlocks++
			inCodeBlock = !inCodeBlock
			continue
		}
		if inCodeBlock {
			continue
		}

		// 标题检测（h1-h4）
		if strings.HasPrefix(trimmed, "# ") || strings.HasPrefix(trimmed, "#\t") ||
			strings.HasPrefix(trimmed, "## ") || strings.HasPrefix(trimmed, "##\t") ||
			strings.HasPrefix(trimmed, "### ") || strings.HasPrefix(trimmed, "###\t") ||
			strings.HasPrefix(trimmed, "#### ") || strings.HasPrefix(trimmed, "####\t") {
			headings++
			if strings.HasPrefix(trimmed, "# ") || strings.HasPrefix(trimmed, "#\t") ||
				strings.HasPrefix(trimmed, "## ") || strings.HasPrefix(trimmed, "##\t") {
				h1h2++
			}
		}

		// 列表项检测
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") ||
			(len(trimmed) > 2 && trimmed[0] >= '0' && trimmed[0] <= '9' && strings.Contains(trimmed[:min(4, len(trimmed))], ". ")) {
			listItems++
		}
	}

	// 条件 1：≥2 个 h1/h2 标题
	if h1h2 >= 2 {
		return true
	}
	// 条件 2：≥3 个任意层级标题
	if headings >= 3 {
		return true
	}
	// 条件 3：有代码块 + 至少 1 个标题
	if codeBlocks >= 2 && headings >= 1 {
		return true
	}
	// 条件 4：≥5 个列表项（结构化列表内容）
	if listItems >= 5 {
		return true
	}

	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// cachedModel 缓存的 ChatModel 及其配置指纹
type cachedModel struct {
	configKey string // provider:model:apiKey:fingerprint
	model     model.ChatModel
}

type markdownStructurer struct {
	configService ConfigService
	modelCache    sync.Map // userID -> *cachedModel
}

// NewMarkdownStructurer 创建 markdown 结构化服务
func NewMarkdownStructurer(configService ConfigService) MarkdownStructurer {
	return &markdownStructurer{
		configService: configService,
	}
}

// configFingerprint 生成 LLM 配置指纹，用于判断配置是否变化
func configFingerprint(cfg *entity.UserLLMConfig) string {
	return cfg.Provider + ":" + cfg.Model + ":" + cfg.APIKey + ":" + cfg.APIURL
}

// getOrCreateChatModel 获取缓存的 ChatModel，配置变化时自动重建
func (s *markdownStructurer) getOrCreateChatModel(ctx context.Context, userID uint) (model.ChatModel, error) {
	llmConfig, err := s.configService.GetUserLLMConfig(userID)
	if err != nil {
		return nil, err
	}
	if llmConfig == nil || !llmConfig.Enabled {
		return nil, nil
	}

	key := configFingerprint(llmConfig)

	// 命中缓存且配置未变化，直接返回
	if v, ok := s.modelCache.Load(userID); ok {
		cm := v.(*cachedModel)
		if cm.configKey == key {
			return cm.model, nil
		}
	}

	// 配置变化或未缓存，创建新模型
	chatModel, err := llm.NewChatModel(ctx, llmConfig)
	if err != nil {
		return nil, err
	}

	s.modelCache.Store(userID, &cachedModel{configKey: key, model: chatModel})
	return chatModel, nil
}

func (s *markdownStructurer) Structure(ctx context.Context, userID uint, content string, meta StructureMeta) (string, error) {
	totalStart := time.Now()

	content = strings.TrimSpace(content)
	if content == "" {
		return "", nil
	}

	// 智能跳过：已有足够结构时直接返回
	if hasSufficientStructure(content) {
		logger.Info("内容已有结构，跳过 LLM 结构化",
			zap.String("source_type", meta.SourceType),
			zap.String("title", meta.Title),
			zap.Int("content_len", len(content)),
			zap.Duration("elapsed", time.Since(totalStart)),
		)
		return content, nil
	}

	logger.Info("开始 LLM 结构化",
		zap.String("source_type", meta.SourceType),
		zap.Int("content_len", len(content)),
	)

	// 获取 ChatModel（带缓存）
	stepStart := time.Now()
	chatModel, err := s.getOrCreateChatModel(ctx, userID)
	if err != nil {
		logger.Warn("获取 ChatModel 失败，跳过结构化",
			zap.Duration("elapsed", time.Since(stepStart)),
			zap.Error(err),
		)
		return content, nil
	}
	if chatModel == nil {
		logger.Debug("用户未配置 LLM，跳过结构化",
			zap.Duration("elapsed", time.Since(stepStart)),
		)
		return content, nil
	}
	logger.Info("获取 ChatModel 完成",
		zap.Bool("cached", true),
		zap.Duration("elapsed", time.Since(stepStart)),
	)

	// 调用 LLM 结构化
	stepStart = time.Now()
	structured, err := s.callLLM(ctx, chatModel, content, meta)
	if err != nil {
		logger.Warn("LLM 结构化失败，降级返回原始内容",
			zap.String("source_type", meta.SourceType),
			zap.Int("content_len", len(content)),
			zap.Duration("elapsed", time.Since(stepStart)),
			zap.Error(err),
		)
		return content, nil // 降级：LLM 调用失败时不阻塞导入
	}
	logger.Info("LLM 调用完成",
		zap.Duration("elapsed", time.Since(stepStart)),
	)

	structured = strings.TrimSpace(structured)
	if structured == "" {
		logger.Warn("LLM 返回空内容，降级返回原始内容",
			zap.String("source_type", meta.SourceType),
			zap.Duration("total_elapsed", time.Since(totalStart)),
		)
		return content, nil
	}

	// 检查是否真正发生了结构化（长度应该有变化）
	if len(structured) == len(content) {
		logger.Info("markdown 结构化完成（内容无变化）",
			zap.String("source_type", meta.SourceType),
			zap.Int("content_len", len(content)),
			zap.Duration("total_elapsed", time.Since(totalStart)),
		)
	} else {
		logger.Info("markdown 结构化完成",
			zap.String("source_type", meta.SourceType),
			zap.Int("original_len", len(content)),
			zap.Int("structured_len", len(structured)),
			zap.Duration("total_elapsed", time.Since(totalStart)),
		)
	}

	return structured, nil
}

func (s *markdownStructurer) callLLM(ctx context.Context, chatModel model.ChatModel, content string, meta StructureMeta) (string, error) {
	title := meta.Title
	if title == "" {
		title = "（无）"
	}

	userMsg := fmt.Sprintf(structureUserPromptTemplate, meta.SourceType, title, content)

	msg, err := chatModel.Generate(ctx, []*schema.Message{
		schema.SystemMessage(structureSystemPrompt),
		schema.UserMessage(userMsg),
	}, model.WithMaxTokens(2048))
	if err != nil {
		return "", fmt.Errorf("LLM 结构化调用失败: %w", err)
	}
	if msg == nil {
		return "", nil
	}
	return msg.Content, nil
}
