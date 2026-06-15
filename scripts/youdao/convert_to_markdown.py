#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
有道云笔记格式转换器
将有道云笔记的 XML/JSON 格式转换为 Markdown

来源: https://github.com/yoogodong/YoudaoNote-pull
"""

import json
import os
import sys
import xml.etree.ElementTree as ET
from typing import Optional


class XmlElementConvert:
    """XML Element 转换规则"""

    @staticmethod
    def get_text_by_key(element_children, key="text"):
        """获取元素的文本内容"""
        for sub_element in element_children:
            if key in sub_element.tag:
                return sub_element.text if sub_element.text else ""
        return ""

    @staticmethod
    def get_element_text(element) -> str:
        """获取元素的直接文本内容（包括子元素的文本）"""
        # 首先尝试获取元素自身的 text
        if element.text:
            return element.text
        # 如果没有，尝试查找 <text> 子元素
        for child in element:
            if "text" in child.tag:
                return child.text if child.text else ""
        return ""

    @staticmethod
    def _encode_string_to_md(original_text: str) -> str:
        """转义特殊字符"""
        if not original_text or original_text == " ":
            return original_text

        original_text = original_text.replace("\\", "\\\\")
        original_text = original_text.replace("*", "\\*")
        original_text = original_text.replace("_", "\\_")
        original_text = original_text.replace("#", "\\#")
        original_text = original_text.replace("&", "&amp;")
        original_text = original_text.replace("<", "&lt;")
        original_text = original_text.replace(">", "&gt;")
        original_text = original_text.replace('"', "&quot;")
        original_text = original_text.replace("'", "&apos;")
        original_text = original_text.replace("\t", "&emsp;")
        original_text = original_text.replace("\r\n", "<br>")
        original_text = original_text.replace("\n\r", "<br>")
        original_text = original_text.replace("\r", "<br>")
        original_text = original_text.replace("\n", "<br>")
        return original_text

    @staticmethod
    def convert_para_func(**kwargs):
        """普通文本"""
        return kwargs.get("text")

    @staticmethod
    def convert_heading_func(**kwargs):
        """标题"""
        level = kwargs.get("element").attrib.get("level", 0)
        level = 1 if level in ("a", "b") else level
        text = kwargs.get("text")
        try:
            h_level = int(level)
        except (ValueError, TypeError):
            h_level = 1
        h_level = max(1, min(6, h_level))
        return " ".join(["#" * h_level, text.strip()]) if text else text

    @staticmethod
    def convert_image_func(**kwargs):
        """图片"""
        image_url = XmlElementConvert.get_text_by_key(
            list(kwargs.get("element")), "source"
        )
        return f"![{kwargs.get('text')}]({image_url})"

    @staticmethod
    def convert_attach_func(**kwargs):
        """附件"""
        element = kwargs.get("element")
        filename = XmlElementConvert.get_text_by_key(list(element), "filename")
        resource_url = XmlElementConvert.get_text_by_key(list(element), "resource")
        return f"[{filename}]({resource_url})"

    @staticmethod
    def convert_code_func(**kwargs):
        """代码块"""
        language = XmlElementConvert.get_text_by_key(
            list(kwargs.get("element")), "language"
        )
        return f"```{language}\r\n{kwargs.get('text')}```"

    @staticmethod
    def convert_todo_func(**kwargs):
        """待办事项"""
        element = kwargs.get("element")
        state = element.attrib.get("state", "todo")
        checkbox = "- [x]" if state == "done" else "- [ ]"
        return f"{checkbox} {kwargs.get('text')}"

    @staticmethod
    def convert_quote_func(**kwargs):
        """引用"""
        return f"> {kwargs.get('text')}"

    @staticmethod
    def convert_horizontal_line_func(**kwargs):
        """分割线"""
        return "---"

    @staticmethod
    def convert_list_item_func(**kwargs):
        """列表"""
        list_id = kwargs.get("element").attrib["list-id"]
        is_ordered = kwargs.get("list_item").get(list_id)
        text = kwargs.get("text")
        if is_ordered == "unordered":
            return f"- {text}"
        elif is_ordered == "ordered":
            return f"1. {text}"

    @staticmethod
    def convert_table_func(**kwargs):
        """表格"""
        element = kwargs.get("element")
        content = XmlElementConvert.get_text_by_key(element, "content")

        table_data_str = ""
        nl = "\r\n"
        table_data = json.loads(content)
        table_data_len = len(table_data["widths"])
        table_data_arr = []
        table_data_line = []

        for cells in table_data["cells"]:
            values = cells.get("value")
            if values is None:
                values = ""
            cell_value = XmlElementConvert._encode_string_to_md(values)
            table_data_line.append(cell_value)
            if len(table_data_line) == table_data_len:
                table_data_arr.append(table_data_line)
                table_data_line = []

        if len(table_data_arr) == 1:
            table_data_arr.insert(0, [" " for _ in range(table_data_len)])
            table_data_arr.insert(1, ["-" for _ in range(table_data_len)])
        elif len(table_data_arr) > 1:
            table_data_arr.insert(1, ["-" for _ in range(table_data_len)])

        for table_line in table_data_arr:
            table_data_str += "|"
            for table_data in table_line:
                table_data_str += f" {table_data} |"
            table_data_str += nl

        return table_data_str


class JsonConvert:
    """JSON 转换规则"""

    def _get_common_text(self, content: dict) -> str:
        all_text = ""
        five_contents = content.get("5")
        if five_contents:
            seven_contents = five_contents[0].get("7")
            if not seven_contents:
                return all_text
            for seven_content in seven_contents:
                text = seven_content.get("8")
                text_attrs = seven_content.get("9")
                if text and text_attrs:
                    text = self._convert_text_attribute(text, text_attrs)
                all_text += text if text else ""
        return all_text

    def _convert_text_attribute(self, text: str, text_attrs: list):
        if isinstance(text_attrs, list) and text_attrs and text:
            for attr in text_attrs:
                if attr["2"] == "b":
                    text = f"**{text}**"
                elif attr["2"] == "i":
                    text = f"*{text}*"
                elif attr["2"] == "s":
                    text = f"~~{text}~~"
        return text

    def convert_text_func(self, content) -> str:
        """普通文本"""
        all_text = ""
        one_five_contents = content.get("5")
        if one_five_contents:
            for one_five_content in one_five_contents:
                two_five_contents = one_five_content.get("5")
                text_type = one_five_content.get("6")
                seven_contents = one_five_content.get("7")

                if seven_contents and not two_five_contents:
                    text = ""
                    for seven_content in seven_contents:
                        raw = seven_content.get("8")
                        text_attrs = seven_content.get("9")
                        if raw and text_attrs:
                            raw = self._convert_text_attribute(raw, text_attrs)
                        text += raw if raw else ""
                elif (text_type == "li" or text_type == "nli") and two_five_contents:
                    source_text = self._get_common_text(one_five_content)
                    four_contents = one_five_content.get("4")
                    if four_contents:
                        url = four_contents.get("hf") or four_contents.get("id") or four_contents.get("rid")
                        if url:
                            text = f"[{source_text}]({url})"
                        else:
                            text = source_text
                    else:
                        text = source_text
                else:
                    text = self._get_common_text(one_five_content)

                if text:
                    all_text += text
        return all_text

    def convert_h_func(self, content) -> str:
        """标题"""
        type_name = content.get("4").get("l")
        text = self._get_common_text(content=content)
        if text and type_name:
            level = int(type_name.replace("h", ""))
            text = " ".join(["#" * int(level), text.strip()])
        return text

    def convert_im_func(self, content):
        """图片"""
        image_url = content["4"]["u"]
        return f"![]({image_url})"

    def convert_a_func(self, content):
        """附件"""
        fn = content["4"]["fn"]
        fl = content["4"]["re"]
        return f"[{fn}]({fl})"

    def convert_cd_func(self, content):
        """代码块"""
        language = content.get("4").get("la")
        codes: list = content.get("5")
        code_block = ""
        for code in codes:
            text = self._get_common_text(code)
            code_block += text + "\n"
        return f"```{language}\r\n{code_block}```"

    def convert_la_func(self, content):
        """高亮块"""
        lines: list = content.get("5")
        highlight_block = ""
        for line in lines:
            text = self._get_common_text(line)
            highlight_block += text + "\n"
        return f"```\r\n{highlight_block}```"

    def convert_q_func(self, content):
        """引用"""
        q_text_list = content["5"]
        text = ""
        for q_text_dict in q_text_list:
            q_text = self._get_common_text(q_text_dict)
            q_text = q_text.replace("\n", "")
            text += f"> {q_text}\n"
        return text

    def convert_l_func(self, content):
        """列表"""
        text = self._get_common_text(content=content)
        is_ordered = content.get("4").get("lt")
        if is_ordered == "unordered":
            level = content.get("4").get("ll", 1)
            return "  " * (level - 1) + f"- {text}"
        elif is_ordered == "ordered":
            return f"1. {text}"

    def convert_todo_func(self, content):
        """待办事项"""
        text = self._get_common_text(content=content)
        ls = content.get("4", {}).get("ls", "todo")
        checkbox = "- [x]" if ls == "done" else "- [ ]"
        return f"{checkbox} {text}"

    def convert_t_func(self, content):
        """表格"""
        nl = "\r\n"
        tr_list = content["5"]
        table_lines = ""
        for index, tc in enumerate(tr_list):
            table_content_list = tc["5"]
            table_content_len = len(table_content_list)
            if index == 1:
                table_line = "| -- " * table_content_len + "|\n| "
            else:
                table_line = "| "
            for table_content in table_content_list:
                try:
                    table_text_list = table_content.get("5")[0].get("5")[0].get("7")
                    table_text = table_text_list[0]["8"] if table_text_list else " "
                except (KeyError, IndexError, TypeError):
                    table_text = " "
                table_line = table_line + table_text + " | "
            table_lines = table_lines + table_line + nl
        return table_lines


def convert_xml_to_markdown(xml_content: str) -> str:
    """将 XML 格式转换为 Markdown"""
    try:
        root = ET.fromstring(xml_content)
    except ET.ParseError as e:
        raise ValueError(f"XML 解析失败: {e}")

    # 获取列表定义
    list_item = {}

    # 查找 body 元素
    body_element = None

    # 遍历根元素的子元素
    for child in root:
        if "body" in child.tag:
            body_element = child
        elif "note-info" in child.tag:
            # 在 note-info 中查找列表定义
            for subchild in child:
                if "list" in subchild.tag:
                    list_item[subchild.attrib["id"]] = subchild.attrib["type"]

    # 如果没有找到 body，尝试使用第二个子元素
    if body_element is None:
        if len(root) >= 2:
            body_element = root[1]
        elif len(root) == 1:
            # 如果只有一个子元素，可能是 body 本身
            body_element = root[0]
        else:
            return ""

    new_content_list = []

    for element in list(body_element):
        # 获取元素文本（支持直接文本和 <text> 子元素）
        text = XmlElementConvert.get_element_text(element)
        tag_raw = element.tag.split("}")[-1]
        name = "todo" if "todo" in tag_raw else tag_raw.replace("-", "_")

        convert_func = getattr(XmlElementConvert, f"convert_{name}_func", None)
        if not convert_func:
            new_content_list.append(text)
            continue

        line_content = convert_func(text=text, element=element, list_item=list_item)
        new_content_list.append(line_content)

    return "\r\n\r\n".join(filter(None, new_content_list))


def convert_json_to_markdown(json_content: str) -> str:
    """将 JSON 格式转换为 Markdown"""
    try:
        json_data = json.loads(json_content)
    except json.JSONDecodeError as e:
        raise ValueError(f"JSON 解析失败: {e}")

    new_content_list = []
    json_contents = json_data.get("5", [])

    converter = JsonConvert()
    for content in json_contents:
        ctype = content.get("6")

        if ctype and "todo" in ctype:
            line_content = converter.convert_todo_func(content)
        elif ctype:
            convert_func = getattr(converter, f"convert_{ctype}_func", None)
            line_content = convert_func(content) if convert_func else converter.convert_text_func(content)
        else:
            line_content = converter.convert_text_func(content)

        if line_content:
            new_content_list.append(line_content)

    return "\r\n\r\n".join(new_content_list)


def detect_format(content: str) -> str:
    """检测内容格式"""
    content_stripped = content.strip()
    if content_stripped.startswith("<?xml") or content_stripped.startswith("<"):
        return "xml"
    elif content_stripped.startswith("{"):
        return "json"
    else:
        return "unknown"


def convert_to_markdown(content: str, format_type: Optional[str] = None) -> str:
    """
    将有道云笔记内容转换为 Markdown

    Args:
        content: 原始内容（XML 或 JSON 格式）
        format_type: 格式类型，可选值: "xml", "json"。不指定则自动检测

    Returns:
        转换后的 Markdown 内容
    """
    if not content:
        return ""

    if format_type is None:
        format_type = detect_format(content)

    if format_type == "xml":
        return convert_xml_to_markdown(content)
    elif format_type == "json":
        return convert_json_to_markdown(content)
    else:
        # 如果无法识别格式，返回原内容
        return content


def main():
    """命令行入口"""
    if len(sys.argv) < 2:
        print("用法: python convert_to_markdown.py <input_file> [output_file]")
        print("  input_file: 输入文件路径（XML 或 JSON 格式）")
        print("  output_file: 输出文件路径（可选，默认为输入文件名.md）")
        sys.exit(1)

    input_file = sys.argv[1]

    if not os.path.exists(input_file):
        print(f"错误: 文件不存在 - {input_file}")
        sys.exit(1)

    # 确定输出文件路径
    if len(sys.argv) >= 3:
        output_file = sys.argv[2]
    else:
        base_name = os.path.splitext(input_file)[0]
        output_file = f"{base_name}.md"

    try:
        # 读取输入文件
        with open(input_file, "r", encoding="utf-8") as f:
            content = f.read()

        # 转换
        markdown_content = convert_to_markdown(content)

        # 写入输出文件
        with open(output_file, "w", encoding="utf-8") as f:
            f.write(markdown_content)

        # 输出结果（JSON 格式，便于程序解析）
        result = {
            "success": True,
            "input_file": input_file,
            "output_file": output_file,
            "content": markdown_content
        }
        print(json.dumps(result, ensure_ascii=False))

    except Exception as e:
        result = {
            "success": False,
            "error": str(e)
        }
        print(json.dumps(result, ensure_ascii=False))
        sys.exit(1)


if __name__ == "__main__":
    main()
