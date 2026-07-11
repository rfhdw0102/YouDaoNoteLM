package rag

import (
	"context"
)

// RAGRetriever RAG 检索接口
type RAGRetriever interface {
	Retrieve(ctx context.Context, req *RetrieveRequest) ([]*RetrieveResult, error)
}

// RetrieveRequest 检索请求
type RetrieveRequest struct {
	Query       string    // 改写后的查询文本
	UserID      uint      // 用户 ID（定位 Milvus collection）
	SourceIDs   []uint    // 限定的资料来源范围
	TopK        int       // 最终返回数量，默认 5
	QueryVector []float32 // 预计算的查询向量（可选）
}

// RetrieveResult 检索结果
type RetrieveResult struct {
	Content       string  // chunk 内容
	SourceID      uint    // 资料来源 ID
	SourceName    string  // 资料来源名称
	ParentBlockID int64   // 父块 ID
	ParentContent string  // 父块完整内容
	Heading       string  // 父块标题
	ChapterPath   string  // 章节路径
	Score         float32 // 最终相关度分数
	ChunkType     string  // chunk 类型
	Metadata      string  // 元数据 JSON
}

const (
	defaultTopK = 8
)
