import fs from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { chromium } from 'playwright';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

async function main() {
  const args = process.argv.slice(2);
  if (args[0] === '--self-test') {
    await runSelfTest();
    return;
  }

  const [inputPath, outputPath, deckTitle = 'ppt-export'] = args;
  if (!inputPath || !outputPath) {
    throw new Error('Usage: node export_dom_to_pptx.mjs <input.html> <output.pptx> [deckTitle]');
  }

  const html = await fs.readFile(inputPath, 'utf8');
  const data = await exportHTMLToPPTX(html, deckTitle);
  await fs.writeFile(outputPath, data);
}

async function runSelfTest() {
  const html = `<!doctype html>
<html>
  <head>
    <style>
      body { margin: 0; font-family: Arial, sans-serif; background: #f8fafc; }
      section { background: #fff7ed; color: #111827; padding: 96px; }
      h1 { font-size: 72px; margin: 0 0 32px; }
      p { font-size: 36px; color: #334155; }
    </style>
  </head>
  <body>
    <section><h1>DOM Export Test</h1><p>Editable PPTX content</p></section>
  </body>
</html>`;
  const data = await exportHTMLToPPTX(html, 'self-test');
  if (data.length < 2 || data[0] !== 0x50 || data[1] !== 0x4b) {
    throw new Error('Self-test did not produce PPTX zip bytes');
  }
  console.log(`Generated PPTX bytes: ${data.length}`);
}

async function exportHTMLToPPTX(html, deckTitle) {
  const browser = await chromium.launch({ headless: true });
  try {
    const page = await browser.newPage({
      viewport: { width: 1920, height: 1080 },
      deviceScaleFactor: 1,
    });

    await page.setContent(wrapHTMLDocument(html), { waitUntil: 'networkidle' });
    await page.addScriptTag({ path: path.join(__dirname, 'node_modules', 'dom-to-pptx', 'dist', 'dom-to-pptx.bundle.js') });
    await page.evaluate(prepareSlidesForExport);

    const base64 = await page.evaluate(async ({ title }) => {
      const slides = Array.from(document.querySelectorAll('section[data-ppt-slide="true"]'));
      if (slides.length === 0) {
        throw new Error('PPT export content must contain at least one <section> slide');
      }

      const blob = await window.domToPptx.exportToPptx(slides, {
        fileName: `${title || 'ppt-export'}.pptx`,
        skipDownload: true,
        layout: 'LAYOUT_16x9',
        svgAsVector: true,
      });
      const buffer = await blob.arrayBuffer();
      const bytes = new Uint8Array(buffer);
      let binary = '';
      const chunkSize = 0x8000;
      for (let i = 0; i < bytes.length; i += chunkSize) {
        binary += String.fromCharCode(...bytes.subarray(i, i + chunkSize));
      }
      return btoa(binary);
    }, { title: deckTitle });

    return Buffer.from(base64, 'base64');
  } finally {
    await browser.close();
  }
}

function wrapHTMLDocument(html) {
  const trimmed = String(html || '').trim();
  if (/<!doctype html|<html[\s>]/i.test(trimmed)) {
    return injectBaseStyles(trimmed);
  }
  return injectBaseStyles(`<!doctype html>
<html>
  <head><meta charset="utf-8"></head>
  <body>${trimmed}</body>
</html>`);
}

function injectBaseStyles(html) {
  const style = `<style id="youdaonotelm-dom-pptx-base">
html, body {
  margin: 0;
  padding: 0;
  width: 1920px;
  min-height: 1080px;
  background: #ffffff;
}
body {
  display: block;
}
section[data-ppt-slide="true"] {
  position: relative;
  display: block;
  width: 1920px !important;
  height: 1080px !important;
  min-width: 1920px !important;
  min-height: 1080px !important;
  max-width: 1920px !important;
  max-height: 1080px !important;
  box-sizing: border-box;
  overflow: hidden;
}
</style>`;

  if (/<\/head>/i.test(html)) {
    return html.replace(/<\/head>/i, `${style}</head>`);
  }
  return html.replace(/<html[^>]*>/i, (match) => `${match}<head><meta charset="utf-8">${style}</head>`);
}

function prepareSlidesForExport() {
  const sections = Array.from(document.querySelectorAll('section'));
  for (const section of sections) {
    section.dataset.pptSlide = 'true';
    section.classList.add('youdaonotelm-ppt-slide');
  }
}

main().catch((error) => {
  console.error(error?.stack || error?.message || String(error));
  process.exit(1);
});

