import { Outlet } from 'react-router-dom';
import { motion } from 'framer-motion';
import { BookOpen } from 'lucide-react';

export default function AuthLayout() {
  return (
    <div className="min-h-screen flex items-center justify-center bg-bg-primary relative overflow-hidden">
      {/* Background */}
      <div className="absolute inset-0 overflow-hidden">
        <div className="absolute top-1/4 -left-32 w-96 h-96 bg-accent/4 rounded-full blur-3xl" />
        <div className="absolute bottom-1/4 -right-32 w-96 h-96 bg-teal/4 rounded-full blur-3xl" />
      </div>

      <motion.div
        initial={{ opacity: 0, y: 20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.5, ease: [0.22, 1, 0.36, 1] }}
        className="relative z-10 w-full max-w-md mx-4"
      >
        {/* Logo */}
        <div className="text-center mb-8">
          <div className="inline-flex items-center gap-2.5 mb-2">
            <BookOpen size={24} className="text-accent" />
            <h1 className="text-2xl font-bold text-text-primary">
              YoudaoNoteLM
            </h1>
          </div>
          <p className="text-text-secondary text-sm">智能知识管理与学习助手</p>
        </div>

        {/* Card */}
        <div className="bg-bg-secondary/80 backdrop-blur-xl rounded-2xl border border-border-light shadow-2xl shadow-black/20 p-8">
          <Outlet />
        </div>
      </motion.div>
    </div>
  );
}
