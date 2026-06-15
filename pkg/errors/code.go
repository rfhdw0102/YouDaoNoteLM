package errors

// 错误码定义
const (
	// 成功
	CodeSuccess = 0

	// 通用错误 4xx
	CodeBadRequest       = 400
	CodeUnauthorized     = 401
	CodeForbidden        = 403
	CodeNotFound         = 404
	CodeMethodNotAllowed = 405
	CodeRequestTimeout   = 408
	CodeConflict         = 409

	// 服务器错误 5xx
	CodeInternalError      = 500
	CodeNotImplemented     = 501
	CodeServiceUnavailable = 503

	// 业务错误 1xxx
	CodeUserNotFound       = 1001
	CodeUserAlreadyExists  = 1002
	CodeInvalidCredentials = 1003
	CodeUserDisabled       = 1004
	CodeInvalidToken       = 1005
	CodeTokenExpired       = 1006
	CodeUserLocked         = 1007

	// 验证码错误 11xx
	CodeVerifyCodeExpired     = 1101
	CodeVerifyCodeInvalid     = 1102
	CodeVerifyCodeLocked      = 1103
	CodeVerifyCodeTooFrequent = 1104

	// 参数错误 2xxx
	CodeInvalidParam     = 2001
	CodeMissingParam     = 2002
	CodeParamFormatError = 2003

	// 资源错误 3xxx
	CodeResourceNotFound      = 3001
	CodeResourceAlreadyExists = 3002
	CodeResourceLocked        = 3003

	// 导入模块错误 4xxxx
	CodeUnsupportedFormat           = 40001
	CodeFileTooLarge                = 40002
	CodeFileParseFailed             = 40003
	CodeWebScrapeFailed             = 40004
	CodeASTranscriptionFailed       = 40005
	CodeSearchQuotaExhausted        = 40006
	CodeInvalidYoudaoAPIKey         = 40007
	CodeDuplicateImport             = 40008
	CodePreviewExpired              = 40009
	CodeSearchProviderNotConfigured = 40010
	CodeSearchInvalidAPIKey         = 40011
	CodeSearchRequestTimeout        = 40012
	CodeSearchProviderUnavailable   = 40013
	CodeSearchInvalidResponse       = 40014
	CodeSearchProviderEmptyResult   = 40015
	CodeSearchNormalizedEmptyResult = 40016

	// 搜索 Agent 错误码 4xxxx（续）
	CodeLLMNotConfigured   = 40010
	CodeLLMCallFailed      = 40011
	CodeLLMResponseInvalid = 40012
	CodeSearchAgentTimeout = 40013

	// 服务器错误 5xxxx
	CodeInternalServiceError = 50001

	// 配置健康检查错误 6xxxx
	CodeConfigTestFailed  = 60001 // 配置连通性测试失败
	CodeConfigTestTimeout = 60002 // 配置连通性测试超时
	CodeConfigTestInvalid = 60003 // 配置参数无效
)

// 错误码默认消息
var codeMessages = map[int]string{
	CodeSuccess:               "成功",
	CodeBadRequest:            "请求参数错误",
	CodeUnauthorized:          "未授权",
	CodeForbidden:             "禁止访问",
	CodeNotFound:              "资源不存在",
	CodeMethodNotAllowed:      "方法不允许",
	CodeRequestTimeout:        "请求超时",
	CodeConflict:              "资源冲突",
	CodeInternalError:         "服务器内部错误",
	CodeNotImplemented:        "功能未实现",
	CodeServiceUnavailable:    "服务不可用",
	CodeUserNotFound:          "用户不存在",
	CodeUserAlreadyExists:     "用户已存在",
	CodeInvalidCredentials:    "邮箱或密码错误",
	CodeUserDisabled:          "用户已被禁用",
	CodeUserLocked:            "账户已被锁定，请15分钟后重试",
	CodeInvalidToken:          "无效的令牌",
	CodeTokenExpired:          "令牌已过期",
	CodeVerifyCodeExpired:     "验证码已过期，请重新获取",
	CodeVerifyCodeInvalid:     "验证码错误",
	CodeVerifyCodeLocked:      "验证码输入错误次数过多，请重新获取",
	CodeVerifyCodeTooFrequent: "验证码发送过于频繁，请60秒后重试",
	CodeInvalidParam:          "参数错误",
	CodeMissingParam:          "缺少必要参数",
	CodeParamFormatError:      "参数格式错误",
	CodeResourceNotFound:      "资源不存在",
	CodeResourceAlreadyExists: "资源已存在",
	CodeResourceLocked:        "资源已被锁定",

	CodeUnsupportedFormat:     "不支持的文件格式",
	CodeFileTooLarge:          "文件大小超限",
	CodeFileParseFailed:       "文件解析失败",
	CodeWebScrapeFailed:       "网页抓取失败",
	CodeASTranscriptionFailed: "音频转写失败",
	CodeSearchQuotaExhausted:  "搜索API配额耗尽",
	CodeInvalidYoudaoAPIKey:   "有道API Key无效",
	CodeDuplicateImport:       "重复导入",
	CodePreviewExpired:        "预览已过期",
	CodeLLMNotConfigured:      "请先在设置中配置 LLM 服务",
	CodeLLMCallFailed:         "LLM 服务调用失败",
	CodeLLMResponseInvalid:    "LLM 返回结果格式异常",
	CodeSearchAgentTimeout:    "搜索 Agent 执行超时",
	CodeInternalServiceError:  "内部服务错误",
	CodeConfigTestFailed:      "配置连通性测试失败",
	CodeConfigTestTimeout:     "配置连通性测试超时",
	CodeConfigTestInvalid:     "配置参数无效",
}

// GetMessage 获取错误码消息
func GetMessage(code int) string {
	if msg, ok := codeMessages[code]; ok {
		return msg
	}
	return "未知错误"
}
