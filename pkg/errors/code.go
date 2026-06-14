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

	// 服务端错误 5xx
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

	// 服务端业务错误 5xxxx
	CodeInternalServiceError = 50001
)

// 错误码默认消息
var codeMessages = map[int]string{
	CodeSuccess:                     "成功",
	CodeBadRequest:                  "请求参数错误",
	CodeUnauthorized:                "未授权",
	CodeForbidden:                   "禁止访问",
	CodeNotFound:                    "资源不存在",
	CodeMethodNotAllowed:            "方法不允许",
	CodeRequestTimeout:              "请求超时",
	CodeConflict:                    "资源冲突",
	CodeInternalError:               "服务端内部错误",
	CodeNotImplemented:              "功能未实现",
	CodeServiceUnavailable:          "服务不可用",
	CodeUserNotFound:                "用户不存在",
	CodeUserAlreadyExists:           "用户已存在",
	CodeInvalidCredentials:          "邮箱或密码错误",
	CodeUserDisabled:                "用户已被禁用",
	CodeUserLocked:                  "账户已被锁定，请稍后重试",
	CodeInvalidToken:                "无效的令牌",
	CodeTokenExpired:                "令牌已过期",
	CodeVerifyCodeExpired:           "验证码已过期，请重新获取",
	CodeVerifyCodeInvalid:           "验证码错误",
	CodeVerifyCodeLocked:            "验证码输入错误次数过多，请重新获取",
	CodeVerifyCodeTooFrequent:       "验证码发送过于频繁，请稍后重试",
	CodeInvalidParam:                "参数错误",
	CodeMissingParam:                "缺少必要参数",
	CodeParamFormatError:            "参数格式错误",
	CodeResourceNotFound:            "资源不存在",
	CodeResourceAlreadyExists:       "资源已存在",
	CodeResourceLocked:              "资源已被锁定",
	CodeUnsupportedFormat:           "不支持的文件格式",
	CodeFileTooLarge:                "文件大小超限",
	CodeFileParseFailed:             "文件解析失败",
	CodeWebScrapeFailed:             "网页抓取失败",
	CodeASTranscriptionFailed:       "音频转写失败",
	CodeSearchQuotaExhausted:        "搜索 API 配额耗尽",
	CodeInvalidYoudaoAPIKey:         "有道 API Key 无效",
	CodeDuplicateImport:             "重复导入",
	CodePreviewExpired:              "预览已过期",
	CodeSearchProviderNotConfigured: "搜索 Provider 未配置",
	CodeSearchInvalidAPIKey:         "搜索 API Key 无效",
	CodeSearchRequestTimeout:        "搜索请求超时",
	CodeSearchProviderUnavailable:   "搜索 Provider 暂不可用",
	CodeSearchInvalidResponse:       "搜索 Provider 返回结构异常",
	CodeSearchProviderEmptyResult:   "搜索未返回结果",
	CodeSearchNormalizedEmptyResult: "搜索结果清洗后为空",
	CodeInternalServiceError:        "内部服务错误",
}

// GetMessage 获取错误码消息
func GetMessage(code int) string {
	if msg, ok := codeMessages[code]; ok {
		return msg
	}
	return "未知错误"
}
