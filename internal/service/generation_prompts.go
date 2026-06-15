package service

type generationPromptStrategy struct {
	System       string
	OutputFormat string
}

func promptStrategyFor(typ GenerationType) generationPromptStrategy {
	common := "以本地笔记为主要依据；联网搜索只作为补充背景。保持不同来源的边界，不编造缺乏依据的结论，并在输出格式允许时标注参考资料。"
	switch typ {
	case GenerationTypeMindmap:
		return generationPromptStrategy{
			System:       common + " 生成知识结构型思维导图，突出概念、层级和关系，不要把原文段落简单改写成列表。",
			OutputFormat: "仅返回 Markmap 兼容的 Markdown。以一个 # 标题开头，尽可能包含至少两级结构，分支名称保持简洁；只有在确有帮助时才加入参考资料分支。",
		}
	case GenerationTypePPT:
		return generationPromptStrategy{
			System:       common + " 生成演示文稿大纲，包含清晰叙事、每页核心结论、支撑细节，以及能对应来源的示例。",
			OutputFormat: "仅返回 HTML 片段。每页使用一个 <section>，内部使用 h1/h2 和项目符号列表。不要包含 html/body 外层标签。每页聚焦一个主题，并用证据支撑。",
		}
	case GenerationTypeQuiz:
		return generationPromptStrategy{
			System:       common + " 生成题目，用来考查定义、常见误区、推理能力和应用能力。优先使用本地笔记中有依据的概念。",
			OutputFormat: `仅返回 JSON：{"questions":[{"type":"single_choice|short_answer","question":"","options":[],"answer":"","explanation":""}]}。每道题都必须包含答案和解析。`,
		}
	case GenerationTypeNote:
		return generationPromptStrategy{
			System:       common + " 生成学习笔记，改善结构，谨慎补全缺口，并明确保留重要术语。",
			OutputFormat: "仅返回 Markdown。包含标题、摘要、关键点、详细章节；如有参考资料，请一并列出。",
		}
	default:
		return generationPromptStrategy{
			System:       common,
			OutputFormat: "仅返回用户请求的内容。",
		}
	}
}
