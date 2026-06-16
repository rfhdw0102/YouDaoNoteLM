import { useState } from 'react';
import { motion } from 'framer-motion';
import { ZoomIn, ZoomOut, RotateCcw } from 'lucide-react';

interface MindmapViewerProps {
  content: string;
}

interface MindNode {
  label: string;
  children: MindNode[];
  color: string;
}

const COLORS = ['#6C63FF', '#4ECDC4', '#FF6B6B', '#FFD93D', '#6BCB77', '#C084FC', '#F97316', '#60A5FA'];

function parseMindmap(markdown: string): MindNode | null {
  const lines = markdown.split('\n').filter((l) => l.trim());
  if (lines.length === 0) return null;

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

  if (!tree) {
    return (
      <div className="flex items-center justify-center h-64 text-text-muted text-sm">
        无法解析思维导图内容
      </div>
    );
  }

  return (
    <div className="p-6">
      {/* Controls */}
      <div className="flex items-center gap-2 mb-4">
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
      </div>

      {/* Mindmap */}
      <motion.div
        style={{ transform: `scale(${zoom})`, transformOrigin: 'top left' }}
        className="bg-bg-card rounded-xl border border-border-light p-6 min-h-[300px]"
      >
        <MindNodeComponent node={tree} />
      </motion.div>
    </div>
  );
}
