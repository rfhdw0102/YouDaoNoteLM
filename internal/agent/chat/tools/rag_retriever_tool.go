package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"YoudaoNoteLm/internal/model/dto/response"
	"YoudaoNoteLm/internal/rag"
	"YoudaoNoteLm/pkg/logger"

	"go.uber.org/zap"
)

// RAGRetrieverTool 知识库检索工具
type RAGRetrieverTool struct {
	retriever rag.RAGRetriever
	userID    uint
	sourceIDs []uint
	collector *ReferenceCollector // 引用收集器，跨多次调用累积
}

// NewRAGRetrieverTool 创建检索工具
func NewRAGRetrieverTool(retriever rag.RAGRetriever, userID uint, sourceIDs []uint, collector *ReferenceCollector) tool.InvokableTool {
	return &RAGRetrieverTool{
		retriever: retriever,
		userID:    userID,
		sourceIDs: sourceIDs,
		collector: collector,
	}
}

// Info 返回工具元信息
func (t *RAGRetrieverTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "search_knowledge",
		Desc: "从用户的知识库中检索相关资料。当需要查找文档、笔记、资料内容时使用此工具。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"query": {
				Type:     schema.String,
				Desc:     "搜索查询词，应该是具体、明确的关键词",
				Required: true,
			},
			"top_k": {
				Type: schema.Integer,
				Desc: "返回结果数量，默认 5，最大 10",
			},
		}),
	}, nil
}

// InvokableRun 执行检索
func (t *RAGRetrieverTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	logger.Info("[RAGRetrieverTool] ====== 工具调用开始 ======",
		zap.String("arguments", argumentsInJSON),
		zap.Uint("userID", t.userID),
		zap.Uints("sourceIDs", t.sourceIDs),
	)

	// 校验：未选中资料时不允许调用工具
	if len(t.sourceIDs) == 0 {
		return "请先选中资料再进行提问", nil
	}

	var rawParams struct {
		Query string      `json:"query"`
		TopK  interface{} `json:"top_k"`
	}
	if err := json.Unmarshal([]byte(argumentsInJSON), &rawParams); err != nil {
		logger.Error("[RAGRetrieverTool] 参数解析失败",
			zap.String("arguments", argumentsInJSON),
			zap.Error(err),
		)
		return "", fmt.Errorf("解析参数失败: %w", err)
	}
	if rawParams.Query == "" {
		logger.Warn("[RAGRetrieverTool] query 参数为空")
		return "错误：query 参数不能为空", nil
	}

	// 兼容 top_k 为数字或字符串的情况
	topK := 5
	switch v := rawParams.TopK.(type) {
	case float64:
		topK = int(v)
	case string:
		if parsed, err := fmt.Sscanf(v, "%d", &topK); err == nil && parsed > 0 {
			// ok
		}
	}
	if topK <= 0 || topK > 10 {
		topK = 5
	}
	params := struct {
		Query string
		TopK  int
	}{Query: rawParams.Query, TopK: topK}

	logger.Info("[RAGRetrieverTool] 开始检索",
		zap.String("query", params.Query),
		zap.Int("topK", params.TopK),
		zap.Uint("userID", t.userID),
		zap.Uints("sourceIDs", t.sourceIDs),
	)

	results, err := t.retriever.Retrieve(ctx, &rag.RetrieveRequest{
		Query:     params.Query,
		UserID:    t.userID,
		SourceIDs: t.sourceIDs,
		TopK:      params.TopK,
	})
	if err != nil {
		logger.Error("[RAGRetrieverTool] 检索失败",
			zap.String("query", params.Query),
			zap.Uint("userID", t.userID),
			zap.Uints("sourceIDs", t.sourceIDs),
			zap.Error(err),
		)
		return "检索失败: " + err.Error(), nil
	}
	if len(results) == 0 {
		logger.Warn("[RAGRetrieverTool] 检索结果为空",
			zap.String("query", params.Query),
			zap.Uint("userID", t.userID),
			zap.Uints("sourceIDs", t.sourceIDs),
		)
	} else {
		logger.Info("[RAGRetrieverTool] 检索成功",
			zap.String("query", params.Query),
			zap.Int("resultCount", len(results)),
		)
	}

	// 累积引用到 collector，并拿到本次检索在全局列表中的起始编号
	refs := make([]response.Reference, 0, len(results))
	for _, r := range results {
		refs = append(refs, response.Reference{
			SourceID:      r.SourceID,
			SourceName:    r.SourceName,
			ParentBlockID: r.ParentBlockID,
			ChunkContent: func() string {
				if r.ParentContent != "" {
					return r.ParentContent
				}
				return r.Content
			}(),
			Score: r.Score,
		})
	}
	startIndex := 1
	if t.collector != nil {
		startIndex = t.collector.Add(refs)
	}

	logger.Info("[RAGRetrieverTool] ====== 工具调用完成 ======",
		zap.Int("resultCount", len(results)),
		zap.Int("startIndex", startIndex),
	)
	return FormatRetrievalResults(results, startIndex), nil
}
