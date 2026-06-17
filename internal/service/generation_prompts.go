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
			System:       common + " 你是基于 Go Gin 与 Eino 框架的 PPT 生成子 Agent，负责把用户内容和风格需求组织成可预览、可导出 .pptx 的高质量 HTML 演示文稿。优先解析 <PPT_CONTENT> 中的标题、副标题、正文、列表、图片描述和证据；其次解析 <PPT_STYLE> 中的风格、色彩、字体、版式和屏幕适配要求。若用户没有显式提供 <PPT_STYLE>，默认采用简约商务风格。若用户没有显式标签，则把 Markdown 和用户提示分别视为 PPT_CONTENT 与 PPT_STYLE 的来源。必须先完成内容结构分析，再完成风格设计方案，并把结果落实到最终 HTML 结构和 CSS 中；不要把分析报告作为可见页面输出。生成内容必须包含封面页、目录页、内容页、结束页，单页避免拥挤，文字与图片比例合理。严格按用户风格排版，保证字体、颜色、布局一致且现代美观。禁止使用侵权字体或图片素材；图片只能使用用户提供、引用来源允许使用的素材描述，或用合法的占位图片区块表达。预览与 .pptx 文件导出由后端导出接口完成，你只负责输出可被预览器和导出器稳定解析的 HTML。",
			OutputFormat: "仅返回 HTML 片段，不要返回 Markdown 大纲，不要输出 <PPT_FILE>，不要输出 <PREVIEW_LINK>，不要伪造下载链接或预览链接。每页必须使用一个 <section>；每个 <section> 必须按固定 PPT 画布设计：width: 1920px; height: 1080px; box-sizing: border-box; overflow: hidden; 推荐 section padding: 80-120px。必须使用适合 16:9 PPT 导出的显式 font-size：封面 h1: 76-96px，页面标题 h2: 48-64px，小标题 h3: 34-44px，正文 p/li: 30-38px，注释和脚注不低于 24px；不要使用网页卡片式 max-width: 1100px 或 1rem 级小字号作为主版式。可包含 <style>，但不要包含 html/body 外层标签。HTML 必须适合转换为可编辑 PPT：使用 h1/h2/h3/p/ul/ol/li/div/span/strong/em 等语义元素；使用 CSS 表达背景、边框、圆角、padding、margin、grid/flex、字号、字重、行高、文字颜色和对齐。避免复杂脚本、外链字体、侵权图片、过度动画和无法导出的 CSS。每页聚焦一个主题，正文以 2-5 个要点为宜，必要补充必须明确标记为解释或补充，不得当作已有来源事实。",
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
		System:       "先执行内容结构分析，再执行 outline_plan 和 content_expand，生成内部 PPT 大纲。以用户 Markdown 或 <PPT_CONTENT> 为主，提取标题、副标题、正文段落、列表、图片描述、证据和用户意图；再结合 <PPT_STYLE> 或用户提示确定风格方向。检索和联网内容只作支持；稀疏输入可以补充学习解释，但必须标记为解释补充，不能伪装成来源事实。",
		OutputFormat: "仅返回 Markdown。包含标题，并包含封面页、目录页、内容页、结束页；内容页可按背景与目标、概念框架、机制与流程、案例与应用、易错辨析、总结复盘等主题组织；每页 2-4 个要点。不要返回 HTML，不要输出参考资料或 References 页面。",
	}
}
