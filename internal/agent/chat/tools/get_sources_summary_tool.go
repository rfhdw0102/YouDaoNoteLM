package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"YoudaoNoteLm/internal/repository"
	"YoudaoNoteLm/pkg/cache"
	"YoudaoNoteLm/pkg/logger"

	"go.uber.org/zap"
)

// SourceSummary 资料摘要信息
type SourceSummary struct {
	SourceID uint   `json:"source_id"`
	Name     string `json:"name"`
	Summary  string `json:"summary"`
}

// GetSourcesSummaryTool 获取资料摘要工具
type GetSourcesSummaryTool struct {
	sourceRepo   repository.SourceRepository
	summaryCache *cache.SourceSummaryCache
	sourceIDs    []uint
	sourceNames  map[uint]string
}

// NewGetSourcesSummaryTool 创建获取资料摘要工具
func NewGetSourcesSummaryTool(
	sourceRepo repository.SourceRepository,
	summaryCache *cache.SourceSummaryCache,
	sourceIDs []uint,
	sourceNames map[uint]string,
) tool.InvokableTool {
	return &GetSourcesSummaryTool{
		sourceRepo:   sourceRepo,
		summaryCache: summaryCache,
		sourceIDs:    sourceIDs,
		sourceNames:  sourceNames,
	}
}

// Info 返回工具元信息
func (t *GetSourcesSummaryTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "get_sources_summary",
		Desc: "获取用户选定资料的摘要信息。当需要了解资料的主要内容、对比资料、总结资料时使用此工具。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"source_ids": {
				Type: schema.Array,
				Desc: "要获取摘要的资料 ID 列表。如果不提供，则返回所有已选定资料的摘要。",
			},
		}),
	}, nil
}

// InvokableRun 执行获取摘要
func (t *GetSourcesSummaryTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	logger.Info("[GetSourcesSummaryTool] ====== 工具调用开始 ======",
		zap.String("arguments", argumentsInJSON),
		zap.Uints("availableSourceIDs", t.sourceIDs),
	)

	// 校验：未选中资料时不允许调用工具
	if len(t.sourceIDs) == 0 {
		return "请先选中资料再进行提问", nil
	}

	// 先用 interface{} 接收，兼容字符串和数字两种格式
	var rawParams struct {
		SourceIDs []interface{} `json:"source_ids"`
	}
	if err := json.Unmarshal([]byte(argumentsInJSON), &rawParams); err != nil {
		logger.Error("[GetSourcesSummaryTool] 参数解析失败",
			zap.String("arguments", argumentsInJSON),
			zap.Error(err),
		)
		return "", fmt.Errorf("解析参数失败: %w", err)
	}

	// 将 source_ids 转换为 []uint，兼容字符串和数字
	var sourceIDs []uint
	for _, v := range rawParams.SourceIDs {
		switch id := v.(type) {
		case float64: // JSON 数字默认解析为 float64
			sourceIDs = append(sourceIDs, uint(id))
		case string: // LLM 可能返回字符串
			var parsed uint
			if _, err := fmt.Sscanf(id, "%d", &parsed); err == nil {
				sourceIDs = append(sourceIDs, parsed)
			} else {
				logger.Warn("[GetSourcesSummaryTool] 无法解析 source_id", zap.String("value", id))
			}
		default:
			logger.Warn("[GetSourcesSummaryTool] 未知的 source_id 类型", zap.Any("value", v))
		}
	}

	// 如果未指定 source_ids，使用所有已选定的资料
	targetIDs := sourceIDs
	if len(targetIDs) == 0 {
		targetIDs = t.sourceIDs
	}

	// 过滤出有效的 sourceIDs（必须在已选定范围内）
	validIDs := make([]uint, 0, len(targetIDs))
	availableSet := make(map[uint]bool)
	for _, id := range t.sourceIDs {
		availableSet[id] = true
	}
	for _, id := range targetIDs {
		if availableSet[id] {
			validIDs = append(validIDs, id)
		}
	}

	if len(validIDs) == 0 {
		logger.Warn("[GetSourcesSummaryTool] 没有有效的资料 ID")
		return "没有找到指定的资料。请确认资料 ID 是否正确。", nil
	}

	// 获取摘要
	summaries := make([]SourceSummary, 0, len(validIDs))
	for _, sourceID := range validIDs {
		summary, err := t.getSummary(ctx, sourceID)
		if err != nil {
			logger.Warn("[GetSourcesSummaryTool] 获取摘要失败",
				zap.Uint("sourceID", sourceID),
				zap.Error(err),
			)
			continue
		}

		name := t.sourceNames[sourceID]
		if name == "" {
			name = fmt.Sprintf("资料#%d", sourceID)
		}

		summaries = append(summaries, SourceSummary{
			SourceID: sourceID,
			Name:     name,
			Summary:  summary,
		})
	}

	if len(summaries) == 0 {
		logger.Warn("[GetSourcesSummaryTool] 所有资料都没有摘要")
		return "这些资料暂时没有摘要信息。可能资料还在处理中，或者摘要生成失败。", nil
	}

	// 格式化输出
	result := formatSourceSummaries(summaries)

	logger.Info("[GetSourcesSummaryTool] ====== 工具调用完成 ======",
		zap.Int("requestedCount", len(targetIDs)),
		zap.Int("validCount", len(validIDs)),
		zap.Int("summaryCount", len(summaries)),
	)
	return result, nil
}

// getSummary 获取单个资料的摘要（优先 Redis，fallback MySQL）
func (t *GetSourcesSummaryTool) getSummary(ctx context.Context, sourceID uint) (string, error) {
	// 1. 优先从 Redis 获取
	if t.summaryCache != nil {
		summary, found, err := t.summaryCache.Get(ctx, sourceID)
		if err == nil && found && summary != "" {
			return summary, nil
		}
		if err != nil {
			logger.Warn("[GetSourcesSummaryTool] 从 Redis 获取摘要失败，降级到 MySQL",
				zap.Uint("sourceID", sourceID),
				zap.Error(err),
			)
		}
	}

	// 2. 降级：从 MySQL 获取
	summary, err := t.sourceRepo.FindSummaryByID(sourceID)
	if err != nil {
		return "", fmt.Errorf("查询摘要失败: %w", err)
	}
	if summary == "" {
		return "", fmt.Errorf("资料没有摘要")
	}

	// 3. 回写到 Redis
	if t.summaryCache != nil {
		if err := t.summaryCache.Set(ctx, sourceID, summary); err != nil {
			logger.Warn("[GetSourcesSummaryTool] 回写 Redis 摘要失败",
				zap.Uint("sourceID", sourceID),
				zap.Error(err),
			)
		}
	}

	return summary, nil
}

// formatSourceSummaries 格式化资料摘要列表
func formatSourceSummaries(summaries []SourceSummary) string {
	var sb strings.Builder
	sb.WriteString("以下是选定资料的摘要信息：\n\n")

	for i, s := range summaries {
		sb.WriteString(fmt.Sprintf("【%d】%s\n", i+1, s.Name))
		sb.WriteString(fmt.Sprintf("摘要：%s\n\n", s.Summary))
	}

	return sb.String()
}
