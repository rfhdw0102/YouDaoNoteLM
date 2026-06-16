import { useState, type ReactNode } from 'react';
import { cn } from '../../utils/cn';

interface TooltipProps {
  content: string;
  children: ReactNode;
  side?: 'top' | 'bottom';
  className?: string;
}

export default function Tooltip({ content, children, side = 'top', className }: TooltipProps) {
  const [visible, setVisible] = useState(false);

  return (
    <div
      className="relative inline-flex"
      onMouseEnter={() => setVisible(true)}
      onMouseLeave={() => setVisible(false)}
    >
      {children}
      {visible && (
        <div
          className={cn(
            'absolute z-50 px-2.5 py-1.5 rounded-lg text-xs font-medium whitespace-nowrap',
            'bg-bg-card text-text-primary border border-border-light shadow-lg',
            'pointer-events-none',
            side === 'top' && 'bottom-full left-1/2 -translate-x-1/2 mb-2',
            side === 'bottom' && 'top-full left-1/2 -translate-x-1/2 mt-2',
            className
          )}
        >
          {content}
          <div
            className={cn(
              'absolute left-1/2 -translate-x-1/2 w-2 h-2 rotate-45 bg-bg-card border-border-light',
              side === 'top' && 'top-full -mt-1 border-r border-b',
              side === 'bottom' && 'bottom-full -mb-1 border-l border-t',
            )}
          />
        </div>
      )}
    </div>
  );
}
