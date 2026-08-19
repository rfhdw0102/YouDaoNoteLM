package memory

import (
	"errors"
	"strings"
)

var (
	ErrInvalidType     = errors.New("无效的记忆类型")
	ErrEmptyContent    = errors.New("记忆内容不能为空")
	ErrContentTooLong  = errors.New("记忆内容不能超过160个字符")
	ErrInvalidLanguage = errors.New("默认语言仅支持中文或English")
	ErrInvalidUser     = errors.New("无效的用户")
)

// NormalizeContent keeps V1 preferences compact and prevents user-provided
// formatting from changing the structure of the generated prompt block.
func NormalizeContent(content string) string {
	return strings.Join(strings.Fields(content), " ")
}

func validateType(typ Type) error {
	if !typ.IsValid() {
		return ErrInvalidType
	}
	return nil
}

func validateUserID(userID uint) error {
	if userID == 0 {
		return ErrInvalidUser
	}
	return nil
}

func validateContent(content string) (string, error) {
	normalized := NormalizeContent(content)
	if normalized == "" {
		return "", ErrEmptyContent
	}
	if len([]rune(normalized)) > MaxContentRunes {
		return "", ErrContentTooLong
	}
	return normalized, nil
}

// validatePreferenceContent validates generic preference text and narrows the
// language slot to values the application can safely enforce as a constraint.
func validatePreferenceContent(typ Type, content string) (string, error) {
	normalized, err := validateContent(content)
	if err != nil || typ != TypeLanguage {
		return normalized, err
	}
	language, ok := canonicalLanguage(normalized)
	if !ok {
		return "", ErrInvalidLanguage
	}
	return language, nil
}

func canonicalLanguage(content string) (string, bool) {
	switch strings.ToLower(content) {
	case "中文":
		return "中文", true
	case "english":
		return "English", true
	default:
		return "", false
	}
}

// IsValidationError lets the HTTP adapter return the existing invalid-param
// response without teaching the memory module about HTTP or business codes.
func IsValidationError(err error) bool {
	return errors.Is(err, ErrInvalidType) ||
		errors.Is(err, ErrEmptyContent) ||
		errors.Is(err, ErrContentTooLong) ||
		errors.Is(err, ErrInvalidLanguage) ||
		errors.Is(err, ErrInvalidUser)
}
