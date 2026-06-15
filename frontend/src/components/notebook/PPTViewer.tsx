import { useState, useRef } from 'react';
import { Maximize2, Minimize2, Download } from 'lucide-react';
import Button from '../ui/Button';

interface PPTViewerProps {
  content: string;
}

export default function PPTViewer({ content }: PPTViewerProps) {
  const [isFullscreen, setIsFullscreen] = useState(false);
  const iframeRef = useRef<HTMLIFrameElement>(null);

  const handleFullscreen = () => {
    if (iframeRef.current) {
      if (!isFullscreen) {
        iframeRef.current.requestFullscreen?.();
      } else {
        document.exitFullscreen?.();
      }
      setIsFullscreen(!isFullscreen);
    }
  };

  const handleDownload = () => {
    const blob = new Blob([content], { type: 'text/html' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = 'presentation.html';
    a.click();
    URL.revokeObjectURL(url);
  };

  return (
    <div className="p-6">
      <div className="flex items-center gap-2 mb-3">
        <Button variant="ghost" size="sm" onClick={handleFullscreen}>
          {isFullscreen ? <Minimize2 size={12} /> : <Maximize2 size={12} />}
          {isFullscreen ? '退出全屏' : '全屏演示'}
        </Button>
        <Button variant="ghost" size="sm" onClick={handleDownload}>
          <Download size={12} /> 导出 HTML
        </Button>
      </div>

      <div className="rounded-xl overflow-hidden border border-border-light bg-bg-card">
        <iframe
          ref={iframeRef}
          srcDoc={content}
          className="w-full border-0"
          style={{ height: '500px' }}
          title="PPT 预览"
          sandbox="allow-same-origin"
        />
      </div>
    </div>
  );
}
