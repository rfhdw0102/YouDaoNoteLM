import { useState, useRef, useEffect, useCallback } from 'react';
import { Maximize2, Minimize2, Download, ArrowLeft } from 'lucide-react';
import Button from '../ui/Button';
import { exportGenerationFile, downloadBlob } from '../../api/generation';

interface PPTViewerProps {
  content: string;
  onClose?: () => void;
}

export default function PPTViewer({ content, onClose }: PPTViewerProps) {
  const [isFullscreen, setIsFullscreen] = useState(false);
  const [exporting, setExporting] = useState(false);
  const [exportError, setExportError] = useState<string | null>(null);
  const containerRef = useRef<HTMLDivElement>(null);

  // 进入全屏演示模式
  const enterFullscreen = useCallback(() => {
    const el = containerRef.current;
    if (el) {
      el.requestFullscreen?.();
      setIsFullscreen(true);
    }
  }, []);

  // 退出全屏
  const exitFullscreen = useCallback(() => {
    if (document.fullscreenElement) {
      document.exitFullscreen?.();
    }
    setIsFullscreen(false);
  }, []);

  // 监听浏览器全屏事件保持状态同步（Esc退出时）
  useEffect(() => {
    const handleFSChange = () => {
      const inFS = !!document.fullscreenElement;
      setIsFullscreen(inFS);
    };
    document.addEventListener('fullscreenchange', handleFSChange);
    return () => document.removeEventListener('fullscreenchange', handleFSChange);
  }, []);

  const handleDownload = async () => {
    if (exporting) return;
    setExporting(true);
    setExportError(null);
    try {
      const file = await exportGenerationFile({
        type: 'ppt',
        content,
        title: 'presentation',
      });
      downloadBlob(file.blob, file.filename);
    } catch (err) {
      console.error('PPT export failed:', err);
      const msg = err instanceof Error ? err.message : '导出失败，请稍后重试';
      setExportError(msg);
    } finally {
      setExporting(false);
    }
  };

  // 全屏模式下的 PPT 预览界面
  const fullscreenView = (
    <div
      ref={containerRef}
      className="w-screen h-screen bg-black flex flex-col"
    >
      {/* 全屏控制栏 */}
      <div className="flex items-center justify-between px-6 py-3 bg-bg-card/80 backdrop-blur-sm border-b border-border-light flex-shrink-0 z-10">
        <div className="flex items-center gap-2">
          {onClose && (
            <Button variant="ghost" size="sm" onClick={() => { exitFullscreen(); onClose(); }}>
              <ArrowLeft size={14} /> 返回
            </Button>
          )}
          <span className="text-sm text-text-secondary">PPT 全屏演示</span>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="ghost" size="sm" onClick={handleDownload} disabled={exporting}>
            <Download size={14} /> {exporting ? '导出中...' : '导出 PPTX'}
          </Button>
          <Button variant="ghost" size="sm" onClick={exitFullscreen}>
            <Minimize2 size={14} /> 退出全屏
          </Button>
        </div>
      </div>

      {/* PPT 内容区 */}
      <div className="flex-1 overflow-auto bg-bg-subtle">
        <iframe
          srcDoc={content}
          className="w-full h-full border-0 block"
          title="PPT 全屏演示"
          sandbox="allow-same-origin allow-scripts"
        />
      </div>
    </div>
  );

  // 如果正在全屏，直接渲染全屏视图
  if (isFullscreen) {
    return fullscreenView;
  }

  // 非全屏模式：只显示操作按钮，不内嵌预览
  return (
    <div
      ref={containerRef}
      className="flex flex-col h-full p-4"
    >
      {/* 操作按钮区 */}
      <div className="flex items-center gap-3 mb-4 flex-shrink-0">
        <Button variant="primary" size="sm" onClick={enterFullscreen}>
          <Maximize2 size={14} /> 全屏演示
        </Button>
        <Button variant="ghost" size="sm" onClick={handleDownload} disabled={exporting}>
          <Download size={14} /> {exporting ? '导出中...' : '导出 PPTX'}
        </Button>
        {exportError && (
          <span className="text-xs text-red-400">{exportError}</span>
        )}
      </div>

      {/* PPT 缩略预览区 - 只提供缩略提示，点击进入全屏 */}
      <div
        className="flex-1 min-h-0 rounded-2xl overflow-hidden border border-border-light bg-bg-subtle shadow-sm cursor-pointer group relative"
        onClick={enterFullscreen}
      >
        <iframe
          srcDoc={content}
          className="w-full h-full border-0 block pointer-events-none"
          title="PPT 缩略预览"
          sandbox="allow-same-origin allow-scripts"
          style={{ transform: 'scale(0.95)', transformOrigin: 'center center' }}
        />
        {/* 点击全屏提示覆盖层 */}
        <div className="absolute inset-0 bg-bg-subtle/40 opacity-0 group-hover:opacity-100 transition-opacity flex items-center justify-center">
          <div className="flex items-center gap-2 px-4 py-2 rounded-lg bg-bg-card border border-border-light shadow-lg text-sm text-text-primary">
            <Maximize2 size={16} /> 点击进入全屏演示
          </div>
        </div>
      </div>
    </div>
  );
}
