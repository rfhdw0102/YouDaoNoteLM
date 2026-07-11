import { useState } from 'react';
import { Download, FileText, Loader2 } from 'lucide-react';
import Button from '../components/ui/Button';
import { downloadBlob, exportGenerationFile } from '../api/generation';
import { getErrorMessage } from '../utils/error';

const sampleHTML = `<!doctype html>
<html>
<head>
  <meta charset="utf-8" />
  <style>
    body { margin: 0; font-family: Aptos, "Microsoft YaHei", sans-serif; background: #f8fafc; color: #172033; }
    section { width: 1280px; height: 720px; box-sizing: border-box; padding: 64px 78px; background: linear-gradient(135deg, #fff7ed 0%, #f8fafc 56%, #e0f2fe 100%); }
    h1 { margin: 0 0 18px; font-size: 58px; line-height: 1.08; color: #0f172a; letter-spacing: -1px; }
    h2 { margin: 0 0 22px; font-size: 40px; color: #0f766e; }
    p { max-width: 820px; font-size: 24px; line-height: 1.55; color: #334155; }
    .eyebrow { color: #ea580c; font-size: 18px; font-weight: 700; letter-spacing: 2px; text-transform: uppercase; }
    .grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 22px; margin-top: 34px; }
    .card { padding: 28px; border-radius: 26px; background: rgba(255,255,255,.78); border: 1px solid rgba(15,23,42,.12); }
    .card strong { display: block; margin-bottom: 10px; font-size: 24px; color: #0f172a; }
    .card span { font-size: 18px; line-height: 1.55; color: #475569; }
    mark { background: #fed7aa; color: #9a3412; padding: 0 6px; border-radius: 8px; }
  </style>
</head>
<body>
  <section>
    <div class="eyebrow">PPT Export Test</div>
    <h1>HTML 驱动的 PPTX 导出</h1>
    <p>这页用于验证导出是否尽量还原 HTML 的字体、颜色、布局和内联样式，而不是套用难看的默认蓝色背景。</p>
  </section>
  <section>
    <h2>样式验证点</h2>
    <p>文本应保持可编辑，并保留 <strong>加粗重点</strong>、<span style="color:#dc2626;font-weight:700;">红色强调</span> 和 <mark>高亮片段</mark>。</p>
    <div class="grid">
      <div class="card"><strong>版式</strong><span>卡片、间距、边框和背景应尽量跟随 HTML。</span></div>
      <div class="card"><strong>字号</strong><span>标题和正文不应被压成统一大小。</span></div>
      <div class="card"><strong>颜色</strong><span>避免强制替换成旧的蓝色或紫色默认风格。</span></div>
    </div>
  </section>
</body>
</html>`;

export default function PPTExportTestPage() {
  const [title, setTitle] = useState('PPT导出测试');
  const [html, setHtml] = useState(sampleHTML);
  const [exporting, setExporting] = useState(false);
  const [message, setMessage] = useState('');

  const handleExportPPTX = async () => {
    setExporting(true);
    setMessage('');
    try {
      const file = await exportGenerationFile({ type: 'ppt', title, content: html });
      downloadBlob(file.blob, file.filename);
      setMessage(`已生成 ${file.filename}`);
    } catch (error) {
      setMessage(getErrorMessage(error, 'PPTX 导出失败'));
    } finally {
      setExporting(false);
    }
  };

  const handleExportHTML = () => {
    downloadBlob(new Blob([html], { type: 'text/html;charset=utf-8' }), `${title || 'ppt-export-test'}.html`);
  };

  return (
    <div className="h-full overflow-y-auto bg-bg-primary">
      <div className="mx-auto max-w-[1500px] px-8 py-6">
        <div className="mb-5 flex items-center justify-between gap-4">
          <div>
            <p className="text-xs font-medium uppercase tracking-[0.18em] text-accent">Export Lab</p>
            <h1 className="mt-1 text-2xl font-semibold text-text-primary">PPT 导出测试页</h1>
            <p className="mt-2 text-sm text-text-muted">编辑 HTML 后点击导出，验证后端 HTML 转 PPTX 的还原效果。</p>
          </div>
          <div className="flex items-center gap-2">
            <Button variant="secondary" onClick={handleExportHTML}>
              <FileText size={15} /> 导出 HTML
            </Button>
            <Button onClick={handleExportPPTX} loading={exporting} disabled={!html.trim()}>
              {exporting ? <Loader2 size={15} className="animate-spin" /> : <Download size={15} />}
              {exporting ? '生成中...' : '导出 PPTX'}
            </Button>
          </div>
        </div>

        {message && (
          <div className="mb-4 rounded-xl border border-border-light bg-bg-card px-4 py-3 text-sm text-text-secondary">
            {message}
          </div>
        )}

        <div className="mb-4 rounded-2xl border border-border-light bg-bg-card p-4">
          <label className="mb-2 block text-xs font-medium text-text-muted">导出标题</label>
          <input
            value={title}
            onChange={(event) => setTitle(event.target.value)}
            className="h-10 w-full rounded-lg border border-border bg-bg-secondary px-3 text-sm text-text-primary outline-none focus:border-accent"
          />
        </div>

        <div className="grid grid-cols-2 gap-5">
          <section className="min-h-[680px] rounded-2xl border border-border-light bg-bg-card p-4">
            <div className="mb-3 flex items-center justify-between">
              <h2 className="text-sm font-semibold text-text-primary">HTML 输入</h2>
              <button className="text-xs text-accent hover:underline" onClick={() => setHtml(sampleHTML)}>
                恢复样例
              </button>
            </div>
            <textarea
              value={html}
              onChange={(event) => setHtml(event.target.value)}
              spellCheck={false}
              className="h-[620px] w-full resize-none rounded-xl border border-border bg-bg-secondary p-4 font-mono text-xs leading-relaxed text-text-primary outline-none focus:border-accent"
            />
          </section>

          <section className="min-h-[680px] rounded-2xl border border-border-light bg-bg-card p-4">
            <h2 className="mb-3 text-sm font-semibold text-text-primary">在线预览</h2>
            <div className="overflow-hidden rounded-xl border border-border bg-bg-subtle">
              <iframe
                srcDoc={html}
                title="PPT export test preview"
                sandbox="allow-same-origin"
                className="block h-[620px] w-full border-0"
              />
            </div>
          </section>
        </div>
      </div>
    </div>
  );
}
