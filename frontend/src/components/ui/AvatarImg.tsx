import { useState, useEffect } from 'react';

interface AvatarImgProps {
  src: string;
  alt?: string;
  className?: string;
  fallback: React.ReactNode;
}

/**
 * 头像图片组件，自动处理 MinIO 预签名 URL 过期后的图片加载失败，
 * 失败时显示 fallback 内容。
 */
export default function AvatarImg({ src, alt = '', className, fallback }: AvatarImgProps) {
  const [error, setError] = useState(false);

  // src 变化时重置错误状态
  useEffect(() => {
    setError(false);
  }, [src]);

  if (error) return <>{fallback}</>;

  return (
    <img src={src} alt={alt} className={className} onError={() => setError(true)} />
  );
}
