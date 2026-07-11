#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""测试转换功能"""

import sys
import os

# 添加脚本目录到路径
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from convert_to_markdown import convert_to_markdown

def test_xml():
    """测试 XML 转换"""
    xml_content = '''<?xml version="1.0" encoding="UTF-8"?>
<note>
  <note-info></note-info>
  <body>
    <heading level="1">Test Title</heading>
    <para>This is normal text</para>
    <code><language>python</language><text>print("hello")</text></code>
    <todo state="done">Done task</todo>
    <todo state="todo">Todo task</todo>
  </body>
</note>'''

    result = convert_to_markdown(xml_content, 'xml')
    print("=== XML Conversion Result ===")
    print(result)
    assert "# Test Title" in result
    assert "This is normal text" in result
    assert "- [x] Done task" in result
    assert "- [ ] Todo task" in result
    print("XML test passed!\n")

def test_json():
    """测试 JSON 转换"""
    json_content = '''
{
  "5": [
    {
      "6": "h",
      "4": {"l": "h1"},
      "5": [{"7": [{"8": "JSON Title"}]}]
    },
    {
      "6": null,
      "5": [{"7": [{"8": "Normal text content"}]}]
    }
  ]
}'''

    result = convert_to_markdown(json_content, 'json')
    print("=== JSON Conversion Result ===")
    print(result)
    assert "# JSON Title" in result
    assert "Normal text content" in result
    print("JSON test passed!\n")

def test_auto_detect():
    """测试自动格式检测"""
    xml_content = '<?xml version="1.0"?><note><body><para>Auto detect XML</para></body></note>'
    result = convert_to_markdown(xml_content)
    print("=== Auto Detect XML ===")
    print(result)
    assert "Auto detect XML" in result
    print("Auto detect test passed!\n")

if __name__ == "__main__":
    test_xml()
    test_json()
    test_auto_detect()
    print("All tests passed!")
