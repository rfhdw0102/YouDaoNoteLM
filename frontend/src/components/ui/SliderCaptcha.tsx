import { useState, useRef, useCallback, useEffect } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { Check, X, Loader2, RotateCcw } from 'lucide-react';
import { getCaptcha, type CaptchaData } from '../../api/auth';

interface SliderCaptchaProps {
  open: boolean;
  onClose: () => void;
  onVerified: (captchaId: string, captchaX: number) => void;
}

export default function SliderCaptcha({ open, onClose, onVerified }: SliderCaptchaProps) {
  const [captcha, setCaptcha] = useState<CaptchaData | null>(null);
  const [loading, setLoading] = useState(false);
  const [sliderX, setSliderX] = useState(0);
  const [isDragging, setIsDragging] = useState(false);
  const [verified, setVerified] = useState(false);
  const [failed, setFailed] = useState(false);
  const [imgLoaded, setImgLoaded] = useState(false);

  const trackRef = useRef<HTMLDivElement>(null);
  const bgImgRef = useRef<HTMLImageElement>(null);
  const startXRef = useRef(0);

  const fetchCaptcha = useCallback(async () => {
    setLoading(true);
    setSliderX(0);
    setVerified(false);
    setFailed(false);
    setImgLoaded(false);
    try {
      const res = await getCaptcha();
      if (res.code === 0) {
        setCaptcha(res.data);
      }
    } catch (err) {
      console.error('Failed to fetch captcha:', err);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (open) fetchCaptcha();
  }, [open, fetchCaptcha]);

  const SLIDER_SIZE = captcha?.slider_size ?? 40;
  const BG_WIDTH = captcha?.bg_width ?? 300;
  const BG_HEIGHT = captcha?.bg_height ?? 150;
  const START_X = captcha?.slider_start_x ?? 0;
  const START_Y = captcha?.slider_start_y;

  // Track display width
  const TRACK_WIDTH = 320;
  const MAX_X = TRACK_WIDTH - SLIDER_SIZE;

  // Calculate slider Y position in CSS pixels
  // Depends on imgLoaded so it recalculates after the image renders
  const getSliderTop = (): string => {
    if (START_Y !== undefined && bgImgRef.current && imgLoaded) {
      const renderedHeight = bgImgRef.current.clientHeight;
      const scale = renderedHeight / BG_HEIGHT;
      return `${START_Y * scale}px`;
    }
    // Default: center vertically
    return '50%';
  };

  const getSliderTransform = (): string => {
    if (START_Y !== undefined && imgLoaded) {
      return 'translateY(0)';
    }
    return 'translateY(-50%)';
  };

  // Convert display X to captcha_x (drag distance relative to slider_start_x)
  // sliderX is in TRACK_WIDTH display pixels, map to BG_WIDTH image pixels
  // Then subtract START_X because backend will add it back: absoluteX = START_X + captcha_x
  const displayToCaptchaX = (displayX: number): number => {
    const sliderAbsoluteX = (displayX / TRACK_WIDTH) * BG_WIDTH;
    return Math.round(sliderAbsoluteX - START_X);
  };

  const handlePointerDown = useCallback((e: React.PointerEvent) => {
    if (verified || !captcha) return;
    setIsDragging(true);
    setFailed(false);
    startXRef.current = e.clientX - sliderX;
    (e.target as HTMLElement).setPointerCapture(e.pointerId);
  }, [sliderX, verified, captcha]);

  const handlePointerMove = useCallback((e: React.PointerEvent) => {
    if (!isDragging) return;
    const newX = Math.max(0, Math.min(MAX_X, e.clientX - startXRef.current));
    setSliderX(newX);
  }, [isDragging, MAX_X]);

  const handlePointerUp = useCallback(() => {
    if (!isDragging || !captcha) return;
    setIsDragging(false);

    const captchaX = displayToCaptchaX(sliderX);
    console.log('[Captcha] display X:', sliderX, '→ captcha_x:', captchaX, 'bg_width:', BG_WIDTH, 'start_x:', START_X);

    setVerified(true);
    setTimeout(() => {
      onVerified(captcha.captcha_id, captchaX);
    }, 300);
  }, [isDragging, sliderX, captcha, BG_WIDTH, START_X, onVerified]);

  const handleRetry = () => {
    fetchCaptcha();
  };

  return (
    <AnimatePresence>
      {open && (
        <div className="fixed inset-0 z-50 flex items-center justify-center">
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            className="absolute inset-0 bg-black/60 backdrop-blur-sm"
            onClick={onClose}
          />
          <motion.div
            initial={{ opacity: 0, scale: 0.9 }}
            animate={{ opacity: 1, scale: 1 }}
            exit={{ opacity: 0, scale: 0.9 }}
            className="relative z-10 w-[380px] bg-bg-secondary rounded-2xl border border-border-light shadow-2xl p-6"
          >
            {/* Header */}
            <div className="flex items-center justify-between mb-4">
              <h3 className="text-base font-semibold text-text-primary">安全验证</h3>
              <div className="flex items-center gap-1">
                <button onClick={handleRetry} className="p-1 rounded-lg text-text-muted hover:text-text-primary hover:bg-bg-hover transition-colors cursor-pointer" title="刷新验证码">
                  <RotateCcw size={16} />
                </button>
                <button onClick={onClose} className="p-1 rounded-lg text-text-muted hover:text-text-primary hover:bg-bg-hover transition-colors cursor-pointer">
                  <X size={18} />
                </button>
              </div>
            </div>

            {/* Captcha image area */}
            <div className="relative rounded-xl overflow-hidden mb-5 border border-border bg-bg-tertiary" style={{ width: `${TRACK_WIDTH}px` }}>
              {loading ? (
                <div className="flex items-center justify-center py-12">
                  <Loader2 size={24} className="animate-spin text-accent" />
                </div>
              ) : captcha ? (
                <>
                  {/* Background image - fixed width to match track */}
                  <img
                    ref={bgImgRef}
                    src={captcha.background}
                    alt=""
                    className="w-full block"
                    draggable={false}
                    onLoad={() => setImgLoaded(true)}
                  />
                  {/* Slider piece - draggable */}
                  <img
                    src={captcha.slider}
                    alt=""
                    className="absolute pointer-events-none select-none"
                    style={{
                      left: `${sliderX}px`,
                      top: getSliderTop(),
                      transform: getSliderTransform(),
                      width: `${SLIDER_SIZE}px`,
                      height: `${SLIDER_SIZE}px`,
                      filter: verified
                        ? 'drop-shadow(0 0 4px rgba(129,201,149,0.6))'
                        : isDragging
                          ? 'drop-shadow(0 0 4px rgba(138,180,248,0.6))'
                          : 'none',
                      transition: isDragging ? 'none' : 'filter 0.2s',
                    }}
                    draggable={false}
                  />
                  {verified && (
                    <motion.div initial={{ opacity: 0 }} animate={{ opacity: 1 }} className="absolute inset-0 bg-success/10 flex items-center justify-center">
                      <span className="text-success font-medium text-sm">验证通过</span>
                    </motion.div>
                  )}
                </>
              ) : null}
            </div>

            {/* Slider track */}
            {captcha && !loading && (
              <div ref={trackRef} className="relative h-12 bg-bg-tertiary rounded-full border border-border-light overflow-hidden" style={{ width: `${TRACK_WIDTH}px` }}>
                <div
                  className={`absolute left-0 top-0 h-full rounded-full transition-colors ${verified ? 'bg-success/20' : failed ? 'bg-error/20' : 'bg-accent/10'}`}
                  style={{ width: `${sliderX + SLIDER_SIZE}px` }}
                />
                <div className="absolute inset-0 flex items-center justify-center pointer-events-none">
                  {verified ? (
                    <span className="text-success text-sm font-medium">验证成功</span>
                  ) : (
                    <span className="text-text-muted text-sm select-none">
                      {failed ? '验证失败，请重试' : '请拖动滑块到缺口位置'}
                    </span>
                  )}
                </div>
                <div
                  className={`absolute top-0 left-0 h-full w-12 rounded-full cursor-grab active:cursor-grabbing flex items-center justify-center transition-colors select-none touch-none ${verified ? 'bg-success' : failed ? 'bg-error' : isDragging ? 'bg-accent' : 'bg-accent/80 hover:bg-accent'}`}
                  style={{ transform: `translateX(${sliderX}px)` }}
                  onPointerDown={handlePointerDown}
                  onPointerMove={handlePointerMove}
                  onPointerUp={handlePointerUp}
                >
                  {verified ? (
                    <Check size={18} className="text-white" />
                  ) : (
                    <div className="flex gap-0.5">
                      <div className="w-0.5 h-4 bg-white/60 rounded" />
                      <div className="w-0.5 h-4 bg-white/60 rounded" />
                      <div className="w-0.5 h-4 bg-white/60 rounded" />
                    </div>
                  )}
                </div>
              </div>
            )}
          </motion.div>
        </div>
      )}
    </AnimatePresence>
  );
}
