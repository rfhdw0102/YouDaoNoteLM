import { create } from 'zustand';
import type { User } from '../types';
import * as authApi from '../api/auth';
import * as userApi from '../api/user';

// 定时刷新用户信息的间隔（6 小时），用于更新 MinIO 预签名头像 URL（有效期 24 小时）
const PROFILE_REFRESH_INTERVAL = 6 * 60 * 60 * 1000;
let refreshTimer: ReturnType<typeof setInterval> | null = null;

function mapUser(u: any): User {
  return {
    id: String(u.id),
    email: u.email,
    nickname: u.nickname || u.username || '',
    avatar: u.avatar || undefined,
    role: (u.role as 'user' | 'admin') || 'user',
  };
}

interface AuthState {
  user: User | null;
  token: string | null;
  isAuthenticated: boolean;
  login: (email: string, password: string, captchaId: string, captchaX: number) => Promise<void>;
  register: (email: string, password: string, confirmPassword: string, code: string) => Promise<void>;
  sendCode: (email: string, type: 'register' | 'reset' | 'delete_account') => Promise<number>;
  resetPassword: (email: string, code: string, newPassword: string, confirmPassword: string) => Promise<void>;
  logout: () => Promise<void>;
  updateProfile: (data: Partial<User>) => void;
  restoreSession: () => void;
}

export const useAuthStore = create<AuthState>((set) => ({
  user: null,
  token: null,
  isAuthenticated: false,

  restoreSession: () => {
    const token = sessionStorage.getItem('access_token');
    const userStr = localStorage.getItem('user');
    if (token && userStr) {
      try {
        const user = JSON.parse(userStr);
        set({ user, token, isAuthenticated: true });
        // Silently refresh user profile from server in background (updates avatar presigned URL)
        userApi.getProfile().then((res) => {
          if (res.code === 0) {
            const updated = mapUser(res.data);
            localStorage.setItem('user', JSON.stringify(updated));
            set({ user: updated });
          }
        }).catch(() => {});
        // 定时刷新用户信息，保持头像 URL 有效
        if (refreshTimer) clearInterval(refreshTimer);
        refreshTimer = setInterval(() => {
          userApi.getProfile().then((res) => {
            if (res.code === 0) {
              const updated = mapUser(res.data);
              localStorage.setItem('user', JSON.stringify(updated));
              set({ user: updated });
            }
          }).catch(() => {});
        }, PROFILE_REFRESH_INTERVAL);
      } catch {
        sessionStorage.removeItem('access_token');
        localStorage.removeItem('user');
      }
    }
  },

  login: async (email, password, captchaId, captchaX) => {
    const res = await authApi.login({ email, password, captcha_id: captchaId, captcha_x: captchaX });
    if (res.code !== 0) {
      throw new Error(res.message);
    }
    const { access_token, refresh_token, user: userData } = res.data;
    const user = mapUser(userData);
    sessionStorage.setItem('access_token', access_token);
    localStorage.setItem('refresh_token', refresh_token);
    localStorage.setItem('user', JSON.stringify(user));
    set({ user, token: access_token, isAuthenticated: true });
    // 登录后启动定时刷新，保持头像 URL 有效
    if (refreshTimer) clearInterval(refreshTimer);
    refreshTimer = setInterval(() => {
      userApi.getProfile().then((r) => {
        if (r.code === 0) {
          const updated = mapUser(r.data);
          localStorage.setItem('user', JSON.stringify(updated));
          set({ user: updated });
        }
      }).catch(() => {});
    }, PROFILE_REFRESH_INTERVAL);
  },

  register: async (email, password, confirmPassword, code) => {
    const res = await authApi.register({
      email,
      password,
      confirm_password: confirmPassword,
      code,
    });
    if (res.code !== 0) {
      throw new Error(res.message);
    }
  },

  sendCode: async (email, type) => {
    const res = await authApi.sendCode({ email, type });
    if (res.code !== 0) {
      throw new Error(res.message);
    }
    return res.data.retry_after;
  },

  resetPassword: async (email, code, newPassword, confirmPassword) => {
    const res = await authApi.resetPassword({
      email,
      code,
      new_password: newPassword,
      confirm_password: confirmPassword,
    });
    if (res.code !== 0) {
      throw new Error(res.message);
    }
  },

  logout: async () => {
    const accessToken = sessionStorage.getItem('access_token') || undefined;
    const refreshToken = localStorage.getItem('refresh_token') || undefined;

    // 清除定时刷新
    if (refreshTimer) { clearInterval(refreshTimer); refreshTimer = null; }

    // 先清除本地状态，确保 UI 立即响应
    sessionStorage.removeItem('access_token');
    localStorage.removeItem('refresh_token');
    localStorage.removeItem('user');
    set({ user: null, token: null, isAuthenticated: false });

    // Call server logout to revoke tokens (best effort, don't block UI)
    try { await authApi.logout({ access_token: accessToken, refresh_token: refreshToken }); } catch {}
  },

  updateProfile: (data) => {
    // Update local state immediately
    set((state) => {
      const updated = state.user ? { ...state.user, ...data } : null;
      if (updated) localStorage.setItem('user', JSON.stringify(updated));
      return { user: updated };
    });
    // Sync to server (best effort)
    userApi.updateProfile({
      nickname: data.nickname,
      avatar: data.avatar,
    }).catch(() => {});
  },
}));
