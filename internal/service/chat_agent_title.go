package service

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"YoudaoNoteLm/internal/llm"
	"YoudaoNoteLm/pkg/logger"
	"YoudaoNoteLm/pkg/utils"

	"go.uber.org/zap"
)

// generateAndUpdateTitle 根据用户问题生成会话标题，返回生成的标题
func (s *chatAgentService) generateAndUpdateTitle(ctx context.Context, conversationID uint, userID uint, userQuestion string) string {
	// 构建生成标题的 prompt
	titlePrompt := fmt.Sprintf(`请根据以下用户问题，生成一个简短的会话标题（不超过20个字符）。

要求：
1. 标题要简洁明了，概括问题主题
2. 不要使用引号或特殊符号
3. 只输出标题，不要其他内容

用户问题：%s

标题：`, userQuestion)

	// 获取用户的 LLM
	llmModel, err := s.getChatModel(ctx, userID)
	if err != nil {
		logger.Warn("[Agent] 获取 LLM 失败，跳过标题生成", zap.Error(err))
		return ""
	}

	// 调用 LLM 生成标题
	messages := []*schema.Message{
		{Role: schema.User, Content: titlePrompt},
	}

	stream, err := llmModel.Stream(ctx, messages)
	if err != nil {
		logger.Warn("[Agent] 调用 LLM 生成标题失败", zap.Error(err))
		return ""
	}
	defer stream.Close()

	var title string
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			logger.Warn("[Agent] 读取标题生成结果失败", zap.Error(err))
			return ""
		}
		title += chunk.Content
	}

	// 清理标题
	title = cleanTitle(title)
	if title == "" {
		return ""
	}

	// 仅更新 title 字段，避免 db.Save 把 summary 等其他字段清零
	if err := s.conversationRepo.UpdateTitle(conversationID, title); err != nil {
		logger.Warn("[Agent] 更新对话标题失败", zap.Error(err))
		return ""
	}

	logger.Info("[Agent] 会话标题生成成功", zap.String("title", title))
	return title
}

// getChatModel 获取用户的 ChatModel
func (s *chatAgentService) getChatModel(ctx context.Context, userID uint) (model.ToolCallingChatModel, error) {
	cfg, err := s.llmConfigRepo.FindDefaultByUserID(userID)
	if err != nil {
		return nil, fmt.Errorf("获取用户 LLM 配置失败: %w", err)
	}
	if cfg == nil {
		return nil, fmt.Errorf("用户 %d 未配置 LLM", userID)
	}

	// 解密 API Key
	if cfg.APIKey != "" && len(s.encryptionKey) > 0 {
		decrypted, decErr := utils.Decrypt(cfg.APIKey, s.encryptionKey)
		if decErr != nil {
			logger.Debug("解密 API Key 失败（可能未加密）", zap.Error(decErr))
		} else {
			cfg.APIKey = decrypted
		}
	}

	return llm.NewChatModel(ctx, cfg)
}

// cleanTitle 清理标题
func cleanTitle(title string) string {
	title = strings.TrimSpace(title)
	title = strings.Trim(title, "\"'")

	// 限制标题长度
	runes := []rune(title)
	if len(runes) > 20 {
		title = string(runes[:20])
	}

	return title
}
