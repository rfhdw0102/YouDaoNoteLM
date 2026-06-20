import { useState } from 'react';
import { motion } from 'framer-motion';
import { ZoomIn, ZoomOut, RotateCcw, Maximize2, Minimize2 } from 'lucide-react';

interface MindmapViewerProps {
  content: string;
}

interface MindNode {
  label: string;
  children: MindNode[];
  color: string;
}

const COLORS = ['#6C63FF', '#4ECDC4', '#FF6B6B', '#FFD93D', '#6BCB77', '#C084FC', '#F97316', '#60A5FA'];

/**
 * 解析 Markmap 兼容的 Markdown 格式思维导图
 * 支持 #/##/###/#### 标题层级和 - 缩进列表两种格式
 */
function parseMindmap(markdown: string): MindNode | null {
  const lines = markdown.split('\n').filter((l) => l.trim());
  if (lines.length === 0) return null;

  // 检测是否使用标题格式（以 # 开头的行）
  const hasHeading = lines.some((l) => /^\s*#{1,6}\s/.test(l));

  if (hasHeading) {
    return parseHeadingFormat(lines);
  }

  // 回退到缩进列表格式
  return parseIndentFormat(lines);
}

/**
 * 解析标题格式的 Markdown（#/##/###/####）
 * 后端 renderMindmap 输出格式：
 * # 标题
 * ## 分支
 * ### 节点
 * #### 细节
 */
function parseHeadingFormat(lines: string[]): MindNode | null {
  const root: MindNode = { label: '', children: [], color: COLORS[0] };
  // stack 记录每一级的父节点和标题级别
  const stack: { node: MindNode; level: number }[] = [{ node: root, level: 0 }];

  for (const line of lines) {
    const trimmed = line.trimStart();
    // 匹配标题行
    const headingMatch = trimmed.match(/^(#{1,6})\s+(.+)/);
    if (headingMatch) {
      const level = headingMatch[1].length;
      const label = headingMatch[2].trim();
      if (!label) continue;

      // 弹出栈中 level >= 当前 level 的节点
      while (stack.length > 1 && stack[stack.length - 1].level >= level) {
        stack.pop();
      }

      const depth = stack.length;
      const node: MindNode = {
        label,
        children: [],
        color: COLORS[depth % COLORS.length],
      };

      stack[stack.length - 1].node.children.push(node);
      stack.push({ node, level });
      continue;
    }

    // 匹配列表行（- 或 * 开头），作为最后一个栈节点的子节点
    const listMatch = trimmed.match(/^[-*]\s+(.+)/);
    if (listMatch) {
      const label = listMatch[1].trim();
      if (!label) continue;

      const parent = stack[stack.length - 1];
      if (parent && parent.node) {
        const depth = stack.length;
        parent.node.children.push({
          label,
          children: [],
          color: COLORS[depth % COLORS.length],
        });
      }
      continue;
    }

    // 处理非标题非列表但有缩进的行（作为描述性文本附加到上一个节点）
    // 不做特殊处理，跳过空行和纯文本行
  }

  // 如果根节点只有一个子节点，提升为根
  if (root.children.length === 1) {
    return root.children[0];
  }
  // 如果根节点没有标签但有子节点，返回根
  if (root.children.length > 0) {
    return root;
  }
  return null;
}

/**
 * 解析缩进列表格式（原始格式）
 * - 根节点
 *   - 子节点1
 *     - 细节1
 *   - 子节点2
 */
function parseIndentFormat(lines: string[]): MindNode | null {
  const root: MindNode = { label: '', children: [], color: COLORS[0] };
  const stack: { node: MindNode; indent: number }[] = [{ node: root, indent: -1 }];

  for (const line of lines) {
    const trimmed = line.trimStart();
    const indent = line.length - trimmed.length;
    const label = trimmed.replace(/^#+\s*/, '').replace(/^[-*]\s*/, '').replace(/^\d+\.\s*/, '');
    if (!label) continue;

    while (stack.length > 1 && stack[stack.length - 1].indent >= indent) {
      stack.pop();
    }

    const depth = stack.length;
    const node: MindNode = {
      label,
      children: [],
      color: COLORS[depth % COLORS.length],
    };

    stack[stack.length - 1].node.children.push(node);
    stack.push({ node, indent });
  }

  return root.children.length === 1 ? root.children[0] : root;
}

function MindNodeComponent({ node, depth = 0, index = 0 }: { node: MindNode; depth?: number; index?: number }) {
  const [expanded, setExpanded] = useState(depth < 3);
  const hasChildren = node.children.length > 0;

  return (
    <motion.div
      initial={{ opacity: 0, x: -10 }}
      animate={{ opacity: 1, x: 0 }}
      transition={{ delay: index * 0.05 }}
      className="relative"
    >
      <div className="flex items-start gap-2">
        {/* Connector line */}
        {depth > 0 && (
          <div
            className="absolute left-0 top-0 h-full w-px"
            style={{ backgroundColor: `${node.color}30`, marginLeft: '-12px' }}
          />
        )}

        {/* Node */}
        <div
          className="flex items-center gap-2 cursor-pointer group"
          onClick={() => hasChildren && setExpanded(!expanded)}
        >
          {hasChildren && (
            <div
              className="w-4 h-4 rounded-full flex items-center justify-center text-[10px] font-bold flex-shrink-0 transition-transform"
              style={{
                backgroundColor: `${node.color}20`,
                color: node.color,
                transform: expanded ? 'rotate(90deg)' : 'rotate(0deg)',
              }}
            >
              {expanded ? '−' : '+'}
            </div>
          )}
          {!hasChildren && (
            <div className="w-1.5 h-1.5 rounded-full flex-shrink-0" style={{ backgroundColor: node.color }} />
          )}
          <span
            className="text-sm font-medium transition-colors"
            style={{ color: depth === 0 ? node.color : 'var(--color-text-primary)' }}
          >
            {node.label}
          </span>
        </div>
      </div>

      {/* Children */}
      {expanded && hasChildren && (
        <motion.div
          initial={{ height: 0, opacity: 0 }}
          animate={{ height: 'auto', opacity: 1 }}
          className="ml-6 mt-1 space-y-1 border-l border-border-light pl-3"
        >
          {node.children.map((child, i) => (
            <MindNodeComponent key={i} node={child} depth={depth + 1} index={i} />
          ))}
        </motion.div>
      )}
    </motion.div>
  );
}

export default function MindmapViewer({ content }: MindmapViewerProps) {
  const tree = parseMindmap(content);
  const [zoom, setZoom] = useState(1);
  const [isFullscreen, setIsFullscreen] = useState(false);
  const containerRef = useState<HTMLDivElement | null>(null);

  if (!tree) {
    return (
      <div className="flex items-center justify-center h-64 text-text-muted text-sm">
        无法解析思维导图内容
      </div>
    );
  }

  const handleFullscreen = () => {
    const el = document.querySelector('[data-mindmap-container]');
    if (el) {
      if (!document.fullscreenElement) {
        el.requestFullscreen?.();
        setIsFullscreen(true);
      } else {
        document.exitFullscreen?.();
        setIsFullscreen(false);
      }
    }
  };

  return (
    <div className="p-6 h-full flex flex-col" data-mindmap-container>
      {/* Controls */}
      <div className="flex items-center gap-2 mb-4 flex-shrink-0">
        <button
          onClick={() => setZoom(Math.max(0.5, zoom - 0.1))}
          className="p-1.5 rounded-lg bg-bg-card border border-border-light text-text-muted hover:text-text-primary transition-colors cursor-pointer"
        >
          <ZoomOut size={14} />
        </button>
        <span className="text-xs text-text-muted w-12 text-center">{Math.round(zoom * 100)}%</span>
        <button
          onClick={() => setZoom(Math.min(2, zoom + 0.1))}
          className="p-1.5 rounded-lg bg-bg-card border border-border-light text-text-muted hover:text-text-primary transition-colors cursor-pointer"
        >
          <ZoomIn size={14} />
        </button>
        <button
          onClick={() => setZoom(1)}
          className="p-1.5 rounded-lg bg-bg-card border border-border-light text-text-muted hover:text-text-primary transition-colors cursor-pointer"
        >
          <RotateCcw size={14} />
        </button>
        <button
          onClick={handleFullscreen}
          className="p-1.5 rounded-lg bg-bg-card border border-border-light text-text-muted hover:text-text-primary transition-colors cursor-pointer"
        >
          {isFullscreen ? <Minimize2 size={14} /> : <Maximize2 size={14} />}
        </button>
      </div>

      {/* Mindmap - 外层 overflow-auto 实现缩放后滚动查看 */}
      <div className="flex-1 overflow-auto rounded-xl border border-border-light bg-bg-card" style={{ minHeight: 300 }}>
        <motion.div
          style={{ transform: `scale(${zoom})`, transformOrigin: 'top left' }}
          className="p-6 inline-block min-w-max"
        >
          <MindNodeComponent node={tree} />
        </motion.div>
      </div>
    </div>
  );
}
