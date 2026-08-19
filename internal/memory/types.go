package memory

import "time"

// Type identifies a user-managed long-term output preference.
type Type string

const (
	TypeLanguage          Type = "language"
	TypeAnswerLength      Type = "answer_length"
	TypeAnswerStyle       Type = "answer_style"
	TypeOutputFormat      Type = "output_format"
	TypeGenerationStyle   Type = "generation_style"
	TypeCustomInstruction Type = "custom_instruction"
)

const (
	MaxContentRunes       = 160
	MaxSnapshotRunes      = 960
	longTermMemoryHeading = "## 用户的跨会话输出偏好"
)

var orderedTypes = []Type{
	TypeLanguage,
	TypeAnswerLength,
	TypeAnswerStyle,
	TypeOutputFormat,
	TypeGenerationStyle,
	TypeCustomInstruction,
}

var labels = map[Type]string{
	TypeLanguage:          "默认语言",
	TypeAnswerLength:      "回答篇幅",
	TypeAnswerStyle:       "回答方式",
	TypeOutputFormat:      "输出格式",
	TypeGenerationStyle:   "生成风格",
	TypeCustomInstruction: "通用偏好",
}

// Preference is the safe, structured view exposed to consumers and the API.
// It intentionally excludes database IDs and user identifiers.
type Preference struct {
	Type      Type      `json:"type"`
	Content   string    `json:"content"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Snapshot is a user's active long-term preferences at a point in time.
// Future ContextManager providers should consume Preferences directly.
type Snapshot struct {
	Preferences []Preference
}

// SupportedTypes returns a copy so callers cannot mutate the module's order.
func SupportedTypes() []Type {
	return append([]Type(nil), orderedTypes...)
}

// IsValid reports whether typ is a V1 long-term memory slot.
func (t Type) IsValid() bool {
	_, ok := labels[t]
	return ok
}

// Label returns the stable Chinese label used in rendered model context.
func (t Type) Label() string {
	return labels[t]
}
