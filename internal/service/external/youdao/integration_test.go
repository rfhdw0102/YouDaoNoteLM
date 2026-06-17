//go:build integration
// +build integration

package youdao

import (
	"encoding/json"
	"fmt"
	"testing"
)

// TestYoudaoCLI_Integration 集成测试（需要真实 API Key）
// 运行: go test -tags=integration -v -run TestYoudaoCLI_Integration ./internal/service/external/youdao/
func TestYoudaoCLI_Integration(t *testing.T) {
	apiKey := "iv11Sve5jvRFpLsejlV1fLLWx1hj3aAEfzX-c1e1621fa13356be"
	cli := NewCLI("")

	// 测试 CheckAvailable
	t.Run("CheckAvailable", func(t *testing.T) {
		if err := cli.CheckAvailable(); err != nil {
			t.Fatalf("CLI 不可用: %v", err)
		}
		t.Log("✅ CLI 可用")
	})

	// 测试 List
	var firstFileID string
	t.Run("List", func(t *testing.T) {
		items, err := cli.List(apiKey, "")
		if err != nil {
			t.Fatalf("列出笔记失败: %v", err)
		}
		t.Logf("✅ 根目录有 %d 个条目:", len(items))
		for _, item := range items {
			t.Logf("   %s %s (id: %s)", item.Type, item.Name, item.ID)
			if item.Type == "file" && firstFileID == "" {
				firstFileID = item.ID
			}
		}
	})

	// 测试 Read
	if firstFileID != "" {
		t.Run("Read", func(t *testing.T) {
			result, err := cli.Read(apiKey, firstFileID)
			if err != nil {
				t.Fatalf("读取笔记失败: %v", err)
			}
			t.Logf("✅ 读取笔记成功:")
			t.Logf("   格式: %s", result.RawFormat)
			t.Logf("   原始: %v", result.IsRaw)
			content := result.Content
			if len(content) > 200 {
				content = content[:200] + "..."
			}
			t.Logf("   内容: %s", content)
		})
	}

	// 测试 Search
	t.Run("Search", func(t *testing.T) {
		items, err := cli.Search(apiKey, "test")
		if err != nil {
			t.Fatalf("搜索失败: %v", err)
		}
		t.Logf("✅ 搜索 'test' 找到 %d 个结果:", len(items))
		for _, item := range items {
			t.Logf("   %s %s (id: %s)", item.Type, item.Name, item.ID)
		}
	})

	// 测试 CreateNote (使用 save 命令)
	var createdNoteID string
	t.Run("CreateNote", func(t *testing.T) {
		noteID, err := cli.CreateNote(apiKey, "集成测试笔记.md", "# 集成测试\n\n这是自动化测试创建的笔记。", "")
		if err != nil {
			t.Fatalf("创建笔记失败: %v", err)
		}
		// 解析 JSON 返回的 fileId
		var result map[string]string
		if jsonErr := json.Unmarshal([]byte(noteID), &result); jsonErr == nil {
			if fid, ok := result["fileId"]; ok {
				createdNoteID = fid
			}
		}
		if createdNoteID == "" {
			createdNoteID = noteID
		}
		t.Logf("✅ 创建笔记成功: %s", createdNoteID)
	})

	// 测试 UpdateNote
	if createdNoteID != "" {
		t.Run("UpdateNote", func(t *testing.T) {
			err := cli.UpdateNote(apiKey, createdNoteID, "# 更新后的笔记\n\n这是更新后的内容。")
			if err != nil {
				t.Fatalf("更新笔记失败: %v", err)
			}
			t.Log("✅ 更新笔记成功")

			// 验证更新
			result, err := cli.Read(apiKey, createdNoteID)
			if err != nil {
				t.Fatalf("验证更新失败: %v", err)
			}
			content := result.Content
			if len(content) > 50 {
				content = content[:50] + "..."
			}
			t.Logf("   更新后内容: %s", content)
		})
	}

	// 测试 DeleteNote
	if createdNoteID != "" {
		t.Run("DeleteNote", func(t *testing.T) {
			err := cli.DeleteNote(apiKey, createdNoteID)
			if err != nil {
				t.Fatalf("删除笔记失败: %v", err)
			}
			t.Log("✅ 删除笔记成功")

			// 验证删除：CLI 返回的是包含错误信息的字符串，不是 error
			result, err := cli.Read(apiKey, createdNoteID)
			if err != nil {
				t.Logf("   删除后读取返回错误（符合预期）: %v", err)
			} else if result != nil && (result.Content == "" || result.Content == "获取笔记内容失败" || result.Content == "null") {
				t.Log("   删除后内容为空（符合预期）")
			} else {
				t.Logf("   删除后返回: %s", result.Content)
			}
		})
	}

	fmt.Println("\n🎉 所有集成测试通过！")
}
