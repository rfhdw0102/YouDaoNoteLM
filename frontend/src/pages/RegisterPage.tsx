import { useState } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import { motion } from 'framer-motion';
import { Mail, Lock, Shield } from 'lucide-react';
import { useAuthStore } from '../stores/useAuthStore';
import Button from '../components/ui/Button';
import Input from '../components/ui/Input';

export default function RegisterPage() {
  const navigate = useNavigate();
  const { register, sendCode } = useAuthStore();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [code, setCode] = useState('');
  const [loading, setLoading] = useState(false);
  const [sendingCode, setSendingCode] = useState(false);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');
  const [countdown, setCountdown] = useState(0);

  const handleSendCode = async () => {
    if (!email || countdown > 0) return;
    setError('');
    setSendingCode(true);
    try {
      const retryAfter = await sendCode(email, 'register');
      setCountdown(retryAfter || 60);
      const timer = setInterval(() => {
        setCountdown((prev) => {
          if (prev <= 1) { clearInterval(timer); return 0; }
          return prev - 1;
        });
      }, 1000);
    } catch (err: any) {
      setError(err.message || '发送验证码失败');
    } finally {
      setSendingCode(false);
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!email || !password || !confirmPassword || !code) {
      setError('请填写所有必填项');
      return;
    }
    if (password !== confirmPassword) {
      setError('两次密码输入不一致');
      return;
    }
    if (password.length < 8) {
      setError('密码长度至少 8 位');
      return;
    }
    setLoading(true);
    setError('');
    try {
      await register(email, password, confirmPassword, code);
      setSuccess('注册成功，即将跳转登录页...');
      setTimeout(() => navigate('/login'), 1500);
    } catch (err: any) {
      setError(err.message || '注册失败，请重试');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div>
      <h2 className="text-xl font-semibold text-text-primary mb-1">注册</h2>
      <p className="text-sm text-text-secondary mb-6">创建您的 YoudaoNoteLM 账户</p>

      <form onSubmit={handleSubmit} className="space-y-4">
        <Input label="邮箱" type="email" placeholder="your@email.com" icon={<Mail size={16} />}
          value={email} onChange={(e) => setEmail(e.target.value)} />

        <div className="flex gap-2">
          <div className="flex-1">
            <Input label="验证码" placeholder="6位验证码" icon={<Shield size={16} />}
              value={code} onChange={(e) => setCode(e.target.value)} />
          </div>
          <div className="flex items-end">
            <Button type="button" variant="secondary" onClick={handleSendCode}
              disabled={!email || countdown > 0} loading={sendingCode} className="h-10 whitespace-nowrap">
              {countdown > 0 ? `${countdown}s` : '获取验证码'}
            </Button>
          </div>
        </div>

        <Input label="密码" type="password" placeholder="至少 8 位，包含字母和数字" icon={<Lock size={16} />}
          value={password} onChange={(e) => setPassword(e.target.value)} />

        <Input label="确认密码" type="password" placeholder="再次输入密码" icon={<Lock size={16} />}
          value={confirmPassword} onChange={(e) => setConfirmPassword(e.target.value)} />

        {error && (
          <motion.p initial={{ opacity: 0, y: -4 }} animate={{ opacity: 1, y: 0 }}
            className="text-sm text-error bg-error/5 px-3 py-2 rounded-lg border border-error/20">{error}</motion.p>
        )}
        {success && (
          <motion.p initial={{ opacity: 0, y: -4 }} animate={{ opacity: 1, y: 0 }}
            className="text-sm text-success bg-success/5 px-3 py-2 rounded-lg border border-success/20">{success}</motion.p>
        )}

        <Button type="submit" className="w-full" size="lg" loading={loading}>注册</Button>
      </form>

      <p className="text-center text-sm text-text-secondary mt-6">
        已有账户？ <Link to="/login" className="text-accent hover:text-accent-light font-medium transition-colors">立即登录</Link>
      </p>
    </div>
  );
}
