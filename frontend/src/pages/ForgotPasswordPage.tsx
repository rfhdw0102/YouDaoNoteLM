import { useState } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import { motion } from 'framer-motion';
import { Mail, Lock, Shield } from 'lucide-react';
import { useAuthStore } from '../stores/useAuthStore';
import Button from '../components/ui/Button';
import Input from '../components/ui/Input';

export default function ForgotPasswordPage() {
  const navigate = useNavigate();
  const { sendCode, resetPassword } = useAuthStore();
  const [step, setStep] = useState<'email' | 'code' | 'done'>('email');
  const [email, setEmail] = useState('');
  const [code, setCode] = useState('');
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [loading, setLoading] = useState(false);
  const [sendingCode, setSendingCode] = useState(false);
  const [error, setError] = useState('');
  const [countdown, setCountdown] = useState(0);

  const handleSendCode = async () => {
    if (!email || countdown > 0) return;
    setError('');
    setSendingCode(true);
    try {
      const retryAfter = await sendCode(email, 'reset');
      setStep('code');
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
    if (!code || !password || !confirmPassword) {
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
      await resetPassword(email, code, password, confirmPassword);
      setStep('done');
    } catch (err: any) {
      setError(err.message || '重置失败，请重试');
    } finally {
      setLoading(false);
    }
  };

  if (step === 'done') {
    return (
      <div className="text-center py-4">
        <motion.div initial={{ scale: 0 }} animate={{ scale: 1 }}
          className="w-16 h-16 mx-auto mb-4 rounded-full bg-success/10 flex items-center justify-center">
          <svg className="w-8 h-8 text-success" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
            <path d="M5 13l4 4L19 7" strokeLinecap="round" strokeLinejoin="round" />
          </svg>
        </motion.div>
        <h3 className="text-lg font-semibold text-text-primary mb-2">密码重置成功</h3>
        <p className="text-sm text-text-secondary mb-6">您的密码已重置，请使用新密码登录</p>
        <Button onClick={() => navigate('/login')} className="w-full">前往登录</Button>
      </div>
    );
  }

  return (
    <div>
      <h2 className="text-xl font-semibold text-text-primary mb-1">找回密码</h2>
      <p className="text-sm text-text-secondary mb-6">
        {step === 'email' ? '输入您的注册邮箱' : '输入验证码和新密码'}
      </p>

      {step === 'email' ? (
        <div className="space-y-4">
          <Input label="邮箱" type="email" placeholder="your@email.com" icon={<Mail size={16} />}
            value={email} onChange={(e) => setEmail(e.target.value)} />
          {error && (
            <motion.p initial={{ opacity: 0 }} animate={{ opacity: 1 }}
              className="text-sm text-error bg-error/5 px-3 py-2 rounded-lg border border-error/20">{error}</motion.p>
          )}
          <Button onClick={handleSendCode} className="w-full" size="lg" disabled={!email} loading={sendingCode}>
            发送验证码
          </Button>
        </div>
      ) : (
        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="flex gap-2">
            <div className="flex-1">
              <Input label="验证码" placeholder="6位验证码" icon={<Shield size={16} />}
                value={code} onChange={(e) => setCode(e.target.value)} />
            </div>
            <div className="flex items-end">
              <Button type="button" variant="secondary" onClick={handleSendCode}
                disabled={countdown > 0} loading={sendingCode} className="h-10 whitespace-nowrap">
                {countdown > 0 ? `${countdown}s` : '重新发送'}
              </Button>
            </div>
          </div>
          <Input label="新密码" type="password" placeholder="至少 8 位" icon={<Lock size={16} />}
            value={password} onChange={(e) => setPassword(e.target.value)} />
          <Input label="确认新密码" type="password" placeholder="再次输入密码" icon={<Lock size={16} />}
            value={confirmPassword} onChange={(e) => setConfirmPassword(e.target.value)} />
          {error && (
            <motion.p initial={{ opacity: 0 }} animate={{ opacity: 1 }}
              className="text-sm text-error bg-error/5 px-3 py-2 rounded-lg border border-error/20">{error}</motion.p>
          )}
          <Button type="submit" className="w-full" size="lg" loading={loading}>重置密码</Button>
        </form>
      )}

      <p className="text-center text-sm text-text-secondary mt-6">
        <Link to="/login" className="text-accent hover:text-accent-light font-medium transition-colors">返回登录</Link>
      </p>
    </div>
  );
}
