package youdao

import (
	"testing"
)

func TestParseListOutput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []NoteItem
	}{
		{
			name: "Tab 分隔格式（带 emoji）",
			input: `📁 SVR459F9DAFF051431F8428974D33FFF091	我的资源
📄 2653FFE363B84B8695852F4F5CE2E3D3	test1.note`,
			expected: []NoteItem{
				{ID: "SVR459F9DAFF051431F8428974D33FFF091", Name: "我的资源", Type: "dir"},
				{ID: "2653FFE363B84B8695852F4F5CE2E3D3", Name: "test1.note", Type: "file"},
			},
		},
		{
			name: "Tab 分隔格式（无 emoji）",
			input: `SVR459F9DAFF051431F8428974D33FFF091	我的资源
2653FFE363B84B8695852F4F5CE2E3D3	test1.note`,
			expected: []NoteItem{
				{ID: "SVR459F9DAFF051431F8428974D33FFF091", Name: "我的资源", Type: "dir"},
				{ID: "2653FFE363B84B8695852F4F5CE2E3D3", Name: "test1.note", Type: "file"},
			},
		},
		{
			name: "旧格式",
			input: `📁 我的资源 (id: SVR459F9DAFF051431F8428974D33FFF091)
📄 测试笔记 (id: 2653FFE363B84B8695852F4F5CE2E3D3)`,
			expected: []NoteItem{
				{ID: "SVR459F9DAFF051431F8428974D33FFF091", Name: "我的资源", Type: "dir"},
				{ID: "2653FFE363B84B8695852F4F5CE2E3D3", Name: "测试笔记", Type: "file"},
			},
		},
		{
			name: "混合格式",
			input: `SVR459F9DAFF051431F8428974D33FFF091	我的资源
📁 子目录 (id: SUB123)
📄 文档.md (id: DOC456)`,
			expected: []NoteItem{
				{ID: "SVR459F9DAFF051431F8428974D33FFF091", Name: "我的资源", Type: "dir"},
				{ID: "SUB123", Name: "子目录", Type: "dir"},
				{ID: "DOC456", Name: "文档.md", Type: "file"},
			},
		},
		{
			name:     "空输出",
			input:    "",
			expected: []NoteItem{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseListOutput(tt.input)
			if err != nil {
				t.Fatalf("parseListOutput 返回错误: %v", err)
			}
			if len(result) != len(tt.expected) {
				t.Errorf("期望 %d 个结果，得到 %d 个", len(tt.expected), len(result))
				return
			}
			for i, item := range result {
				if item.ID != tt.expected[i].ID {
					t.Errorf("第 %d 项 ID: 期望 %q, 得到 %q", i, tt.expected[i].ID, item.ID)
				}
				if item.Name != tt.expected[i].Name {
					t.Errorf("第 %d 项 Name: 期望 %q, 得到 %q", i, tt.expected[i].Name, item.Name)
				}
				if item.Type != tt.expected[i].Type {
					t.Errorf("第 %d 项 Type: 期望 %q, 得到 %q", i, tt.expected[i].Type, item.Type)
				}
			}
		})
	}
}

func TestParseListOutput_NilSafety(t *testing.T) {
	result, err := parseListOutput("")
	if err != nil {
		t.Fatalf("parseListOutput 返回错误: %v", err)
	}
	if result == nil {
		t.Error("parseListOutput 应该返回空数组而不是 nil")
	}
	if len(result) != 0 {
		t.Errorf("期望空数组，得到 %d 个元素", len(result))
	}
}
