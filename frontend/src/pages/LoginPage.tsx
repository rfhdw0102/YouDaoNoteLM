import { useState } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import { motion } from 'framer-motion';
import { Mail, Lock, Eye, EyeOff } from 'lucide-react';
import { useAuthStore } from '../stores/useAuthStore';
import Button from '../components/ui/Button';
import Input from '../components/ui/Input';
import SliderCaptcha from '../components/ui/SliderCaptcha';

export default function LoginPage() {
  const navigate = useNavigate();
  const { login } = useAuthStore();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [showCaptcha, setShowCaptcha] = useState(false);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!email || !password) {
      setError('请填写邮箱和密码');
      return;
    }
    setError('');
    setShowCaptcha(true);
  };

  const handleCaptchaVerified = async (captchaId: string, captchaX: number) => {
    setShowCaptcha(false);
    setLoading(true);
    setError('');
    try {
      await login(email, password, captchaId, captchaX);
      navigate('/');
    } catch (err: any) {
      setError(err.message || '登录失败，请检查邮箱和密码');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div>
      <h2 className="text-xl font-semibold text-text-primary mb-1">登录</h2>
      <p className="text-sm text-text-secondary mb-6">欢迎回来，请登录您的账户</p>

      <form onSubmit={handleSubmit} className="space-y-4">
        <Input label="邮箱" type="email" placeholder="your@email.com" icon={<Mail size={16} />}
          value={email} onChange={(e) => setEmail(e.target.value)} />
        <div className="relative">
          <Input label="密码" type={showPassword ? 'text' : 'password'} placeholder="输入密码" icon={<Lock size={16} />}
            value={password} onChange={(e) => setPassword(e.target.value)} />
          <button type="button" onClick={() => setShowPassword(!showPassword)}
            className="absolute right-3 top-[38px] text-text-muted hover:text-text-primary transition-colors cursor-pointer">
            {showPassword ? <EyeOff size={16} /> : <Eye size={16} />}
          </button>
        </div>

        {error && (
          <motion.p initial={{ opacity: 0, y: -4 }} animate={{ opacity: 1, y: 0 }}
            className="text-sm text-error bg-error/5 px-3 py-2 rounded-lg border border-error/20">{error}</motion.p>
        )}

        <div className="flex justify-end">
          <Link to="/forgot-password" className="text-sm text-accent hover:text-accent-light transition-colors">忘记密码？</Link>
        </div>

        <Button type="submit" className="w-full" size="lg" loading={loading}>登录</Button>
      </form>

      <p className="text-center text-sm text-text-secondary mt-6">
        还没有账户？ <Link to="/register" className="text-accent hover:text-accent-light font-medium transition-colors">立即注册</Link>
      </p>

      <SliderCaptcha open={showCaptcha} onClose={() => setShowCaptcha(false)} onVerified={handleCaptchaVerified} />
    </div>
  );
}
