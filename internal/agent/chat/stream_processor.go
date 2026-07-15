package chat

import (
	"context"
	"errors"
	"io"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"

	"YoudaoNoteLm/pkg/logger"

	"go.uber.org/zap"
)

// StreamProcessor 流式事件处理器
type StreamProcessor struct{}

// NewStreamProcessor 创建流式事件处理器
func NewStreamProcessor() *StreamProcessor {
	return &StreamProcessor{}
}

// ProcessEvents 处理 Agent 事件，返回最终内容
func (p *StreamProcessor) ProcessEvents(ctx context.Context, eventCh chan<- StreamEvent, iter *adk.AsyncIterator[*adk.AgentEvent]) string {
	var fullContent string
	var prevAgentName string // 跟踪上一个事件的 AgentName，用于检测子 agent 执行边界

	for {
		event, ok := iter.Next()
		if !ok {
			break
		}

		// 处理错误事件
		if event.Err != nil {
			// 检查是否是主动取消
			if errors.Is(ctx.Err(), context.Canceled) {
				logger.Info("[StreamProcessor] 用户主动取消，保留已生成内容",
					zap.Int("contentLen", len(fullContent)),
				)
				return fullContent
			}
			// 子 agent 的错误（如搜索子 agent 超过迭代限制）不直接报错给前端，
			// 让主 agent 有机会基于已有信息降级回答（主 agent 会收到 tool error）。
			if event.AgentName != "" {
				logger.Warn("[StreamProcessor] 子 agent 错误，跳过让主 agent 处理",
					zap.String("agent", event.AgentName), zap.Error(event.Err))
				continue
			}
			logger.Error("[StreamProcessor] Agent 错误", zap.Error(event.Err))
			p.sendError(eventCh, "Agent 执行失败: "+event.Err.Error())
			return fullContent
		}

		// 处理输出事件
		if event.Output == nil || event.Output.MessageOutput == nil {
			continue
		}
		output := event.Output.MessageOutput

		// 主从协同：子 agent 边界检测（AgentName 空↔非空变化）
		// 主 agent 未设 Name（AgentName 为空），子 agent 有 Name（如 "YoudaoNoteAgent"）
		if event.AgentName != prevAgentName {
			if prevAgentName != "" {
				// 前一个子 agent 结束，通知前端关闭对应面板
				eventCh <- StreamEvent{Type: EventSubAgentEnd, Data: prevAgentName}
			}
			if event.AgentName != "" {
				// 新子 agent 开始，通知前端打开对应面板
				eventCh <- StreamEvent{Type: EventSubAgentStart, Data: event.AgentName}
			}
			prevAgentName = event.AgentName
		}

		// 主从协同：其他子 agent 事件（AgentName 非空，主 agent 未设 Name）
		// 单独处理，不累积到 fullContent（子 agent 输出作为 tool result 返回给主 agent，主 agent 会重新生成回答）
		if event.AgentName != "" {
			p.handleSubAgentEvent(event.AgentName, output, eventCh)
			continue
		}

		// 流式模式：逐 chunk 读取，实时推送 token
		if output.IsStreaming {
			p.handleStreamingOutput(ctx, output, eventCh, &fullContent)
			continue
		}

		// 非流式兜底：一次性读取完整消息
		msg, msgErr := output.GetMessage()
		if msgErr != nil {
			logger.Error("[StreamProcessor] 获取消息失败", zap.Error(msgErr))
			continue
		}
		p.handleCompleteMessage(ctx, msg, output, eventCh, &fullContent)
	}

	// 循环结束，如果还有未关闭的子 agent，推 end 通知前端关闭面板
	if prevAgentName != "" {
		eventCh <- StreamEvent{Type: EventSubAgentEnd, Data: prevAgentName}
	}

	return fullContent
}

// handleStreamingOutput 处理流式输出，逐 token 推送给前端
func (p *StreamProcessor) handleStreamingOutput(ctx context.Context, output *adk.MessageVariant, eventCh chan<- StreamEvent, fullContent *string) {
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
				logger.Error("[StreamProcessor] 流式读取失败", zap.Error(err))
				return
			}

			// 文本内容：逐 token 推送前端（但暂不累积到 fullContent）
			if chunk.Content != "" {
				streamedContent += chunk.Content
				eventCh <- StreamEvent{
					Type:    EventToken,
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
			eventCh <- StreamEvent{
				Type:    EventToolCall,
				Content: tc.Function.Name,
				Data:    tc.Function.Arguments,
			}
		}
	} else if output.Role == schema.Tool {
		// tool 结果消息：一次性读取
		msg, err := output.GetMessage()
		if err != nil {
			logger.Error("[StreamProcessor] 获取工具结果失败", zap.Error(err))
			return
		}
		eventCh <- StreamEvent{
			Type:    EventToolResult,
			Content: msg.Content,
			Data:    output.ToolName,
		}
	}
}

// handleCompleteMessage 处理非流式的完整消息（兜底）
func (p *StreamProcessor) handleCompleteMessage(ctx context.Context, msg *schema.Message, output *adk.MessageVariant, eventCh chan<- StreamEvent, fullContent *string) {
	if output.Role == schema.Assistant {
		// 文本内容和工具调用独立处理
		if msg.Content != "" {
			*fullContent += msg.Content
			eventCh <- StreamEvent{
				Type:    EventToken,
				Content: msg.Content,
			}
		}

		if len(msg.ToolCalls) > 0 {
			for _, tc := range msg.ToolCalls {
				eventCh <- StreamEvent{
					Type:    EventToolCall,
					Content: tc.Function.Name,
					Data:    tc.Function.Arguments,
				}
			}
		}
	} else if output.Role == schema.Tool {
		eventCh <- StreamEvent{
			Type:    EventToolResult,
			Content: msg.Content,
			Data:    output.ToolName,
		}
	}
}

// handleSubAgentEvent 处理子 agent 转发的事件（EmitInternalEvents）。
// 子 agent 的输出不累积到 fullContent（主 agent 会基于子 agent 返回的 tool result 重新生成回答）。
// 前端可据 sub_agent_* 事件类型展示子 agent 执行进度；前端不处理时降级为忽略，不影响主流程。
func (p *StreamProcessor) handleSubAgentEvent(agentName string, output *adk.MessageVariant, eventCh chan<- StreamEvent) {
	// 流式模式
	if output.IsStreaming {
		stream := output.MessageStream
		if stream == nil {
			return
		}
		defer stream.Close()

		if output.Role == schema.Assistant {
			var toolCalls []schema.ToolCall
			for {
				chunk, err := stream.Recv()
				if err == io.EOF {
					break
				}
				if err != nil {
					logger.Error("[StreamProcessor] 子 agent 流式读取失败",
						zap.String("agent", agentName), zap.Error(err))
					return
				}
				if chunk.Content != "" {
					eventCh <- StreamEvent{
						Type:    EventSubAgentToken,
						Content: chunk.Content,
						Data:    agentName,
					}
				}
				if len(chunk.ToolCalls) > 0 {
					toolCalls = append(toolCalls, chunk.ToolCalls...)
				}
			}
			for _, tc := range toolCalls {
				eventCh <- StreamEvent{
					Type:    EventSubAgentToolCall,
					Content: tc.Function.Name,
					Data:    agentName,
				}
			}
		}
		return
	}

	// 非流式兜底
	msg, err := output.GetMessage()
	if err != nil {
		logger.Error("[StreamProcessor] 子 agent 获取消息失败",
			zap.String("agent", agentName), zap.Error(err))
		return
	}

	if output.Role == schema.Assistant {
		if msg.Content != "" {
			eventCh <- StreamEvent{
				Type:    EventSubAgentToken,
				Content: msg.Content,
				Data:    agentName,
			}
		}
		for _, tc := range msg.ToolCalls {
			eventCh <- StreamEvent{
				Type:    EventSubAgentToolCall,
				Content: tc.Function.Name,
				Data:    agentName,
			}
		}
	} else if output.Role == schema.Tool {
		eventCh <- StreamEvent{
			Type:    EventSubAgentToolResult,
			Content: msg.Content,
			Data:    agentName,
		}
	}
}

// sendError 发送错误事件
func (p *StreamProcessor) sendError(eventCh chan<- StreamEvent, msg string) {
	eventCh <- StreamEvent{
		Type:    EventError,
		Content: msg,
	}
}
