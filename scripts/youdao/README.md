# 有道云笔记 Markdown 转换器

将有道云笔记的 XML/JSON 格式转换为标准 Markdown 格式。

来源：[YoudaoNote-pull](https://github.com/yoogodong/YoudaoNote-pull)

## 功能

- 支持 XML 格式笔记转换
- 支持 JSON 格式笔记转换（新版笔记）
- 自动检测输入格式
- 支持以下元素转换：
  - 标题（h1-h6）
  - 段落文本
  - 粗体、斜体、删除线
  - 图片
  - 代码块
  - 待办事项
  - 引用
  - 列表（有序/无序）
  - 表格
  - 附件链接

## 使用方式

### 1. 命令行使用

```bash
# 转换文件
python scripts/youdao/convert_to_markdown.py input.note output.md

# 如果不指定输出文件，会自动生成 .md 文件
python scripts/youdao/convert_to_markdown.py input.note
```

输出格式为 JSON：
```json
{
  "success": true,
  "input_file": "input.note",
  "output_file": "output.md",
  "content": "# 标题\n\n正文内容..."
}
```

### 2. Go 代码调用

```go
import "YoudaoNoteLm/internal/service/external/youdao"

// 创建转换器
converter := youdao.NewNoteConverter("scripts/youdao/convert_to_markdown.py")

// 方式1：直接转换内容
xmlContent := `<?xml version="1.0"?>...`
markdown, err := converter.ConvertToMarkdown(xmlContent, "xml")

// 方式2：转换文件
markdown, err := converter.ConvertFile("path/to/note.xml")
```

### 3. Python 代码调用

```python
from convert_to_markdown import convert_to_markdown

# 转换 XML 内容
xml_content = '<?xml version="1.0"?>...'
markdown = convert_to_markdown(xml_content, "xml")

# 转换 JSON 内容
json_content = '{"5": [...]}'
markdown = convert_to_markdown(json_content, "json")

# 自动检测格式
markdown = convert_to_markdown(content)
```

## 支持的笔记格式

### XML 格式

有道云笔记旧版导出格式，结构如下：

```xml
<?xml version="1.0" encoding="UTF-8"?>
<note>
  <note-info>...</note-info>
  <body>
    <para>普通文本</para>
    <heading level="1">标题</heading>
    <code><language>python</language><text>代码内容</text></code>
    ...
  </body>
</note>
```

### JSON 格式

有道云笔记新版格式，结构如下：

```json
{
  "5": [
    {
      "6": "h",
      "4": {"l": "h1"},
      "5": [{"7": [{"8": "标题内容"}]}]
    },
    ...
  ]
}
```

## 依赖

- Python 3.6+
- 无额外依赖（使用标准库）

## 注意事项

1. 转换脚本使用 Python 标准库，无需安装额外包
2. 图片链接保持原始 URL，不会自动下载
3. 转换后建议检查特殊字符是否正确转义
