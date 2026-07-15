import { forwardRef, useState, type InputHTMLAttributes } from 'react';
import { Eye, EyeOff } from 'lucide-react';
import { cn } from '../../utils/cn';

interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  label?: string;
  error?: string;
  icon?: React.ReactNode;
}

const Input = forwardRef<HTMLInputElement, InputProps>(
  ({ className, label, error, icon, type, autoComplete, ...props }, ref) => {
    const [show, setShow] = useState(false);
    const isPassword = type === 'password';
    const effectiveType = isPassword ? (show ? 'text' : 'password') : type;

    return (
      <div className="flex flex-col gap-1.5">
        {label && (
          <label className="text-sm font-medium text-text-secondary">{label}</label>
        )}
        <div className="relative">
          {icon && (
            <div className="absolute left-3 top-1/2 -translate-y-1/2 text-text-muted">
              {icon}
            </div>
          )}
          {/* 隐藏的假 input 消耗浏览器自动填充（Chrome 会优先填这些，保护真实输入框不被填充登录密码） */}
          {isPassword && (
            <>
              <input type="text" style={{ display: 'none' }} autoComplete="username" aria-hidden tabIndex={-1} />
              <input type="password" style={{ display: 'none' }} autoComplete="new-password" aria-hidden tabIndex={-1} />
            </>
          )}
          <input
            ref={ref}
            type={effectiveType}
            autoComplete={autoComplete ?? (isPassword ? 'new-password' : 'off')}
            className={cn(
              'w-full h-10 rounded-lg border bg-bg-tertiary text-text-primary placeholder:text-text-muted',
              'border-border-light focus:border-accent focus:ring-1 focus:ring-accent/30',
              'transition-all duration-200 outline-none',
              'text-sm',
              icon ? 'pl-10' : 'pl-4',
              isPassword ? 'pr-10' : 'pr-4',
              error && 'border-error focus:border-error focus:ring-error/30',
              className
            )}
            {...props}
          />
          {isPassword && (
            <button
              type="button"
              onClick={() => setShow((s) => !s)}
              className="absolute right-3 top-1/2 -translate-y-1/2 text-text-muted hover:text-text-primary transition-colors flex items-center justify-center"
              aria-label={show ? '隐藏' : '显示'}
            >
              {show ? <EyeOff size={18} /> : <Eye size={18} />}
            </button>
          )}
        </div>
        {error && <p className="text-xs text-error">{error}</p>}
      </div>
    );
  }
);

Input.displayName = 'Input';
export default Input;
