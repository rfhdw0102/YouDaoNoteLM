package errors

import "fmt"

// BizError 业务错误
type BizError struct {
	Code    int
	Message string
	Err     error
}

// Error 实现 error 接口
func (e *BizError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%d] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

// New 创建新的业务错误
func New(code int, message string) *BizError {
	return &BizError{
		Code:    code,
		Message: message,
	}
}

// NewWithErr 创建带原始错误的业务错误
func NewWithErr(code int, message string, err error) *BizError {
	return &BizError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// NewDefault 创建使用默认消息的业务错误
func NewDefault(code int) *BizError {
	return &BizError{
		Code:    code,
		Message: GetMessage(code),
	}
}

// 预定义常用错误
var (
	ErrBadRequest         = NewDefault(CodeBadRequest)
	ErrUnauthorized       = NewDefault(CodeUnauthorized)
	ErrForbidden          = NewDefault(CodeForbidden)
	ErrNotFound           = NewDefault(CodeNotFound)
	ErrInternalError      = NewDefault(CodeInternalError)
	ErrUserNotFound       = NewDefault(CodeUserNotFound)
	ErrUserAlreadyExists  = NewDefault(CodeUserAlreadyExists)
	ErrUserDisabled       = NewDefault(CodeUserDisabled)
	ErrUserLocked         = NewDefault(CodeUserLocked)
	ErrInvalidCredentials = NewDefault(CodeInvalidCredentials)
	ErrInvalidToken       = NewDefault(CodeInvalidToken)
	ErrTokenExpired       = NewDefault(CodeTokenExpired)
	ErrInvalidParam       = NewDefault(CodeInvalidParam)
	ErrMissingParam       = NewDefault(CodeMissingParam)

	// 验证码相关错误
	ErrVerifyCodeExpired     = NewDefault(CodeVerifyCodeExpired)
	ErrVerifyCodeInvalid     = NewDefault(CodeVerifyCodeInvalid)
	ErrVerifyCodeLocked      = NewDefault(CodeVerifyCodeLocked)
	ErrVerifyCodeTooFrequent = NewDefault(CodeVerifyCodeTooFrequent)

	ErrUnsupportedFormat           = NewDefault(CodeUnsupportedFormat)
	ErrFileTooLarge                = NewDefault(CodeFileTooLarge)
	ErrFileParseFailed             = NewDefault(CodeFileParseFailed)
	ErrWebScrapeFailed             = NewDefault(CodeWebScrapeFailed)
	ErrASTranscriptionFailed       = NewDefault(CodeASTranscriptionFailed)
	ErrSearchQuotaExhausted        = NewDefault(CodeSearchQuotaExhausted)
	ErrInvalidYoudaoAPIKey         = NewDefault(CodeInvalidYoudaoAPIKey)
	ErrDuplicateImport             = NewDefault(CodeDuplicateImport)
	ErrPreviewExpired              = NewDefault(CodePreviewExpired)
	ErrSearchProviderNotConfigured = NewDefault(CodeSearchProviderNotConfigured)
	ErrSearchInvalidAPIKey         = NewDefault(CodeSearchInvalidAPIKey)
	ErrSearchRequestTimeout        = NewDefault(CodeSearchRequestTimeout)
	ErrSearchProviderUnavailable   = NewDefault(CodeSearchProviderUnavailable)
	ErrSearchInvalidResponse       = NewDefault(CodeSearchInvalidResponse)
	ErrSearchProviderEmptyResult   = NewDefault(CodeSearchProviderEmptyResult)
	ErrSearchNormalizedEmptyResult = NewDefault(CodeSearchNormalizedEmptyResult)
	ErrInternalServiceError        = NewDefault(CodeInternalServiceError)
)
