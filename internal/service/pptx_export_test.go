package service

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
)

func TestExportPPTXNotCorrupted(t *testing.T) {
	// 构建一个包含代码块的 PPT HTML
	html := `<style>
.ppt-slide { width: 1920px; height: 1080px; overflow: hidden; padding: 80px; background: #f8fafc; }
h2 { font-size: 48px; }
p { font-size: 32px; }
.ppt-code-block { background: #1e293b; color: #e2e8f0; padding: 20px; font-family: Consolas, monospace; font-size: 24px; white-space: pre-wrap; }
</style>
<section class="ppt-slide" data-ppt-slide="true">
  <h2>光合作用</h2>
  <p>植物通过光合作用将光能转化为化学能</p>
</section>
<section class="ppt-slide" data-ppt-slide="true">
  <h2>代码示例</h2>
  <p>Go语言函数定义：</p>
  <pre class="ppt-code-block"><code>func main() {
    fmt.Println("hello")
}</code></pre>
</section>`

	// 测试 Go 纯实现路径 (buildDynamicHTMLPPTX)
	data, err := buildDynamicHTMLPPTX(html, "test-export")
	if err != nil {
		t.Fatalf("buildDynamicHTMLPPTX 失败: %v", err)
	}
	t.Logf("PPTX 大小: %d bytes", len(data))

	// 验证 PPTX 结构
	if err := validatePPTXStructure(data, t); err != nil {
		t.Fatalf("PPTX 结构验证失败: %v", err)
	}

	// 保存到临时文件供手动验证
	tempDir, _ := os.MkdirTemp("", "pptx-validate-*")
	path := tempDir + "/test_export.pptx"
	os.WriteFile(path, data, 0644)
	t.Logf("PPTX 保存到: %s", path)
}

func TestExportPPTXWithDOMFallback(t *testing.T) {
	// 测试 exportPPTWithDefaultEngine 路径
	// 当 Playwright 不可用时，应 fallback 到 buildDynamicHTMLPPTX
	html := `<section class="ppt-slide" data-ppt-slide="true"><h2>测试</h2><p>内容</p></section>`

	data, err := exportPPTWithDefaultEngine(context.Background(), html, "test-dom")
	if err != nil {
		t.Fatalf("exportPPTWithDefaultEngine 失败: %v", err)
	}
	t.Logf("PPTX 大小: %d bytes", len(data))

	// 验证 PK 签名
	if len(data) < 2 || data[0] != 0x50 || data[1] != 0x4b {
		t.Fatalf("不是有效的 PPTX (ZIP) 文件")
	}

	if err := validatePPTXStructure(data, t); err != nil {
		t.Fatalf("PPTX 结构验证失败: %v", err)
	}
}

func validatePPTXStructure(data []byte, t *testing.T) error {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("ZIP 读取失败: %w", err)
	}

	existing := make(map[string]bool)
	for _, f := range reader.File {
		existing[f.Name] = true
	}

	// 检查必需文件
	required := []string{
		"[Content_Types].xml",
		"ppt/presentation.xml",
		"ppt/_rels/presentation.xml.rels",
		"ppt/slideMasters/slideMaster1.xml",
	}
	for _, name := range required {
		if !existing[name] {
			return fmt.Errorf("缺少必需文件: %s", name)
		}
	}

	// 计算幻灯片数量
	slideCount := 0
	for _, f := range reader.File {
		if strings.HasPrefix(f.Name, "ppt/slides/slide") &&
			strings.HasSuffix(f.Name, ".xml") &&
			!strings.Contains(f.Name, "_rels") {
			slideCount++
		}
	}
	t.Logf("幻灯片数量: %d", slideCount)

	// 检查每个 slide 的 rels
	for i := 1; i <= slideCount; i++ {
		relsName := fmt.Sprintf("ppt/slides/_rels/slide%d.xml.rels", i)
		if !existing[relsName] {
			return fmt.Errorf("缺少 slide rels: %s", relsName)
		}
	}

	// 检查 Content_Types.xml
	for _, f := range reader.File {
		if f.Name == "[Content_Types].xml" {
			rc, _ := f.Open()
			content, _ := io.ReadAll(rc)
			rc.Close()
			ct := string(content)
			for i := 1; i <= slideCount; i++ {
				partName := fmt.Sprintf("/ppt/slides/slide%d.xml", i)
				if !strings.Contains(ct, partName) {
					t.Errorf("[Content_Types].xml 缺少 slide%d 的 Override", i)
				}
			}
		}
	}

	// 检查 presentation.xml 中的 sldId
	for _, f := range reader.File {
		if f.Name == "ppt/presentation.xml" {
			rc, _ := f.Open()
			content, _ := io.ReadAll(rc)
			rc.Close()
			pxml := string(content)
			for i := 1; i <= slideCount; i++ {
				sldId := fmt.Sprintf(`rId%d"`, i+2)
				if !strings.Contains(pxml, sldId) {
					t.Errorf("presentation.xml 缺少 slide%d 的 sldId (rId%d)", i, i+2)
				}
			}
		}
	}

	return nil
}
