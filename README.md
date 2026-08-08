# 📚 YouDaoNoteLM

> 基于 RAG 的有道云笔记知识问答系统，前后端一体，纯 Docker 部署。

![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white)
![React](https://img.shields.io/badge/React-19-61DAFB?logo=react&logoColor=white)
![Docker](https://img.shields.io/badge/Docker-Compose-2496ED?logo=docker&logoColor=white)
![Milvus](https://img.shields.io/badge/Milvus-向量数据库-00A6FB?logo=milvus&logoColor=white)
![License](https://img.shields.io/badge/License-MIT-green)

YouDaoNoteLM 是一个 NotebookLM 风格的全栈知识问答应用：将你的有道云笔记（或本地文件、网页、音频）导入为知识源，通过向量检索 + 大模型生成（RAG）实现精准问答、异步内容生成（思维导图 / PPT / 测验 / 笔记）和流式对话。用户可维护跨会话输出偏好，并对已保存回答提交分类点赞/点踩反馈；前端 React + Vite，后端 Go + Gin，向量库 Milvus，对象存储 MinIO，全部通过 Docker Compose 一键拉起。

---
## 📋 一、项目总体介绍

### ✨ 核心能力

| 能力 | 说明 |
|------|------|
| 📥 **多源知识导入** | 有道云笔记批量同步、本地文件（PDF / Word / PPT / TXT，经 MarkItDown 转 Markdown）、网页 URL、音频（阿里云 ASR 转写） |
| 🔍 **RAG 检索与协同对话** | 基于 Milvus 向量库 + Eino 编排，支持父子分块、语义分块、重排序、SSE 流式输出，以及主 Agent 协调搜索与生成子 Agent |
| 🎨 **异步内容生成与导出** | 思维导图、PPT（支持导出 HTML / docx）、测验、AI 笔记；生成任务支持排队、进度状态、取消、历史查看与结果回填 |
| 🧠 **跨会话输出偏好** | 用户可在设置中维护默认语言、回答篇幅、回答方式、输出格式、生成风格和通用偏好；当前请求始终优先于已保存偏好 |
| 👍 **回答质量反馈** | 对已保存的 Chat 助手回答点赞/点踩并选择固定原因；可修改或撤回，刷新会话后保留当前状态 |
| 🧩 **多模型即插即用** | LLM / Embedding / ASR / 搜索四类服务均采用可插拔 Provider 注册表，用户级配置、API Key 加密存储 |
| 🔐 **账号体系** | 邮箱注册 + 验证码、双 Token（Access / Refresh）+ Redis 黑名单、滑动验证码、登录失败锁定、bcrypt 密码哈希 |
| ⚙️ **管理后台** | 用户管理、系统级配置（搜索 / ASR / Embedding）动态管理，以及脱敏反馈报表、筛选明细和 CSV 导出 |

### 🧠 偏好与反馈闭环

- **长期记忆**：只保存用户明确维护的输出偏好，不保存用户事实、完整对话或资料内容。偏好会用于后续 Chat 和内容生成，用户可随时编辑或删除。
- **回答反馈**：反馈绑定一条已持久化的 Chat 助手消息，采用固定原因分类；每位用户对同一回答只保留一份当前评价，可改选或撤回。
- **隐私边界**：反馈记录、管理员列表和 CSV 不复制用户问题、回答正文、引用或会话标题。管理员仅可查看提交时间、评价和原因的脱敏明细与聚合数据。

### 🛠️ 技术栈

| 层 | 技术 |
|----|------|
| 前端 | React 19 + TypeScript + Vite 8 + Tailwind CSS 4 + Zustand + React Router 7 |
| 后端 | Go 1.25 + Gin + GORM + Viper + Zap |
| 数据库 | MySQL 8.0（业务）+ Redis 7（缓存 / 会话）+ Milvus（向量） |
| 对象存储 | MinIO |
| AI 编排 | CloudWeGo Eino + eino-ext（Milvus indexer / retriever、Ark / OpenAI embedding、OpenAI / Anthropic LLM） |
| 文档转换 | MarkItDown（FastAPI 微服务，docx / pptx / pdf → Markdown） |
| 音频转写 | 阿里云 NLS ASR + ffmpeg |
| 容器化 | Docker Compose（8 个服务）+ Nginx 反向代理 |

### 🏗️ 服务架构

| 服务 | 容器名 | 作用 | 端口（容器内） |
|------|--------|------|---------------|
| **app** | youdaonotelm-app | Nginx(8080) → Go 后端(8081) + 前端静态资源 | 8080 |
| **mysql** | youdaonotelm-mysql | 业务数据库 | 3306（不暴露） |
| **redis** | youdaonotelm-redis | 缓存 / 会话 / Token 黑名单 | 6379（不暴露） |
| **minio** | youdaonotelm-minio | 对象存储（附件、音频、头像） | 9000 / 9001 |
| **etcd** | youdaonotelm-etcd | Milvus 依赖 | 2379（不暴露） |
| **milvus-minio** | youdaonotelm-milvus-minio | Milvus 内部存储 | 9000（不暴露） |
| **milvus** | youdaonotelm-milvus | 向量数据库 | 19530（不暴露） |
| **markitdown** | youdaonotelm-markitdown | 文档转 Markdown | 8085 |

> 容器内部：Nginx 监听 `8080`，反向代理 `/api/` 到 Go 后端的 `8081`，前端静态文件由 Nginx 直接提供。

---

## 🚀 二、部署方式

项目提供两种部署方式：**Docker 一键部署**（推荐，生产可用）和**源码本地部署**（适合二次开发）。

### 🐳 方式一：Docker 部署（推荐）

服务器上只需 `docker-compose.yml` + 配置文件，镜像直接从 Docker Hub 拉取，**无需克隆源码仓库**。

#### 前置准备

- **服务器**：Linux，≥ 4 核 CPU、≥ 8 GB 内存、≥ 40 GB 磁盘
- **软件**：Docker ≥ 20.10、docker compose v2
- **端口放行**（安全组 / 防火墙）：

| 端口 | 用途 | 公网 |
|------|------|------|
| 8080 | 应用访问 | 必须 |
| 9000 | MinIO API（文件上传 / 下载） | 必须 |
| 9001 | MinIO 控制台 | 可选 |
| 22 | SSH | 必须 |

> 若端口冲突，可在 `.env` 中修改 `APP_PORT`、`MINIO_API_PORT`、`MINIO_CONSOLE_PORT`。

#### 部署步骤

```bash
# 1. 创建目录并下载编排文件
mkdir -p youdaonotelm/configs && cd youdaonotelm
curl -fsSL 'https://raw.githubusercontent.com/rfhdw0102/YouDaoNoteLM/develop/docker-compose.yml' -o docker-compose.yml
curl -fsSL 'https://raw.githubusercontent.com/rfhdw0102/YouDaoNoteLM/develop/configs/docker_config.yaml.example' -o configs/docker_config.yaml

# 2. 创建 .env（参照下方"配置详解"填写密码、密钥、服务器IP等）
vim .env

# 3. configs/docker_config.yaml 通常无需修改
#    敏感字段（密码、密钥、MinIO 公网端点等）已留空，由 .env 中同名变量自动覆盖
vim configs/docker_config.yaml

# 4. 从 Docker Hub 拉取镜像并启动
docker compose pull
docker compose up -d

# 5. 验证
curl -i http://localhost:8080/api/v1/health
# 期望返回 HTTP 200
```

#### 访问入口

- **应用首页**：`http://<服务器IP>:8080`
- **MinIO 控制台**：`http://<服务器IP>:9001`（账号见 `.env`）

#### 部署目录结构

```
youdaonotelm/                     # 部署根目录
├── docker-compose.yml            # 服务编排（从 GitHub 下载，无需修改）
├── .env                          # 环境变量配置（密码、端口等，需自行创建）
└── configs/
    ├── docker_config.yaml        # 应用配置（从 docker_config.yaml.example 复制后修改）
```

### 💻 方式二：源码本地部署（开发）

适合二次开发或自定义构建。需要克隆完整源码并在本地构建镜像。

#### 前置准备

- **Go** ≥ 1.25
- **Node.js** ≥ 20（用于构建前端）
- **Python** ≥ 3.11（MarkItDown 服务，可选）
- **Docker** + docker compose v2（运行依赖服务：MySQL / Redis / MinIO / Milvus）
- 系统需安装 `ffmpeg`（音频处理）
- 本地直接运行 Go 后端还需安装 `youdaonote` CLI；Docker 镜像会在构建时自动安装

#### 有道云笔记 CLI

有道云笔记的目录浏览、搜索、读取、创建和更新操作通过 `youdaonote` CLI 完成。Docker 构建会自动安装 CLI；本地直接运行 `go run ./cmd/server` 时，需要先自行安装并确保它在 `PATH` 中：

```bash
curl -fsSL https://artifact.lx.netease.com/download/youdaonote-cli/install.sh \
  | bash -s -- -f -b "$HOME/.local/bin"

youdaonote --source ydn check --json
```

如果 CLI 不在 `PATH` 中，请在 `configs/config.yaml` 的 `external.youdao.cli_path` 中填写绝对路径。默认值 `youdaonote` 表示从 `PATH` 查找。CLI 不可用时应用仍可启动，但有道云笔记相关功能无法使用。

#### YoudaoNoteLM API CLI 包装

项目提供了一个 Python CLI 包装用于调用 HTTP API。先启动本地服务：

```bash
docker compose up -d
```

然后安装并查看命令：

```bash
cd agent-harness
python -m pip install -e .
python -m cli_anything.youdaonotelm --help
```

CLI 默认请求 `http://localhost:8080`。其他 API 地址通过放在子命令之前的 `--base-url` 指定：

```bash
python -m cli_anything.youdaonotelm --base-url http://localhost:8081 health
python -m cli_anything.youdaonotelm --base-url http://localhost:8081 login <email> <password>
```

登录时出现 `WinError 10061` 通常表示后端尚未启动或地址/端口错误。访问令牌保存在用户目录下的 `.config/youdaonotelm/token.json`。

#### 部署步骤

```bash
# 1. 克隆源码
git clone https://github.com/rfhdw0102/YouDaoNoteLM.git
cd YouDaoNoteLM

# 2. 准备配置
cp .env.example .env
# 编辑 .env：填写所有必填项（密码、密钥、ENCRYPTION_KEY 必须 32 字节、MINIO_PUBLIC_ENDPOINT 等）
cp configs/config.yaml.example configs/config.yaml
# 本地开发用 configs/config.yaml；docker 部署用 configs/docker_config.yaml

# 3. （本地直接运行 Go 后端，跳过镜像构建）启动依赖服务
docker compose up -d mysql redis minio etcd milvus-minio milvus markitdown

# 4. 构建前端
cd frontend
npm install
npm run build           # 产物在 frontend/dist
cd ..

# 5. 启动 Go 后端
go run ./cmd/server     # 或 go build -o bin/server ./cmd/server && ./bin/server

# 6. 验证
curl -i http://localhost:8081/api/v1/health
```

> 前端开发热更新：`cd frontend && npm run dev`，通过 Vite dev server 访问（需配置代理转发 `/api/` 到后端 8081）。

#### 完整本地镜像构建（含前端 + 后端 + Nginx）

若希望以容器方式跑完整 stack 但使用本地源码构建镜像：

```bash
# 注释 .env 中 DOCKER_IMAGE 和 MARKITDOWN_IMAGE 两行
# 然后构建并启动所有服务
docker compose build
docker compose up -d
```

> 注意：Dockerfile 中引用了 `docker-entrypoint.sh`，若该文件缺失需自行补全或调整 Dockerfile。

---

## ⚙️ 三、配置详解

### 🔑 `.env` — 环境变量

| 变量 | 说明 | 是否必填 |
|------|------|---------|
| `DOCKER_IMAGE` | 应用镜像，默认从 Docker Hub 拉取 `flandern/youdaonote:latest`；本地构建时注释此行 | 可选 |
| `MARKITDOWN_IMAGE` | MarkItDown 镜像，默认从 Docker Hub 拉取 `flandern/markitdown:latest`；本地构建时注释此行 | 可选 |
| `APP_PORT` | 宿主机端口，默认 `8080` | 可选 |
| `MYSQL_ROOT_PASSWORD` | MySQL root 密码 | **必填** |
| `MYSQL_DATABASE` | MySQL 数据库名，默认 `youdao` | 可选 |
| `REDIS_PASSWORD` | Redis 密码 | **必填** |
| `MINIO_ROOT_USER` / `MINIO_ROOT_PASSWORD` | MinIO 管理员账号 | **必填** |
| `MINIO_API_PORT` / `MINIO_CONSOLE_PORT` | MinIO API / 控制台端口 | 可选 |
| `MYSQL_PASSWORD` | 应用连接 MySQL 的密码（通常与 `MYSQL_ROOT_PASSWORD` 一致） | **必填** |
| `JWT_SECRET` | JWT 签名密钥 | **必填** |
| `EMAIL_PASSWORD` | 邮箱 SMTP 密码（默认 QQ 邮箱 `smtp.qq.com:587`） | **必填** |
| `ENCRYPTION_KEY` | API Key 加密密钥，必须恰好 **32 字节** | **必填** |
| `MINIO_ACCESS_KEY` / `MINIO_SECRET_KEY` | 应用连接 MinIO 的密钥（通常与 `MINIO_ROOT_USER/PASSWORD` 一致） | **必填** |
| `MINIO_ENDPOINT` | MinIO SDK 连接地址，Docker 部署设为 `minio:9000`，本地开发设为 `localhost:9000` | 可选 |
| `MINIO_PUBLIC_ENDPOINT` | MinIO 公网地址，用于预签名 URL 的 host 重写（SDK 不连接此地址），供浏览器 / 阿里云 ASR 访问，格式 `服务器IP:端口` | **必填** |
| `BOCHA_API_KEY` | 博查搜索 API Key，留空禁用联网搜索 | 可选 |

### 📄 `configs/docker_config.yaml` — 应用配置

- 大部分字段保持默认值即可，**通常无需手动修改**
- 敏感字段（密码、密钥、MinIO 公网端点等）已留空，由 `.env` 中同名变量自动覆盖
- 如需调整非敏感字段（日志级别、连接池大小、CORS 等）再编辑此文件

> **配置优先级**：`docker_config.yaml` 中的空字段会被 `.env` 中同名环境变量覆盖（通过 `pkg/config/loader.go` 的 `os.Getenv` 逻辑）。非空字段以 yaml 为准。

### ⚠️ 重要提示（部署前必读）

**修改默认密码 / 密钥**：`.env.example` 中的值仅作示例，生产环境必须全部更换。

**`encryption_key` 硬性约束**：

- 必须恰好 **32 字节**，否则应用启动失败
- **首次部署后不要随意更改**，否则历史加密的 API Key 将无法解密（启动时会校验历史密文，密钥不匹配将直接 fail-fast）

### 📊 三层配置模型

项目采用三层配置设计：

1. **静态 secrets**：`.env`（密码、密钥、端口等敏感信息）
2. **非敏感默认值**：`configs/docker_config.yaml`（日志、连接池、CORS 等）
3. **动态运行配置**：MySQL `sys_config` / `user_config` / `user_llm_config` 表，通过管理后台 / 用户配置 API 管理，API Key 经 AES 加密存储

---

## 👤 四、管理员设置

系统**不会自动将首个注册用户设为管理员**，所有新注册用户默认角色为 `user`。管理员需通过数据库手动提升。

### 步骤

1. 用户先通过前端注册页面完成正常注册并登录一次（确保用户记录已创建）。

2. 进入 MySQL 容器执行 SQL：

```bash
docker exec -it youdaonotelm-mysql mysql -uroot -p
# 输入 MYSQL_ROOT_PASSWORD
```

3. 将对应用户的 `role` 字段更新为 `admin`：

```sql
USE youdao;

-- 查看用户列表
SELECT id, email, username, role, status FROM users;

-- 提升为管理员
UPDATE users SET role = 'admin' WHERE email = 'your_email@example.com';
```

4. 用户**重新登录**后即可访问管理后台功能（管理后台入口位于前端用户菜单内）。

### 管理员能力

- **用户管理**：查看用户列表、启用 / 禁用用户（`PUT /admin/users/:id/status`）
- **系统配置**：动态管理 `search` / `asr` / `embedding` 三类系统级配置（`/admin/config/*`）
- **反馈工作台**：按最多 31 天的时间窗口和固定枚举筛选，查看反馈总量、点赞/点踩数量、正向占比、原因分布与脱敏分页明细；可导出最多 10,000 行的 CSV

> 注意：LLM 配置为用户级，每个用户独立配置自己的 LLM Provider 和 API Key；系统级配置（搜索 / ASR / Embedding）由管理员统一管理。

---

## 🔧 五、日常运维

```bash
# 启停所有服务
docker compose start / stop / restart

# 重启单个服务
docker compose restart app

# 查看实时日志
docker compose logs -f app

# 升级镜像
docker compose pull && docker compose up -d

# 完全重来（⚠️ -v 会删除所有数据卷）
docker compose down -v && docker compose up -d

# 查看数据卷占用
docker system df
```

---

## ❓ 六、常见问题

<details>
<summary><b>🏥 健康检查返回非 200</b></summary>

```bash
docker compose logs --tail=100 app    # 查看后端日志
docker compose logs --tail=50 mysql   # 查看数据库是否就绪
```

常见原因：
- `encryption_key` 不是 32 字节
- MySQL / Redis 密码与 `.env` 不一致
- Milvus 未就绪 → 重启 app：`docker compose restart app`

</details>

<details>
<summary><b>📄 前端能打开，但上传 / 播放文件 404</b></summary>

检查 `.env` 中 `MINIO_PUBLIC_ENDPOINT` 是否正确，以及安全组是否放行了 MinIO 端口。

</details>

<details>
<summary><b>🔌 端口冲突</b></summary>

修改 `.env` 中的 `APP_PORT`、`MINIO_API_PORT`、`MINIO_CONSOLE_PORT`，并同步修改 `MINIO_PUBLIC_ENDPOINT`。

</details>

<details>
<summary><b>📒 有道云笔记导入失败</b></summary>

先确认 `youdaonote` CLI 已安装且可执行：

```bash
youdaonote --source ydn check --json
```

绑定和同步使用用户 API Key；CLI 返回的内容直接进入后续导入流程。

</details>

---
## 🗺️ 七、后续计划

<details>
<summary><b>点击展开 / 收起后续计划</b></summary>

- **HTTPS 与域名**：集成 HTTPS 证书与自定义域名访问，提升生产环境安全性。
- **Provider 注册引导**：对四类可配置服务（LLM / Embedding / ASR / 搜索）补充常见提供商的注册申请指引，降低新用户配置门槛。
- **多笔记平台接入**：不限于有道云笔记，计划接入印象笔记、Notion、飞书文档、语雀等主流笔记 / 文档端，实现统一的知识库管理。

</details>

---

