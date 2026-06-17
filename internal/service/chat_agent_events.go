package service

import (
	"context"
	"io"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"

	"YoudaoNoteLm/pkg/logger"

	"go.uber.org/zap"
)

// forwardAgentEvents 转发 Agent 事件，返回最终内容
func (s *chatAgentService) forwardAgentEvents(ctx context.Context, eventCh chan<- AgentStreamEvent, iter *adk.AsyncIterator[*adk.AgentEvent]) string {
	var fullContent string

	for {
		event, ok := iter.Next()
		if !ok {
			break
		}

		// 处理错误事件
		if event.Err != nil {
			// 检查是否是主动取消
			if ctx.Err() == context.Canceled {
				logger.Info("[Agent] 用户主动取消，保留已生成内容", zap.Int("contentLen", len(fullContent)))
				// 如果有已生成内容，发送 token 事件让前端保留
				if len(fullContent) > 0 {
					eventCh <- AgentStreamEvent{
						Type:    AgentEventToken,
						Content: "", // 空内容表示流结束
					}
				}
				return fullContent
			}
			logger.Error("[Agent] Agent 错误", zap.Error(event.Err))
			s.sendAgentError(eventCh, "Agent 执行失败: "+event.Err.Error())
			return fullContent
		}

		// 处理输出事件
		if event.Output == nil || event.Output.MessageOutput == nil {
			continue
		}
		output := event.Output.MessageOutput

		// 流式模式：逐 chunk 读取，实时推送 token
		if output.IsStreaming {
			s.handleStreamingOutput(output, eventCh, &fullContent)
			continue
		}

		// 非流式兜底：一次性读取完整消息
		msg, msgErr := output.GetMessage()
		if msgErr != nil {
			logger.Error("[Agent] 获取消息失败", zap.Error(msgErr))
			continue
		}
		s.handleCompleteMessage(msg, output, eventCh, &fullContent)
	}

	return fullContent
}

// handleStreamingOutput 处理流式输出，逐 token 推送给前端
func (s *chatAgentService) handleStreamingOutput(output *adk.MessageVariant, eventCh chan<- AgentStreamEvent, fullContent *string) {
	stream := output.MessageStream
	if stream == nil {
		return
	}
	defer stream.Close()

	if output.Role == schema.Assistant {
		var toolCalls []schema.ToolCall
		var streamedContent string // 本轮流式内容，暂存

		for {
			chunk, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				logger.Error("[Agent] 流式读取失败", zap.Error(err))
				return
			}

			// 文本内容：逐 token 推送前端（但暂不累积到 fullContent）
			if chunk.Content != "" {
				streamedContent += chunk.Content
				eventCh <- AgentStreamEvent{
					Type:    AgentEventToken,
					Content: chunk.Content,
				}
			}

			// 收集工具调用（Anthropic 流式中 tool_use 以完整块发出）
			if len(chunk.ToolCalls) > 0 {
				toolCalls = append(toolCalls, chunk.ToolCalls...)
			}
		}

		// 流结束后判断：有工具调用则为中间推理，丢弃文本；无工具调用则为最终回答，保留
		if len(toolCalls) == 0 {
			*fullContent += streamedContent
		}

		// 发送工具调用事件
		for _, tc := range toolCalls {
			eventCh <- AgentStreamEvent{
				Type:    AgentEventToolCall,
				Content: tc.Function.Name,
				Data:    tc.Function.Arguments,
			}
		}
	} else if output.Role == schema.Tool {
		// tool 结果消息：一次性读取
		msg, err := output.GetMessage()
		if err != nil {
			logger.Error("[Agent] 获取工具结果失败", zap.Error(err))
			return
		}
		eventCh <- AgentStreamEvent{
			Type:    AgentEventToolResult,
			Content: msg.Content,
			Data:    output.ToolName,
		}
	}
}

// handleCompleteMessage 处理非流式的完整消息（兜底）
func (s *chatAgentService) handleCompleteMessage(msg *schema.Message, output *adk.MessageVariant, eventCh chan<- AgentStreamEvent, fullContent *string) {
	if output.Role == schema.Assistant {
		// 文本内容和工具调用独立处理
		if msg.Content != "" {
			*fullContent += msg.Content
			eventCh <- AgentStreamEvent{
				Type:    AgentEventToken,
				Content: msg.Content,
			}
		}

		if len(msg.ToolCalls) > 0 {
			for _, tc := range msg.ToolCalls {
				eventCh <- AgentStreamEvent{
					Type:    AgentEventToolCall,
					Content: tc.Function.Name,
					Data:    tc.Function.Arguments,
				}
			}
		}
	} else if output.Role == schema.Tool {
		eventCh <- AgentStreamEvent{
			Type:    AgentEventToolResult,
			Content: msg.Content,
			Data:    output.ToolName,
		}
	}
}

// sendAgentError 发送错误事件
func (s *chatAgentService) sendAgentError(eventCh chan<- AgentStreamEvent, msg string) {
	eventCh <- AgentStreamEvent{
		Type:    AgentEventError,
		Content: msg,
	}
}
