package service

type generationPromptStrategy struct {
	System       string
	OutputFormat string
}

func promptStrategyFor(typ GenerationType) generationPromptStrategy {
	common := "以本地笔记为主要依据，联网搜索只作为补充背景。保持不同来源的边界，不编造缺乏依据的结论。参考资料会由系统在响应元数据中单独展示，不要在生成正文中添加参考资料、References、来源列表或引用附录。"
	switch typ {
	case GenerationTypeMindmap:
		return generationPromptStrategy{
			System:       common + " 生成学习资料型思维导图。先分析概念、关系、过程和例子，再规划核心概念、原理机制、过程步骤、应用场景、易错点和总结等稳定分支；保留证据边界，不要把原文段落简单改写成列表。",
			OutputFormat: "仅返回 Markmap 兼容的 Markdown。以一个 # 标题开头，优先使用 ## 核心概念、## 原理机制、## 过程步骤、## 应用场景、## 易错点、## 总结 等稳定分支，并在分支下补入 ### 节点和必要的 #### 解释形成清晰层级；不要加入参考资料或 References 分支。",
		}
	case GenerationTypePPT:
		return generationPromptStrategy{
			System:       common + " 根据内部演示大纲生成学习课件型演示文稿 HTML。遵循 content_analyze -> outline_plan -> content_expand 的计划结果组织内容；补充性 bullet 必须明确标记为解释或补充，不得当作已有来源事实。",
			OutputFormat: "仅返回 HTML 片段。每页使用一个 <section>，内部使用 h1/h2 和项目符号列表。不要包含 html/body 外层标签。每页聚焦一个学习主题，并用证据支撑；不要返回 Markdown 大纲。",
		}
	case GenerationTypeQuiz:
		return generationPromptStrategy{
			System:       common + " 生成题目，用来考查定义、常见误区、推理能力和应用能力。优先使用本地笔记中有依据的概念。",
			OutputFormat: `仅返回 JSON：{"questions":[{"type":"single_choice|short_answer","question":"","options":[],"answer":"","explanation":""}]}。每道题都必须包含答案和解析。`,
		}
	case GenerationTypeNote:
		return generationPromptStrategy{
			System:       common + " 生成学习笔记，改善结构，谨慎补全缺口，并明确保留重要术语。",
			OutputFormat: "仅返回 Markdown。包含标题、摘要、关键点、详细章节；不要输出参考资料、References、来源列表或引用附录。",
		}
	default:
		return generationPromptStrategy{
			System:       common,
			OutputFormat: "仅返回用户请求的内容。",
		}
	}
}

func pptOutlinePromptStrategy() generationPromptStrategy {
	return generationPromptStrategy{
		System:       "先执行 content_analyze，再执行 outline_plan 和 content_expand，生成内部学习课件大纲。以用户 Markdown 为主，检索和联网内容只作支持；稀疏输入可以补充学习解释，但必须标记为解释补充，不能伪装成来源事实。",
		OutputFormat: "仅返回 Markdown。包含标题，并包含封面、目录、背景与目标、概念框架、机制与流程、案例与应用、易错辨析、总结复盘等页面；每页 2-4 个要点。不要返回 HTML，不要输出参考资料或 References 页面。",
	}
}
