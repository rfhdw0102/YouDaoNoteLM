import { type InputHTMLAttributes } from 'react';
import { cn } from '../../utils/cn';

interface CheckboxProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'type'> {
  label?: string;
}

export default function Checkbox({ label, className, ...props }: CheckboxProps) {
  return (
    <label className={cn('inline-flex items-center gap-2 cursor-pointer select-none group', className)}>
      <div className="relative">
        <input type="checkbox" className="peer sr-only" {...props} />
        <div className={cn(
          'w-4.5 h-4.5 rounded border-2 border-border-light bg-bg-tertiary',
          'peer-checked:bg-accent peer-checked:border-accent',
          'peer-focus-visible:ring-2 peer-focus-visible:ring-accent/30',
          'transition-all duration-200',
          'group-hover:border-accent/50'
        )}>
          <svg
            className="absolute inset-0 w-full h-full text-white opacity-0 peer-checked:opacity-100 transition-opacity p-0.5"
            viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2"
          >
            <path d="M3 8l3.5 3.5L13 5" strokeLinecap="round" strokeLinejoin="round" />
          </svg>
        </div>
        {/* Checkmark via CSS */}
        <style>{`
          input:checked + div svg { opacity: 1; }
        `}</style>
      </div>
      {label && <span className="text-sm text-text-secondary group-hover:text-text-primary transition-colors">{label}</span>}
    </label>
  );
}
