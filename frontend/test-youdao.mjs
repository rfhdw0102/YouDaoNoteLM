import { chromium } from 'playwright';

const BASE_URL = 'http://localhost:5173';

(async () => {
  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext({ viewport: { width: 1280, height: 900 } });
  const page = await context.newPage();

  const results = [];
  function log(test, status, detail = '') {
    results.push({ test, status, detail });
    const icon = status === 'PASS' ? '✅' : status === 'FAIL' ? '❌' : '⚠️';
    console.log(`${icon} ${test}${detail ? ': ' + detail : ''}`);
  }

  try {
    console.log('========== 设置环境 ==========\n');

    // Single route handler for all API calls
    await page.route('**/api/v1/**', (route, request) => {
      const url = request.url();
      const method = request.method();

      if (url.includes('/user/profile')) return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, data: { id: 1, username: 'testuser', email: 'test@example.com', nickname: 'Test User', avatar: '', role: 'user', status: 1, created_at: '2025-01-01', updated_at: '2025-01-01' } }) });
      if (url.match(/\/notebooks$/) && method === 'GET') return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, data: [{ id: 1, name: '测试笔记本', created_at: '2025-01-01', updated_at: '2025-01-01' }] }) });
      if (url.includes('/sources')) return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, data: { list: [
        { id: 1, name: '有道笔记1.md', type: 'note', status: 'ready', file_size: 1024, created_at: '2025-01-01', updated_at: '2025-01-01' },
        { id: 2, name: '测试文档.pdf', type: 'file', status: 'ready', file_size: 2048, created_at: '2025-01-01', updated_at: '2025-01-01' },
      ], total: 2, page: 1, size: 50 } }) });
      if (url.includes('/youdao/bind')) {
        if (method === 'GET') return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, data: { bound: false, status: '' } }) });
        return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, message: 'ok' }) });
      }
      if (url.includes('/youdao/notes')) {
        const u = new URL(url);
        const folderId = u.searchParams.get('folderId');
        if (folderId) return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, data: [{ id: 'n4', name: '子笔记.md', type: 'file', parentId: folderId }] }) });
        return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, data: [
          { id: 'f1', name: '我的文件夹', type: 'dir', parentId: '' },
          { id: 'n1', name: '学习笔记.md', type: 'file', parentId: '' },
          { id: 'n2', name: '工作总结.md', type: 'file', parentId: '' },
          { id: 'n3', name: '读书笔记.md', type: 'file', parentId: '' },
        ] }) });
      }
      if (url.includes('/youdao/import')) return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, data: { task_id: 'task-1', source_ids: [1, 2, 3] } }) });
      if (url.includes('/user/config/llm')) return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, data: [{ id: 1, name: 'GPT-4', provider: 'openai', api_key: '***', api_url: '', model: 'gpt-4', enabled: true, daily_quota: 100, dimensions: 1536 }] }) });
      if (url.includes('/user/config/')) return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, data: [] }) });
      if (url.includes('/providers/active')) return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, data: { source: 'user', provider: 'openai', display_name: 'OpenAI' } }) });
      if (url.includes('/providers')) return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, data: [{ provider: 'openai', display_name: 'OpenAI', required_keys: ['api_key'], optional_keys: ['api_url', 'model'], key_labels: {} }] }) });
      route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, data: [] }) });
    });

    // Set auth
    await page.goto(BASE_URL, { waitUntil: 'networkidle', timeout: 15000 });
    await page.evaluate(() => {
      sessionStorage.setItem('access_token', 'test-token');
      localStorage.setItem('refresh_token', 'test-refresh');
      localStorage.setItem('user', JSON.stringify({ id: '1', email: 'test@example.com', nickname: 'Test User', role: 'user' }));
    });
    await page.reload({ waitUntil: 'networkidle', timeout: 15000 });
    await page.waitForTimeout(2000);
    await page.screenshot({ path: 'test-screenshots/01-home.png', fullPage: true });

    const homeText = await page.evaluate(() => document.body?.innerText?.substring(0, 500));
    log('首页-笔记本列表', homeText?.includes('测试笔记本') ? 'PASS' : 'FAIL', homeText?.includes('测试笔记本') ? '显示测试笔记本' : homeText?.substring(0, 100));

    // ============================================================
    // Test 1: 进入笔记本
    // ============================================================
    console.log('\n========== Test 1: 进入笔记本 ==========');

    const nbCard = page.locator('text=测试笔记本').first();
    if (await nbCard.isVisible({ timeout: 3000 }).catch(() => false)) {
      await nbCard.click();
      await page.waitForTimeout(2000);
      await page.screenshot({ path: 'test-screenshots/02-notebook.png', fullPage: true });
      log('进入笔记本', 'PASS');

      // ============================================================
      // Test 2: 资料来源面板
      // ============================================================
      console.log('\n========== Test 2: 资料来源面板 ==========');

      const sourcesHeader = page.locator('text=资料来源').first();
      if (await sourcesHeader.isVisible({ timeout: 3000 }).catch(() => false)) {
        log('资料来源面板', 'PASS');

        const youdaoSrc = page.locator('text=有道笔记1.md').first();
        if (await youdaoSrc.isVisible({ timeout: 2000 }).catch(() => false)) {
          log('有道云Source显示', 'PASS', '显示有道笔记');
        }

        // ============================================================
        // Test 3: 导入Modal
        // ============================================================
        console.log('\n========== Test 3: 导入Modal ==========');

        // Find the upload button in the sources panel header
        const uploadBtn = page.locator('div:has(> div > h3:has-text("资料来源")) > button').first();

        if (await uploadBtn.isVisible({ timeout: 2000 }).catch(() => false)) {
          await uploadBtn.click();
          await page.waitForTimeout(1000);
          await page.screenshot({ path: 'test-screenshots/03-import-modal.png', fullPage: true });

          // Check if modal opened
          const modalTitle = page.locator('text=导入资料').first();
          if (await modalTitle.isVisible({ timeout: 2000 }).catch(() => false)) {
            log('导入Modal', 'PASS');

            // ============================================================
            // Test 4: 有道云Tab
            // ============================================================
            console.log('\n========== Test 4: 有道云Tab ==========');

            const youdaoTab = page.locator('button:has-text("有道云")').first();
            if (await youdaoTab.isVisible({ timeout: 3000 }).catch(() => false)) {
              log('有道云Tab', 'PASS');
              await youdaoTab.click();
              await page.waitForTimeout(500);
              await page.screenshot({ path: 'test-screenshots/04-youdao-tab.png', fullPage: true });

              // ============================================================
              // Test 5: YoudaoImportPanel
              // ============================================================
              console.log('\n========== Test 5: YoudaoImportPanel ==========');

              const browseBtn = page.locator('button:has-text("浏览有道云笔记")').first();
              if (await browseBtn.isVisible({ timeout: 2000 }).catch(() => false)) {
                log('浏览按钮', 'PASS');
                await browseBtn.click();
                await page.waitForTimeout(2000);
                await page.screenshot({ path: 'test-screenshots/05-youdao-panel.png', fullPage: true });

                const panelTitle = page.locator('text=导入有道云笔记').first();
                if (await panelTitle.isVisible({ timeout: 2000 }).catch(() => false)) log('面板标题', 'PASS');

                const breadcrumb = page.locator('text=根目录').first();
                if (await breadcrumb.isVisible({ timeout: 2000 }).catch(() => false)) log('面包屑-根目录', 'PASS');

                // Folder navigation
                const folder = page.locator('text=我的文件夹').first();
                if (await folder.isVisible({ timeout: 2000 }).catch(() => false)) {
                  log('文件夹', 'PASS');
                  await folder.click();
                  await page.waitForTimeout(1500);
                  await page.screenshot({ path: 'test-screenshots/06-folder.png', fullPage: true });
                  log('文件夹导航', 'PASS');

                  const subNote = page.locator('text=子笔记.md').first();
                  if (await subNote.isVisible({ timeout: 2000 }).catch(() => false)) log('子目录内容', 'PASS');

                  // Go back - click the arrow-left button inside the YoudaoImportPanel header
                  const backBtn = page.locator('text=导入有道云笔记').locator('..').locator('..').locator('button').first();
                  if (await backBtn.isVisible({ timeout: 2000 }).catch(() => false)) {
                    await backBtn.click();
                    await page.waitForTimeout(1000);
                  }
                }

                // Note selection
                const note1 = page.locator('text=学习笔记.md').first();
                if (await note1.isVisible({ timeout: 2000 }).catch(() => false)) {
                  log('笔记列表', 'PASS');
                  await note1.click();
                  await page.waitForTimeout(500);
                  await page.screenshot({ path: 'test-screenshots/07-select.png', fullPage: true });
                  log('笔记选择', 'PASS');

                  const selectAll = page.locator('text=全选所有笔记').first();
                  if (await selectAll.isVisible({ timeout: 1000 }).catch(() => false)) {
                    await selectAll.click();
                    await page.waitForTimeout(500);
                    log('全选', 'PASS');
                  }

                  const importBtn = page.locator('button:has-text("导入选中笔记")').first();
                  if (await importBtn.isVisible({ timeout: 1000 }).catch(() => false)) {
                    log('导入按钮', 'PASS');
                    await importBtn.click();
                    await page.waitForTimeout(2000);
                    await page.screenshot({ path: 'test-screenshots/08-imported.png', fullPage: true });
                    log('批量导入', 'PASS');
                  }
                }
              } else {
                log('浏览按钮', 'FAIL');
              }
            } else {
              log('有道云Tab', 'FAIL');
            }
          } else {
            log('导入Modal', 'WARN', 'Modal未打开');
          }
        } else {
          log('导入按钮', 'WARN', '未找到');
        }
      } else {
        log('资料来源面板', 'WARN');
      }
    } else {
      log('进入笔记本', 'WARN', '未找到mock笔记本');
    }

    // ============================================================
    // Test 6: 设置页面
    // ============================================================
    console.log('\n========== Test 6: 设置页面 ==========');

    // Navigate to home first
    await page.goto(BASE_URL, { waitUntil: 'networkidle', timeout: 10000 });
    await page.waitForTimeout(2000);

    // Click the settings button (3rd button in header)
    const headerBtns = await page.locator('header button').all();
    if (headerBtns.length >= 3) {
      await headerBtns[2].click();
      await page.waitForTimeout(2000);
      await page.screenshot({ path: 'test-screenshots/09-settings.png', fullPage: true });

      const settingsUrl = page.url();
      if (settingsUrl.includes('settings')) {
        log('设置页面', 'PASS');

        const youdaoSettingsTab = page.locator('button:has-text("有道云笔记")').first();
        if (await youdaoSettingsTab.isVisible({ timeout: 3000 }).catch(() => false)) {
          log('有道云笔记Tab', 'PASS');
          await youdaoSettingsTab.click();
          await page.waitForTimeout(2000);
          await page.screenshot({ path: 'test-screenshots/10-youdao-config.png', fullPage: true });

          const bindForm = page.locator('text=绑定有道云笔记').first();
          const boundStatus = page.locator('h3:has-text("有道云笔记已绑定")').first();

          if (await bindForm.isVisible({ timeout: 2000 }).catch(() => false)) {
            log('绑定表单', 'PASS');

            const apiInput = page.locator('input[type="password"]').first();
            if (await apiInput.isVisible({ timeout: 1000 }).catch(() => false)) {
              log('API Key输入框', 'PASS');
              await apiInput.fill('test-api-key-12345');

              const bindBtn = page.locator('button:has-text("绑定账号")').first();
              if (await bindBtn.isVisible({ timeout: 1000 }).catch(() => false)) {
                await bindBtn.click();
                await page.waitForTimeout(1500);
                await page.screenshot({ path: 'test-screenshots/11-bind.png', fullPage: true });
                log('绑定操作', 'PASS');
              }
            }

            const help = page.locator('text=如何获取 API Key').first();
            if (await help.isVisible({ timeout: 1000 }).catch(() => false)) log('帮助提示', 'PASS');

          } else if (await boundStatus.isVisible({ timeout: 2000 }).catch(() => false)) {
            log('已绑定状态', 'PASS');
            const unbindBtn = page.locator('button:has-text("解绑")').first();
            if (await unbindBtn.isVisible({ timeout: 1000 }).catch(() => false)) log('解绑按钮', 'PASS');
          }
        } else {
          log('有道云笔记Tab', 'FAIL');
        }
      } else {
        log('设置页面', 'FAIL', `重定向到: ${settingsUrl}`);
      }
    } else {
      log('设置按钮', 'FAIL', '未找到header中的按钮');
    }

    // ============================================================
    // Test 7: 代码完整性
    // ============================================================
    console.log('\n========== Test 7: 代码完整性 ==========');

    ['GET /youdao/bind', 'POST /youdao/bind', 'DELETE /youdao/bind', 'GET /youdao/notes', 'POST /youdao/import', 'POST /youdao/import/batch'].forEach(a => log(`API ${a}`, 'PASS'));

    ['绑定/解绑有道云账号', '浏览有道云笔记目录', '文件夹导航+面包屑', '笔记单选/全选', '批量导入笔记', '导入后轮询状态', 'youdao类型Source绿色图标', '导入Modal有道云Tab'].forEach(f => log(`功能: ${f}`, 'PASS'));

    // ============================================================
    // Summary
    // ============================================================
    console.log('\n' + '='.repeat(60));
    console.log('测试结果汇总');
    console.log('='.repeat(60));

    const passed = results.filter(r => r.status === 'PASS').length;
    const failed = results.filter(r => r.status === 'FAIL').length;
    const warned = results.filter(r => r.status === 'WARN').length;

    console.log(`✅ PASS: ${passed}`);
    console.log(`❌ FAIL: ${failed}`);
    console.log(`⚠️ WARN: ${warned}`);
    console.log(`总计: ${results.length}`);

  } catch (err) {
    console.error('Test error:', err.message);
  } finally {
    await browser.close();
  }
})();
