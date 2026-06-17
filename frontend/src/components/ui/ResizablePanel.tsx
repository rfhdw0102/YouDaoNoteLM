import { useState, useRef, useEffect, type ReactNode } from 'react';

interface ResizablePanelProps {
  children: ReactNode;
  defaultWidth?: number;
  minWidth?: number;
  maxWidth?: number;
  direction?: 'left' | 'right';
  className?: string;
}

export default function ResizablePanel({
  children,
  defaultWidth = 25,
  minWidth = 15,
  maxWidth = 40,
  direction = 'right',
  className = ''
}: ResizablePanelProps) {
  const [width, setWidth] = useState(defaultWidth);
  const [isResizing, setIsResizing] = useState(false);
  const panelRef = useRef<HTMLDivElement>(null);
  const startXRef = useRef(0);
  const startWidthRef = useRef(0);

  useEffect(() => {
    const handleMouseMove = (e: MouseEvent) => {
      if (!isResizing) return;

      const dx = e.clientX - startXRef.current;
      const newWidth = direction === 'right'
        ? startWidthRef.current + (dx / window.innerWidth) * 100
        : startWidthRef.current - (dx / window.innerWidth) * 100;

      const clampedWidth = Math.min(Math.max(newWidth, minWidth), maxWidth);
      setWidth(clampedWidth);
    };

    const handleMouseUp = () => {
      setIsResizing(false);
      document.body.style.cursor = '';
      document.body.style.userSelect = '';
    };

    if (isResizing) {
      document.addEventListener('mousemove', handleMouseMove);
      document.addEventListener('mouseup', handleMouseUp);
      document.body.style.cursor = 'col-resize';
      document.body.style.userSelect = 'none';
    }

    return () => {
      document.removeEventListener('mousemove', handleMouseMove);
      document.removeEventListener('mouseup', handleMouseUp);
    };
  }, [isResizing, direction, minWidth, maxWidth]);

  const handleMouseDown = (e: React.MouseEvent) => {
    e.preventDefault();
    setIsResizing(true);
    startXRef.current = e.clientX;
    startWidthRef.current = width;
  };

  return (
    <div
      ref={panelRef}
      className={`relative flex-shrink-0 ${className}`}
      style={{ width: `${width}%` }}
    >
      {children}
      <div
        className={`absolute top-0 bottom-0 w-1 cursor-col-resize hover:bg-accent/50 transition-colors z-10 ${
          direction === 'right' ? 'right-0' : 'left-0'
        }`}
        onMouseDown={handleMouseDown}
      />
    </div>
  );
}