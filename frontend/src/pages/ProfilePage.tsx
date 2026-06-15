import { useState, useRef } from 'react';
import { useNavigate } from 'react-router-dom';
import { motion } from 'framer-motion';
import { User, Mail, Lock, Camera, ArrowLeft, Save, Check, Loader2, AlertTriangle, Shield } from 'lucide-react';
import { useAuthStore } from '../stores/useAuthStore';
import { uploadAvatar, changeUsername, changePassword } from '../api/user';
import { deleteAccount } from '../api/notebook';
import Button from '../components/ui/Button';
import Input from '../components/ui/Input';
import Modal from '../components/ui/Modal';
import AvatarImg from '../components/ui/AvatarImg';
import { getErrorMessage } from '../utils/error';

export default function ProfilePage() {
  const navigate = useNavigate();
  const { user, updateProfile, logout, sendCode } = useAuthStore();
  const fileInputRef = useRef<HTMLInputElement>(null);

  const [nickname, setNickname] = useState(user?.nickname || '');
  const [oldPassword, setOldPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [uploadingAvatar, setUploadingAvatar] = useState(false);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');

  // Account deletion state
  const [showDeleteModal, setShowDeleteModal] = useState(false);
  const [deletePassword, setDeletePassword] = useState('');
  const [deleteCode, setDeleteCode] = useState('');
  const [deleteCountdown, setDeleteCountdown] = useState(0);
  const [deleting, setDeleting] = useState(false);

  const handleSendDeleteCode = async () => {
    if (deleteCountdown > 0) return;
    try {
      const retryAfter = await sendCode(user!.email, 'delete_account');
      setDeleteCountdown(retryAfter || 60);
      const timer = setInterval(() => {
        setDeleteCountdown((prev) => {
          if (prev <= 1) { clearInterval(timer); return 0; }
          return prev - 1;
        });
      }, 1000);
    } catch (err: any) {
      setError(getErrorMessage(err, '发送验证码失败'));
    }
  };

  const handleDeleteAccount = async () => {
    if (!deletePassword || !deleteCode) return;
    setDeleting(true);
    setError('');
    try {
      const res = await deleteAccount({ password: deletePassword, code: deleteCode });
      if (res.code === 0) {
        await logout();
        navigate('/login');
      } else {
        setError(res.message);
      }
    } catch (err: any) {
      setError(getErrorMessage(err, '注销失败'));
    } finally {
      setDeleting(false);
    }
  };

  const handleAvatarClick = () => fileInputRef.current?.click();

  const handleAvatarChange = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    if (!['image/jpeg', 'image/png'].includes(file.type)) {
      setError('仅支持 jpg/jpeg/png 格式');
      return;
    }
    if (file.size > 2 * 1024 * 1024) {
      setError('头像文件大小不能超过 2MB');
      return;
    }
    setError('');
    setUploadingAvatar(true);
    try {
      const res = await uploadAvatar(file);
      if (res.code === 0) {
        // 上传接口已在服务端更新头像，只需更新本地状态（不回传 URL 到 PUT /user/profile）
        useAuthStore.setState((state) => {
          const updated = state.user ? { ...state.user, avatar: res.data.avatar } : null;
          if (updated) localStorage.setItem('user', JSON.stringify(updated));
          return { user: updated };
        });
        setSuccess('头像上传成功');
        setTimeout(() => setSuccess(''), 2000);
      } else {
        setError(res.message);
      }
    } catch (err: any) {
      setError(getErrorMessage(err, '上传失败'));
    } finally {
      setUploadingAvatar(false);
    }
  };

  const handleSaveUsername = async () => {
    if (!nickname.trim() || nickname.trim() === user?.nickname) return;
    if (nickname.trim().length < 3) {
      setError('用户名至少 3 位');
      return;
    }
    setSaving(true);
    setError('');
    try {
      const res = await changeUsername(nickname.trim());
      if (res.code === 0) {
        updateProfile({ nickname: nickname.trim() });
        setSaved(true);
        setTimeout(() => setSaved(false), 2000);
      } else {
        setError(res.message);
      }
    } catch (err: any) {
      setError(getErrorMessage(err, '修改失败'));
    } finally {
      setSaving(false);
    }
  };

  const handleChangePassword = async () => {
    if (!oldPassword || !newPassword) {
      setError('请填写当前密码和新密码');
      return;
    }
    if (newPassword.length < 8) {
      setError('新密码长度至少 8 位');
      return;
    }
    setSaving(true);
    setError('');
    try {
      const res = await changePassword({ old_password: oldPassword, new_password: newPassword });
      if (res.code === 0) {
        setSuccess('密码修改成功，请重新登录');
        setOldPassword('');
        setNewPassword('');
        setTimeout(async () => {
          await logout();
          navigate('/login');
        }, 1500);
      } else {
        setError(res.message);
      }
    } catch (err: any) {
      setError(getErrorMessage(err, '修改失败'));
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="h-full overflow-y-auto">
      <div className="max-w-2xl mx-auto px-8 py-8">
        {/* Header */}
        <div className="flex items-center gap-3 mb-8">
          <button onClick={() => navigate(-1)} className="p-2 rounded-lg text-text-muted hover:text-text-primary hover:bg-bg-hover transition-colors cursor-pointer">
            <ArrowLeft size={18} />
          </button>
          <User size={22} className="text-accent" />
          <h1 className="text-xl font-bold text-text-primary">个人中心</h1>
        </div>

        {/* Avatar */}
        <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }}
          className="bg-bg-card rounded-xl border border-border-light p-6 mb-6">
          <div className="flex items-center gap-6">
            <div className="relative group cursor-pointer" onClick={handleAvatarClick}>
              <div className="w-20 h-20 rounded-full bg-gradient-to-br from-accent/30 to-teal/30 border-2 border-accent/30 flex items-center justify-center overflow-hidden">
                {uploadingAvatar ? (
                  <Loader2 size={24} className="animate-spin text-accent" />
                ) : user?.avatar ? (
                  <AvatarImg
                    src={user.avatar}
                    className="w-full h-full object-cover"
                    fallback={<User size={32} className="text-accent" />}
                  />
                ) : (
                  <User size={32} className="text-accent" />
                )}
              </div>
              <div className="absolute inset-0 rounded-full bg-black/50 opacity-0 group-hover:opacity-100 flex items-center justify-center transition-opacity">
                <Camera size={18} className="text-white" />
              </div>
              <input ref={fileInputRef} type="file" accept="image/jpeg,image/png" className="hidden" onChange={handleAvatarChange} />
            </div>
            <div>
              <h3 className="text-base font-semibold text-text-primary">{user?.nickname || '未设置昵称'}</h3>
              <p className="text-sm text-text-muted">{user?.email}</p>
              <p className="text-xs text-text-muted mt-1">支持 jpg/jpeg/png，≤ 2MB，点击头像上传</p>
            </div>
          </div>
        </motion.div>

        {/* Username */}
        <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.1 }}
          className="bg-bg-card rounded-xl border border-border-light p-6 mb-6">
          <h3 className="text-sm font-semibold text-text-primary mb-4">修改用户名</h3>
          <div className="space-y-4">
            <Input label="用户名" value={nickname} onChange={(e) => setNickname(e.target.value)} icon={<User size={16} />}
              onKeyDown={(e) => e.key === 'Enter' && handleSaveUsername()} />
            <Input label="邮箱" value={user?.email || ''} disabled icon={<Mail size={16} />} className="opacity-60" />
          </div>
          <div className="flex justify-end mt-4">
            <Button onClick={handleSaveUsername} loading={saving} size="sm"
              disabled={!nickname.trim() || nickname.trim() === user?.nickname}>
              {saved ? <><Check size={13} /> 已保存</> : <><Save size={13} /> 保存</>}
            </Button>
          </div>
        </motion.div>

        {/* Change password */}
        <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.2 }}
          className="bg-bg-card rounded-xl border border-border-light p-6">
          <h3 className="text-sm font-semibold text-text-primary mb-4">修改密码</h3>
          <div className="space-y-4">
            <Input label="当前密码" type="password" placeholder="输入当前密码" icon={<Lock size={16} />}
              value={oldPassword} onChange={(e) => setOldPassword(e.target.value)} />
            <Input label="新密码" type="password" placeholder="至少 8 位" icon={<Lock size={16} />}
              value={newPassword} onChange={(e) => setNewPassword(e.target.value)} />
          </div>
          <div className="flex justify-end mt-4">
            <Button onClick={handleChangePassword} loading={saving} size="sm"
              disabled={!oldPassword || !newPassword}>
              修改密码
            </Button>
          </div>
        </motion.div>

        {/* Account Deletion */}
        <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.3 }}
          className="bg-bg-card rounded-xl border border-error/20 p-6 mt-6">
          <div className="flex items-center gap-2 mb-2">
            <AlertTriangle size={16} className="text-error" />
            <h3 className="text-sm font-semibold text-error">危险操作</h3>
          </div>
          <p className="text-xs text-text-muted mb-4">注销账号将永久删除您的所有数据，包括笔记本、会话、资料来源等，此操作不可撤销。</p>
          <Button variant="danger" size="sm" onClick={() => setShowDeleteModal(true)}>注销账号</Button>
        </motion.div>

        {/* Messages */}
        {error && (
          <motion.div initial={{ opacity: 0, y: 8 }} animate={{ opacity: 1, y: 0 }}
            className="mt-4 text-sm text-error bg-error/5 px-4 py-2.5 rounded-lg border border-error/20">{error}</motion.div>
        )}
        {success && (
          <motion.div initial={{ opacity: 0, y: 8 }} animate={{ opacity: 1, y: 0 }}
            className="mt-4 text-sm text-success bg-success/5 px-4 py-2.5 rounded-lg border border-success/20">{success}</motion.div>
        )}
      </div>

      {/* Delete Account Modal */}
      <Modal open={showDeleteModal} onClose={() => { setShowDeleteModal(false); setDeletePassword(''); setDeleteCode(''); }}
        title="注销账号" size="sm">
        <div className="space-y-4">
          <div className="flex items-start gap-3 p-3 rounded-lg bg-error/5 border border-error/20">
            <AlertTriangle size={18} className="text-error flex-shrink-0 mt-0.5" />
            <p className="text-sm text-error">此操作将永久删除您的账号及所有数据，且无法恢复。</p>
          </div>
          <Input label="当前密码" type="password" placeholder="输入密码确认" icon={<Lock size={16} />}
            value={deletePassword} onChange={(e) => setDeletePassword(e.target.value)} />
          <div className="flex gap-2">
            <div className="flex-1">
              <Input label="邮箱验证码" placeholder="6位验证码" icon={<Shield size={16} />}
                value={deleteCode} onChange={(e) => setDeleteCode(e.target.value)} />
            </div>
            <div className="flex items-end">
              <Button type="button" variant="secondary" onClick={handleSendDeleteCode}
                disabled={deleteCountdown > 0} className="h-10 whitespace-nowrap">
                {deleteCountdown > 0 ? `${deleteCountdown}s` : '获取验证码'}
              </Button>
            </div>
          </div>
          <div className="flex justify-end gap-2 pt-2">
            <Button variant="ghost" onClick={() => { setShowDeleteModal(false); setDeletePassword(''); setDeleteCode(''); }}>取消</Button>
            <Button variant="danger" onClick={handleDeleteAccount} loading={deleting}
              disabled={!deletePassword || !deleteCode}>确认注销</Button>
          </div>
        </div>
      </Modal>
    </div>
  );
}
