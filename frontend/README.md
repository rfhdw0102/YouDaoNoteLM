# YoudaoNoteLM Web

智能知识管理平台前端，基于资料导入、AI 对话、内容生成三大核心能力，为学生、研究者和知识工作者提供一站式知识管理体验。

## 技术栈

| 类别 | 技术 | 版本 |
|------|------|------|
| 框架 | React + TypeScript | 19 / 6 |
| 构建 | Vite | 8 |
| 样式 | Tailwind CSS | 4 |
| 状态管理 | Zustand | 5 |
| 路由 | React Router | 7 |
| HTTP | Axios | 1.17 |
| 动画 | Framer Motion | 12 |
| 图标 | Lucide React | — |
| Markdown | React Markdown + remark-gfm | — |

## 快速开始

```bash
# 安装依赖
npm install

# 启动开发服务器（默认 http://localhost:5173）
npm run dev

# 构建生产版本
npm run build

# 预览构建结果
npm run preview
```

开发服务器会将 `/api` 和 `/uploads` 请求代理到 `http://localhost:8080`，确保后端服务已启动。

## 项目结构

```
src/
├── api/                    # API 接口层
│   ├── client.ts           # Axios 实例（双 Token 自动刷新）
│   ├── auth.ts             # 认证接口
│   ├── notebook.ts         # 笔记本接口
│   └── user.ts             # 用户接口
├── components/
│   ├── notebook/           # 笔记本业务组件
│   │   ├── ChatPanel.tsx   # AI 对话面板
│   │   ├── SourcesPanel.tsx# 资料来源管理
│   │   ├── NotesPanel.tsx  # 笔记管理
│   │   ├── MindmapViewer.tsx# 思维导图查看器
│   │   ├── PPTViewer.tsx   # PPT 预览
│   │   └── QuizCard.tsx    # 测验卡片
│   └── ui/                 # 通用 UI 组件
├── layouts/                # 布局组件
├── pages/                  # 页面组件
├── routes/                 # 路由配置（含鉴权守卫）
├── stores/                 # Zustand 状态管理
├── types/                  # TypeScript 类型定义
└── utils/                  # 工具函数
```

## 核心功能

### 用户认证
- 邮箱注册 / 登录 / 找回密码
- 滑块验证码人机验证
- 双 Token 机制（access_token 15 分钟 + refresh_token 1 天）无感刷新

### 笔记本管理
- 创建、重命名、删除笔记本
- 创建后自动跳转详情页

### 资料导入
- 文件上传（PDF、Word、TXT 等）
- URL 网页抓取
- 音频导入
- 网络搜索

### AI 对话
- 基于已导入资料的智能问答
- 流式输出
- 对话历史管理
- 对话内容保存为笔记

### 内容生成
- 思维导图：自动提炼知识结构，支持缩放和展开/折叠
- PPT：自动生成演示文稿，支持全屏演示和 HTML 导出
- 测验：基于资料生成题目，逐题作答、即时反馈
- 笔记：AI 辅助整理笔记

### 个人中心
- 头像上传
- 用户名 / 密码修改
- 账号注销

### 系统设置
- AI 服务配置
- 搜索 API 配置
- 深色 / 浅色主题切换

## 路由结构

```
/                         → 首页（笔记本列表）      [需登录]
/notebook/:id             → 笔记本详情              [需登录]
/settings                 → 设置                    [需登录]
/profile                  → 个人中心                [需登录]
/login                    → 登录                    [仅游客]
/register                 → 注册                    [仅游客]
/forgot-password          → 找回密码                [仅游客]
```

## 环境要求

- Node.js >= 18
- 后端服务运行在 `http://localhost:8080`

## License

Private
