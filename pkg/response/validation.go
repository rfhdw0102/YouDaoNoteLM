package response

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
)

// 字段名中文映射（key 全小写，匹配时忽略大小写）
var fieldLabelMap = map[string]string{
	"name":          "配置名称",
	"provider":      "服务商",
	"api_key":       "API Key",
	"apiurl":        "API 地址",
	"api_url":       "API 地址",
	"model":         "模型名称",
	"dimensions":    "向量维度",
	"daily_quota":   "每日配额",
	"extra_config":  "扩展配置",
	"enabled":       "启用状态",
	"email":         "邮箱",
	"password":      "密码",
	"nickname":      "昵称",
	"username":      "用户名",
	"code":          "验证码",
	"refresh_token": "刷新令牌",
}

// 验证 tag 中文提示
var tagMessageMap = map[string]string{
	"required": "不能为空",
	"min":      "长度不足",
	"max":      "长度超出限制",
	"email":    "格式不正确",
	"oneof":    "值不合法",
	"len":      "长度不正确",
	"gt":       "值太小",
	"gte":      "值太小",
	"lt":       "值太大",
	"lte":      "值太大",
	"url":      "格式不正确，请输入有效的 URL",
	"uuid":     "格式不正确",
}

// ParseValidationErrors 将 validator.ValidationErrors 转为友好的中文错误信息
// 返回格式如："配置名称不能为空；API Key 不能为空"
func ParseValidationErrors(err error) string {
	var validationErrors validator.ValidationErrors
	if !errors.As(err, &validationErrors) {
		// 非 validator 错误（如 JSON 解析错误），原样返回
		return err.Error()
	}

	var msgs []string
	for _, fieldErr := range validationErrors {
		// fieldErr.Field() 可能返回 "UserConfigRequest.Name" 这种带命名空间的格式
		// fieldErr.StructField() 返回 "Name"
		label := resolveFieldLabel(fieldErr)
		msg := buildFieldMessage(label, fieldErr)
		msgs = append(msgs, msg)
	}

	return strings.Join(msgs, "；")
}

// resolveFieldLabel 从 validator.FieldError 解析出中文字段标签
func resolveFieldLabel(fieldErr validator.FieldError) string {
	// 优先用 StructField()（不带命名空间），如 "Name"
	structField := fieldErr.StructField()
	lower := strings.ToLower(structField)

	if label, ok := fieldLabelMap[lower]; ok {
		return label
	}
	return structField
}

// buildFieldMessage 根据验证 tag 构建错误提示
func buildFieldMessage(label string, fieldErr validator.FieldError) string {
	tag := fieldErr.Tag()

	switch tag {
	case "required":
		return fmt.Sprintf("%s不能为空", label)
	case "min":
		return fmt.Sprintf("%s长度至少为 %s 个字符", label, fieldErr.Param())
	case "max":
		return fmt.Sprintf("%s长度不能超过 %s 个字符", label, fieldErr.Param())
	case "email":
		return fmt.Sprintf("%s格式不正确", label)
	case "oneof":
		return fmt.Sprintf("%s的值必须是 [%s] 之一", label, fieldErr.Param())
	case "url":
		return fmt.Sprintf("%s请输入有效的 URL", label)
	case "gt":
		return fmt.Sprintf("%s必须大于 %s", label, fieldErr.Param())
	case "gte":
		return fmt.Sprintf("%s必须大于等于 %s", label, fieldErr.Param())
	case "lt":
		return fmt.Sprintf("%s必须小于 %s", label, fieldErr.Param())
	case "lte":
		return fmt.Sprintf("%s必须小于等于 %s", label, fieldErr.Param())
	default:
		if msg, ok := tagMessageMap[tag]; ok {
			return fmt.Sprintf("%s%s", label, msg)
		}
		return fmt.Sprintf("%s验证失败（%s）", label, tag)
	}
}
