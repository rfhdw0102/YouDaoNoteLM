// Package generation 实现笔记内容生成模块的核心能力。
//
// 本包负责将用户输入的 Markdown 源材料转换为四种类型的产出：
// 笔记（note）、思维导图（mindmap）、PPT 课件、测验题（quiz）。
// 同时提供异步任务调度能力，支持前端通过 WebSocket 实时感知任务状态。
//
// # 架构分层
//
// 本包文件按职责分为以下几层（同属 package generation，通过文件名前缀分组）：
//
//   - 任务调度层（task_*.go）：异步任务队列、状态存储、事件推送
//   - 生成服务核心（generation_*.go）：对外接口、提示词、查询规划、记忆、导出
//   - Agent 基础设施（agent_*.go）：生成 Agent 的基类、类型、工厂
//   - 各类型生成器（{note|mindmap|quiz|ppt}_*.go）：具体类型的规划与渲染逻辑
//   - 通用工具（common_text.go、content_analysis.go、domain_aliases.go、search_types.go）
//
// # 任务调度层
//
// 任务调度层负责将同步的生成调用转换为可观测的异步任务：
//
//   - task_service.go：GenerationTaskService 实现，负责任务提交、状态流转、worker 循环
//   - task_queue.go：任务队列（内存/Redis 两种实现），worker 从中 dequeue 任务执行
//   - task_store.go：任务持久化（GenerationTaskStore 接口的缓存实现）
//   - task_event_hub.go：事件订阅中心，WebSocket 通过 SubscribeTasks 订阅任务状态变更
//
// 任务状态流转：pending → running → completed/failed/cancelled
// worker 单线程串行执行，每个任务有 10 分钟超时（generationTaskMaxRunTime），
// 避免单个 LLM 调用挂起导致后续任务永久阻塞。
//
// # 生成服务核心
//
//   - generation_interface.go：定义对外公共契约（GenerationType、GenerationRequest、
//     GenerationResponse、GenerationTask* 等类型和接口）
//   - generation_service.go：generationService 实现，负责 Agent 编排和生成流程
//   - generation_user_llm_config.go：支持用户自定义 LLM 配置的生成服务构造器
//   - generation_context.go：构建生成上下文、筛选引用材料
//   - generation_query.go：查询规划（提取关键词、构建搜索查询）
//   - generation_prompts.go：提示词策略（按类型生成不同提示词）
//   - generation_memory.go / generation_memory_store.go：会话记忆，支持跨任务上下文复用
//   - generation_export.go：内容导出（Markdown/PPTX 文件生成）
//   - generation_validators.go：各类型生成内容的校验
//   - generation_eino_model.go：eino 框架的模型适配器
//
// # Agent 基础设施
//
// 生成流程基于 Agent 链式编排（使用 cloudwego/eino 框架）：
//
//   - agent_types.go：定义 baseGenerationAgent 基类和各类型 Agent 的链式状态类型
//   - agent_base.go：baseGenerationAgent.Generate 实现 5 步链
//     （generateDraft → structureCheck → factEnhance → formatValidate → finalize）
//   - agent_factory.go：各类型 Agent 的构造工厂
//
// 各类型 Agent 通过重写基类方法实现差异化逻辑，步骤实现分布在 *_agent_steps.go 文件中。
//
// # 各类型生成器
//
// 每种生成类型由 planner（规划）+ agent_steps（链式步骤）两部分组成，
// 规划逻辑委托给对应的子包（mindmap/、note/、quiz/、ppt/）：
//
//   - 笔记生成器：note_planner.go + note_agent_steps.go + note/ 子包
//   - 思维导图生成器：mindmap_planner.go + mindmap_agent_steps.go + mindmap/ 子包
//   - 测验生成器：quiz_planner.go + quiz_agent_steps.go + quiz/ 子包
//   - PPT 生成器：ppt_plan.go + ppt_outline.go + ppt_enrich.go + ppt_agent_steps.go
//     + ppt_html_render_*.go + ppt_quality_*.go + ppt_style.go + ppt_text_context.go + ppt/ 子包
//
// PPT 生成器最为复杂，涉及大纲规划、内容增强、HTML 渲染、质量校验、样式主题等环节。
// ppt/ 子包专门负责将生成的 HTML/CSS 转换为 .pptx 文件。
//
// # 子包结构
//
//   - mindmap/：思维导图规划（types、planner、helpers、validator）
//   - note/：笔记规划（types、planner、helpers、validator）
//   - quiz/：测验规划（types、planner、helpers、validator）
//   - ppt/：PPTX 文件导出（DOM 渲染、动态 CSS 处理、模板构建）
//
// 子包与父包依赖方向为单向（父 → 子），子包不 import 父包以避免循环依赖。
// domain_aliases.go 作为桥接层，将子包的领域类型别名为父包类型。
//
// # 外部访问入口
//
// 本包对外通过 internal/service/generation_compat.go 收敛导出，
// 外部包统一 import "YoudaoNoteLm/internal/service" 并通过 service.Generation* 访问。
// 因此本包内部结构重构不会影响外部调用方，只需保持 generation_interface.go 中的契约稳定。
package generation
