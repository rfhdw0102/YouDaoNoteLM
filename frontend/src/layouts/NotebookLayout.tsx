import { Outlet, useNavigate, useLocation } from 'react-router-dom';
import { motion } from 'framer-motion';
import {
  BookOpen, Settings, User, LogOut, ChevronDown, Sun, Moon, Shield
} from 'lucide-react';
import { useState } from 'react';
import { useAuthStore } from '../stores/useAuthStore';
import { useThemeStore } from '../stores/useThemeStore';
import { cn } from '../utils/cn';
import AvatarImg from '../components/ui/AvatarImg';

export default function NotebookLayout() {
  const navigate = useNavigate();
  const location = useLocation();
  const { user, logout } = useAuthStore();
  const { theme, toggleTheme } = useThemeStore();

  const [showUserMenu, setShowUserMenu] = useState(false);

  return (
    <div className="h-screen flex flex-col bg-bg-primary">
      {/* Top Navbar */}
      <header className="h-14 flex items-center justify-between px-4 border-b border-border bg-bg-secondary/50 backdrop-blur-sm flex-shrink-0 z-30">
        <button
          onClick={() => navigate('/')}
          className="flex items-center gap-2 hover:opacity-80 transition-opacity cursor-pointer flex-shrink-0"
        >
          <BookOpen size={20} className="text-accent flex-shrink-0" />
          <span className="text-base font-bold text-text-primary whitespace-nowrap">
            YoudaoNoteLM
          </span>
        </button>

        <div className="flex items-center gap-1">
          {/* Theme toggle */}
          <button
            onClick={toggleTheme}
            className="p-2 rounded-lg text-text-muted hover:text-text-primary hover:bg-bg-hover transition-colors cursor-pointer"
            title={theme === 'dark' ? '切换亮色模式' : '切换深色模式'}
          >
            {theme === 'dark' ? <Sun size={18} /> : <Moon size={18} />}
          </button>

          <button
            onClick={() => navigate('/settings')}
            className={cn(
              'p-2 rounded-lg transition-colors cursor-pointer',
              location.pathname === '/settings'
                ? 'text-accent bg-accent-glow'
                : 'text-text-muted hover:text-text-primary hover:bg-bg-hover'
            )}
          >
            <Settings size={18} />
          </button>

          <div className="relative">
            <button
              onClick={() => setShowUserMenu(!showUserMenu)}
              className="flex items-center gap-2 p-1.5 rounded-lg hover:bg-bg-hover transition-colors cursor-pointer"
            >
              <div className="w-8 h-8 rounded-full bg-gradient-to-br from-accent/20 to-teal/20 border border-accent/30 flex items-center justify-center">
                {user?.avatar ? (
                  <AvatarImg
                    src={user.avatar}
                    className="w-full h-full rounded-full object-cover"
                    fallback={<User size={14} className="text-accent" />}
                  />
                ) : (
                  <User size={14} className="text-accent" />
                )}
              </div>
              <ChevronDown size={14} className="text-text-muted" />
            </button>

            {showUserMenu && (
              <>
                <div className="fixed inset-0 z-40" onClick={() => setShowUserMenu(false)} />
                <motion.div
                  initial={{ opacity: 0, y: -4 }}
                  animate={{ opacity: 1, y: 0 }}
                  className="absolute right-0 top-full mt-2 w-48 bg-bg-card border border-border-light rounded-xl shadow-xl z-50 py-1.5 overflow-hidden"
                >
                  <div className="px-4 py-2 border-b border-border">
                    <p className="text-sm font-medium text-text-primary">{user?.nickname}</p>
                    <p className="text-xs text-text-muted">{user?.email}</p>
                    {user?.role === 'admin' && (
                      <span className="inline-block mt-1 px-2 py-0.5 text-xs bg-warning/10 text-warning rounded-full">
                        管理员
                      </span>
                    )}
                  </div>
                  <button
                    onClick={() => { navigate('/profile'); setShowUserMenu(false); }}
                    className="w-full flex items-center gap-2.5 px-4 py-2 text-sm text-text-secondary hover:text-text-primary hover:bg-bg-hover transition-colors cursor-pointer"
                  >
                    <User size={14} /> 个人中心
                  </button>
                  <button
                    onClick={() => { navigate('/settings'); setShowUserMenu(false); }}
                    className="w-full flex items-center gap-2.5 px-4 py-2 text-sm text-text-secondary hover:text-text-primary hover:bg-bg-hover transition-colors cursor-pointer"
                  >
                    <Settings size={14} /> 设置
                  </button>
                  {user?.role === 'admin' && (
                    <button
                      onClick={() => { navigate('/admin'); setShowUserMenu(false); }}
                      className="w-full flex items-center gap-2.5 px-4 py-2 text-sm text-warning hover:bg-warning/5 transition-colors cursor-pointer"
                    >
                      <Shield size={14} /> 后台管理
                    </button>
                  )}
                  <div className="border-t border-border mt-1 pt-1">
                    <button
                      onClick={async () => {
                        setShowUserMenu(false);
                        await logout();
                        // logout 已清除 isAuthenticated，路由守卫会自动跳转到 /login
                      }}
                      className="w-full flex items-center gap-2.5 px-4 py-2 text-sm text-error hover:bg-error/5 transition-colors cursor-pointer"
                    >
                      <LogOut size={14} /> 退出登录
                    </button>
                  </div>
                </motion.div>
              </>
            )}
          </div>
        </div>
      </header>

      <main className="flex-1 overflow-hidden">
        <Outlet />
      </main>
    </div>
  );
}
