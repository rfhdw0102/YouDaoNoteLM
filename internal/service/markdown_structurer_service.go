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

const structureSystemPrompt = `你是一个 markdown 结构化助手。你必须为缺乏标题结构的 markdown 文本补充标题和段落结构。

判断标准（满足任一则认为需要结构化）：
- 没有任何 # 开头的标题
- 有标题但不超过 1 个

如果需要结构化：
1. 识别内容的主题和子主题
2. 使用 ## 和 ### 层级标题标记主题和子主题
3. 按逻辑段落分段，段落之间用空行分隔
4. 对列表、步骤等内容使用 markdown 列表语法（- 或 1.）
5. 保留原始内容的文字不变，只添加结构标记（标题、段落分隔、列表标记）
6. 不要添加、删除或改写原始内容的文字
7. 不要添加元信息、摘要或总结

只有当内容已有 2 个以上清晰的标题层级时，才直接原样返回。
直接输出结果 markdown，不要加任何解释。`

const structureUserPromptTemplate = `请为以下 markdown 文本补充标题和段落结构。如果内容没有标题或标题不足，请务必添加标题。只有已有 2 个以上标题的内容才可以原样返回。

来源类型：%s
原始标题：%s

---

%s`

// hasSufficientStructure 检测内容是否已有足够的标题结构
// 只看标题数量：≥2 个 h1/h2 或 ≥3 个任意层级标题，说明已有清晰结构
// 标题不够的交给 LLM 判断是否需要结构化
func hasSufficientStructure(content string) bool {
	headings := 0
	h1h2 := 0

	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") || strings.HasPrefix(trimmed, "## ") ||
			strings.HasPrefix(trimmed, "### ") || strings.HasPrefix(trimmed, "#### ") {
			headings++
			if strings.HasPrefix(trimmed, "# ") || strings.HasPrefix(trimmed, "## ") {
				h1h2++
			}
		}
	}

	return h1h2 >= 2 || headings >= 3
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

func (s *markdownStructurer) Structure(ctx context.Context, userID uint, content string, meta StructureMeta) (StructureResult, error) {
	totalStart := time.Now()

	content = strings.TrimSpace(content)
	if content == "" {
		return StructureResult{}, nil
	}

	// 智能跳过：已有足够结构时直接返回
	if hasSufficientStructure(content) {
		logger.Info("内容已有结构，跳过 LLM 结构化",
			zap.String("source_type", meta.SourceType),
			zap.String("title", meta.Title),
			zap.Int("content_len", len(content)),
			zap.Duration("elapsed", time.Since(totalStart)),
		)
		return StructureResult{Content: content, ActuallyCalled: false}, nil
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
		return StructureResult{Content: content, ActuallyCalled: false}, nil
	}
	if chatModel == nil {
		logger.Debug("用户未配置 LLM，跳过结构化",
			zap.Duration("elapsed", time.Since(stepStart)),
		)
		return StructureResult{Content: content, ActuallyCalled: false}, nil
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
		return StructureResult{Content: content, ActuallyCalled: false}, nil // 降级：LLM 调用失败时不阻塞导入
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
		return StructureResult{Content: content, ActuallyCalled: false}, nil
	}

	// 兜底：LLM 返回了原文且内容确实缺乏结构，强制重试一次
	if structured == content && !hasSufficientStructure(content) {
		logger.Warn("LLM 未结构化缺乏结构的内容，强制重试",
			zap.String("source_type", meta.SourceType),
			zap.Int("content_len", len(content)),
		)
		retryResult, retryErr := s.callLLMForceful(ctx, chatModel, content, meta)
		if retryErr == nil && retryResult != "" && retryResult != content {
			logger.Info("强制重试结构化成功",
				zap.Int("original_len", len(content)),
				zap.Int("structured_len", len(retryResult)),
			)
			return StructureResult{Content: retryResult, ActuallyCalled: true}, nil
		}
		logger.Warn("强制重试也未能结构化，返回原始内容",
			zap.String("source_type", meta.SourceType),
		)
	}

	if structured == content {
		logger.Info("markdown 结构化完成（LLM 判断无需结构化）",
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

	return StructureResult{Content: structured, ActuallyCalled: true}, nil
}

func (s *markdownStructurer) callLLM(ctx context.Context, chatModel model.ChatModel, content string, meta StructureMeta) (string, error) {
	title := meta.Title
	if title == "" {
		title = "（无）"
	}

	userMsg := fmt.Sprintf(structureUserPromptTemplate, meta.SourceType, title, content)

	// 动态计算 MaxTokens：内容越长给越多空间，最少 4096，最多 16384
	maxTokens := len(content) * 2
	if maxTokens < 4096 {
		maxTokens = 4096
	}
	if maxTokens > 16384 {
		maxTokens = 16384
	}

	msg, err := chatModel.Generate(ctx, []*schema.Message{
		schema.SystemMessage(structureSystemPr
	msg, err := chatModel.Generate(ctx, []*schema.Message{
	}, model.WithMaxTokens(maxTokens))
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
