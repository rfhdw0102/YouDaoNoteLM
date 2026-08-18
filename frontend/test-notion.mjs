import { chromium } from 'playwright';

const BASE_URL = 'http://localhost:5173';

function apiResponse(data, message = 'ok') {
  return { status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, message, data }) };
}

async function visible(locator, timeout = 3000) {
  return locator.isVisible({ timeout }).catch(() => false);
}

const browser = await chromium.launch({ headless: true });
const context = await browser.newContext({ viewport: { width: 1280, height: 900 } });
const page = await context.newPage();
const results = [];
let imported = false;
let bindingConnected = true;

function check(name, passed, detail = '') {
  results.push({ name, passed, detail });
  console.log(`${passed ? 'PASS' : 'FAIL'} ${name}${detail ? `: ${detail}` : ''}`);
  if (!passed) throw new Error(`${name}${detail ? `: ${detail}` : ''}`);
}

try {
  await page.route('**/api/v1/**', async (route, request) => {
    const url = new URL(request.url());
    const { pathname } = url;

    if (pathname === '/api/v1/user/profile') {
      return route.fulfill(apiResponse({ id: 1, username: 'fixture-user', email: 'fixture@example.test', nickname: 'Fixture User', avatar: '', role: 'user', status: 1 }));
    }
    if (pathname === '/api/v1/notebooks' && request.method() === 'GET') {
      return route.fulfill(apiResponse([{ id: 1, name: 'Notion 测试笔记本', created_at: '2026-01-01', updated_at: '2026-01-01' }]));
    }
    if (pathname === '/api/v1/notion/bind') {
      return route.fulfill(apiResponse({ bound: bindingConnected, status: bindingConnected ? 'active' : '' }));
    }
    if (pathname === '/api/v1/notion/pages') {
      return route.fulfill(apiResponse({
        list: [
          { id: 'page-fixture-1', title: 'Notion 项目计划', url: 'https://www.notion.so/page-fixture-1', last_edited_time: '2026-01-02T03:04:05.000Z', has_children: false },
          { id: 'page-fixture-2', title: 'Notion 会议记录', url: 'https://www.notion.so/page-fixture-2', last_edited_time: '2026-01-03T03:04:05.000Z', has_children: true },
          { id: 'page-fixture-unsafe', title: '不安全 Notion 页面', url: 'javascript:alert(1)', last_edited_time: '2026-01-04T03:04:05.000Z', has_children: false },
        ],
        has_more: false,
      }));
    }
    if (pathname === '/api/v1/notion/import/batch' && request.method() === 'POST') {
      imported = true;
      return route.fulfill(apiResponse({ task_id: 'notion-task-fixture', source_ids: [101] }));
    }
    if (pathname === '/api/v1/notion/import/tasks/notion-task-fixture') {
      return route.fulfill(apiResponse({ task_id: 'notion-task-fixture', task_type: 'notion_import', notebook_id: 1, total_count: 1, processed_count: 1, success_count: 1, fail_count: 0, status: 'completed', error_detail: '', created_at: 0 }));
    }
    if (pathname === '/api/v1/notebooks/1/sources') {
      const list = imported
        ? [{ id: 101, name: 'Notion 项目计划', type: 'notion', status: 'ready', vectorized: true, original_url: 'javascript:alert(1)', file_size: 0, created_at: '2026-01-02', updated_at: '2026-01-02' }]
        : [];
      return route.fulfill(apiResponse({ list, total: list.length, page: 1, size: 50, total_page: 1 }));
    }
    if (pathname.includes('/content')) return route.fulfill(apiResponse({ content: '# fixture' }));
    if (pathname === '/api/v1/user/config/llm') return route.fulfill(apiResponse([]));
    if (pathname.startsWith('/api/v1/user/config/')) return route.fulfill(apiResponse([]));
    if (pathname === '/api/v1/providers/active') return route.fulfill(apiResponse({ source: 'system', provider: 'fixture', display_name: 'Fixture' }));
    if (pathname === '/api/v1/providers') return route.fulfill(apiResponse([]));
    return route.fulfill(apiResponse([]));
  });

  await page.goto(BASE_URL, { waitUntil: 'networkidle', timeout: 15000 });
  await page.evaluate(() => {
    sessionStorage.setItem('access_token', 'fixture-token');
    localStorage.setItem('refresh_token', 'fixture-refresh');
    localStorage.setItem('user', JSON.stringify({ id: 'fixture-user', email: 'fixture@example.test', nickname: 'Fixture User', role: 'user' }));
  });
  await page.reload({ waitUntil: 'networkidle', timeout: 15000 });

  const notebook = page.getByText('Notion 测试笔记本').first();
  check('open notebook', await visible(notebook));
  await notebook.click();

  const importButton = page.locator('div:has(> div > h3:has-text("资料来源")) > button').first();
  check('open import modal', await visible(importButton));
  await importButton.click();
  check('import modal visible', await visible(page.getByText('导入资料').first()));

  const notionTab = page.getByRole('button', { name: 'Notion', exact: true }).first();
  check('Notion tab visible', await visible(notionTab));
  await notionTab.click();

  check('Notion page selector visible', await visible(page.getByText('导入 Notion 页面').first()));
  const search = page.getByPlaceholder('搜索 Notion 页面');
  check('Notion page search visible', await visible(search));
  await search.fill('项目');
  check('Notion page list visible', await visible(page.getByText('Notion 项目计划').first()));
  check('unsafe Notion page URL is not linked', !(await visible(page.getByRole('link', { name: '在 Notion 打开 不安全 Notion 页面', exact: true }))));
  await page.getByText('Notion 项目计划').first().click();
  check('selected count visible', await visible(page.getByText('已选 1 个页面').first()));

  const submit = page.getByRole('button', { name: '导入选中页面', exact: true });
  check('Notion import submit visible', await visible(submit));
  await submit.click();
  await page.waitForTimeout(300);
  check('Notion source appears', await visible(page.getByText('Notion 项目计划').first()));
  check('Notion source label appears', await visible(page.getByText('Notion').last()));
  await page.getByText('Notion 项目计划').first().click();
  check('unsafe Notion source URL is not linked', !(await visible(page.getByRole('link', { name: '查看 Notion 原页面', exact: true }))));

  bindingConnected = false;
  await page.goto(`${BASE_URL}/settings?tab=notion`, { waitUntil: 'networkidle', timeout: 15000 });
  check('settings shows disconnected state', await visible(page.getByText('连接 Notion').first()));
  check('settings shows authorization button', await visible(page.getByRole('button', { name: '登录并授权 Notion', exact: true })));

  console.log(`PASS ${results.length} assertions`);
} finally {
  await browser.close();
}
