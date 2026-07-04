# YouDaoNoteLM

基于 RAG 的有道云笔记知识问答系统，前后端一体，纯 Docker 部署。

---

## 一、快速开始

### 前置准备

- **服务器**：Linux，≥ 4 核 CPU、≥ 8 GB 内存、≥ 40 GB 磁盘
- **软件**：Docker ≥ 20.10、docker compose v2
- **端口放行**（安全组/防火墙）：

| 端口 | 用途 | 公网 |
|------|------|------|
| 8080 | 应用访问 | ✅ 必须 |
| 9000 | MinIO API（文件上传/下载） | ✅ 必须 |
| 9001 | MinIO 控制台 | ⚠️ 可选 |
| 22 | SSH | ✅ |

> 若端口冲突，可在 `.env` 中修改 `APP_PORT`、`MINIO_API_PORT`、`MINIO_CONSOLE_PORT`。

### 部署步骤

服务器上只需 `docker-compose.yml` + 配置文件，镜像直接从 Docker Hub 拉取，**无需克隆源码仓库**。

```bash
# 1. 创建目录并下载编排文件
mkdir -p youdaonotelm/configs && cd youdaonotelm
curl -fsSL 'https://raw.githubusercontent.com/rfhdw0102/YouDaoNoteLM/develop/docker-compose.yml' -o docker-compose.yml
curl -fsSL 'https://raw.githubusercontent.com/rfhdw0102/YouDaoNoteLM/develop/configs/docker_config.yaml.example' -o configs/docker_config.yaml

# 2. 创建 .env（参照下方说明填写密码、密钥、服务器IP等）
vim .env

# 3. configs/docker_config.yaml 通常无需修改
#    敏感字段（密码、密钥、MinIO 公网端点等）已留空，由 .env 中同名变量自动覆盖
#    如需调整非敏感字段（日志级别、连接池大小、CORS 等）再编辑此文件
vim configs/docker_config.yaml

# 4. 从 Docker Hub 拉取镜像并启动
docker compose pull
docker compose up -d

# 5. 验证
curl -i http://localhost:8080/api/v1/health
# 期望返回 HTTP 200
```

> 💡 如需本地二次开发，注释 `.env` 中 `DOCKER_IMAGE` / `MARKITDOWN_IMAGE` 两行，然后 `docker compose build` 构建本地镜像。

### 访问入口

- **应用首页**：`http://<服务器IP>:8080`
- **MinIO 控制台**：`http://<服务器IP>:9001`（账号见 `.env`）

---

## 二、架构与服务

| 服务 | 容器名 | 作用 | 端口（容器内） |
|------|--------|------|---------------|
| **app** | youdaonotelm-app | Nginx(8080) → Go 后端(8081) + 前端静态资源 | 8080 |
| **mysql** | youdaonotelm-mysql | 业务数据库 | 3306（不暴露） |
| **redis** | youdaonotelm-redis | 缓存 / 会话 | 6379（不暴露） |
| **minio** | youdaonotelm-minio | 对象存储（附件、音频等） | 9000 / 9001 |
| **etcd** | youdaonotelm-etcd | Milvus 依赖 | 2379（不暴露） |
| **milvus-minio** | youdaonotelm-milvus-minio | Milvus 内部存储 | 9000（不暴露） |
| **milvus** | youdaonotelm-milvus | 向量数据库 | 19530（不暴露） |
| **markitdown** | youdaonotelm-markitdown | 文档转 Markdown | 8085 |

> 容器内部：Nginx 监听 `8080`，反向代理 `/api/` 到 Go 后端的 `8081`，前端静态文件由 Nginx 直接提供。

### 部署目录结构（服务器上）

```
youdaonotelm/                     # 部署根目录
├── docker-compose.yml            # 服务编排（从 GitHub 下载，无需修改）
├── .env                          # 环境变量配置（密码、端口等，需自行创建）
└── configs/
    ├── docker_config.yaml        # 应用配置（从 docker_config.yaml.example 复制后修改）
    └── youdao_cookies.json       # 有道云笔记 Cookie（可选，需自行创建）
```

---

## 三、配置详解

### `.env` — 环境变量

| 变量 | 说明 | 是否必填 |
|------|------|---------|
| `DOCKER_IMAGE` | 应用镜像，默认从 Docker Hub 拉取 `flandern/youdaonote:latest`；本地构建时注释此行即可 | 可选 |
| `MARKITDOWN_IMAGE` | MarkItDown 镜像，默认从 Docker Hub 拉取 `flandern/markitdown:latest`；本地构建时注释此行即可 | 可选 |
| `APP_PORT` | 宿主机端口，默认 `8080` | 可选 |
| `MYSQL_ROOT_PASSWORD` | MySQL root 密码 | **必填** |
| `REDIS_PASSWORD` | Redis 密码 | **必填** |
| `MINIO_ROOT_USER` / `MINIO_ROOT_PASSWORD` | MinIO 管理员账号 | **必填** |
| `MYSQL_PASSWORD` | 应用连接 MySQL 的密码（通常与 `MYSQL_ROOT_PASSWORD` 一致） | **必填** |
| `JWT_SECRET` | JWT 签名密钥 | **必填** |
| `EMAIL_PASSWORD` | 邮箱 SMTP 密码 | **必填** |
| `ENCRYPTION_KEY` | API Key 加密密钥，必须恰好 **32 字节** | **必填** |
| `MINIO_ACCESS_KEY` / `MINIO_SECRET_KEY` | 应用连接 MinIO 的密钥（通常与 `MINIO_ROOT_USER/PASSWORD` 一致） | **必填** |
| `MINIO_PUBLIC_ENDPOINT` | MinIO 公网地址，供阿里云 ASR 下载音频文件，格式 `服务器IP:端口` | **必填** |
| `BOCHA_API_KEY` | 博查搜索 API Key，留空禁用联网搜索 | 可选 |

### `configs/docker_config.yaml` — 应用配置

- 大部分字段保持默认值即可，**通常无需手动修改**
- 敏感字段（密码、密钥、MinIO 公网端点等）已留空，由 `.env` 中同名变量自动覆盖：
  - `security.encryption_key` ← `ENCRYPTION_KEY`
  - `external.minio.access_key` / `secret_key` ← `MINIO_ACCESS_KEY` / `MINIO_SECRET_KEY`
  - MinIO 公网端点 ← `MINIO_PUBLIC_ENDPOINT`（yaml 中无此字段，仅由环境变量注入，见 `pkg/config/loader.go`）
- 如需调整非敏感字段（日志级别、连接池大小、CORS 等）再编辑此文件

> **配置优先级**：`docker_config.yaml` 中的空字段会被 `.env` 中同名环境变量覆盖（通过 `pkg/config/loader.go` 的 `os.Getenv` 逻辑）。非空字段以 yaml 为准。

---

## 四、重要提示（部署前必读）

### ⚠️ 修改默认密码/密钥

`.env` 中的默认值仅作示例，**生产环境必须全部更换**：

| 变量 | 默认值 |
|------|--------|
| `MYSQL_ROOT_PASSWORD` / `MYSQL_PASSWORD` | `20041211wzwaicjW.` |
| `REDIS_PASSWORD` | `redis123` |
| `MINIO_ROOT_PASSWORD` / `MINIO_SECRET_KEY` | `minio123` |
| `JWT_SECRET` | `YouDaoNoteBookLM-API-Web-...` |
| `EMAIL_PASSWORD` | `koptculidhqpdcjb` |
| `ENCRYPTION_KEY` | `YouDaoNoteLM-AES-Key-32Bytes!!` |

### ⚠️ `encryption_key` 硬性约束

- 必须恰好 **32 字节**，否则应用启动失败
- **首次部署后不要随意更改**，否则历史加密的 API Key 将无法解密

### ⚠️ 敏感字段只在 `.env` 中设置

以下字段在 `configs/docker_config.yaml` 中**留空**，由 `.env` 中同名环境变量覆盖（见 `pkg/config/loader.go`）。**只需在 `.env` 中填写一次**，无需在 yaml 中重复填写：

| 配置项 | `.env` 变量 | yaml 字段（留空） |
|--------|-------------|------------------|
| MySQL 密码 | `MYSQL_PASSWORD` | `database.mysql.password` |
| Redis 密码 | `REDIS_PASSWORD` | `database.redis.password` |
| JWT 密钥 | `JWT_SECRET` | `jwt.secret` |
| 邮箱密码 | `EMAIL_PASSWORD` | `email.password` |
| 加密密钥 | `ENCRYPTION_KEY` | `security.encryption_key` |
| MinIO 密钥 | `MINIO_ACCESS_KEY` / `MINIO_SECRET_KEY` | `external.minio.access_key` / `secret_key` |
| MinIO 公网端点 | `MINIO_PUBLIC_ENDPOINT` | yaml 中无此字段，仅由环境变量注入 |
| 博查 API Key | `BOCHA_API_KEY` | `external.bocha.api_key` |

---

## 五、日常运维

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

## 六、常见问题

### 健康检查返回非 200

```bash
docker compose logs --tail=100 app    # 查看后端日志
docker compose logs --tail=50 mysql   # 查看数据库是否就绪
```

常见原因：
- `encryption_key` 不是 32 字节
- MySQL/Redis 密码与 `.env` 不一致
- Milvus 未就绪 → 重启 app：`docker compose restart app`

### 前端能打开，但上传/播放文件 404

检查 `.env` 中 `MINIO_PUBLIC_ENDPOINT` 是否正确，以及安全组是否放行了 MinIO 端口。

### 端口冲突

修改 `.env` 中的 `APP_PORT`、`MINIO_API_PORT`、`MINIO_CONSOLE_PORT`，并同步修改 `MINIO_PUBLIC_ENDPOINT`。

---

## 附录 A：youdao_cookies.json 说明

用于有道云笔记 CLI 登录，格式如下（值需从浏览器登录有道云笔记后抓取 Cookie 填入）：

```json
{
    "cookies": [
        ["YNOTE_CSTK", "你的YNOTE_CSTK值", ".note.youdao.com", "/"],
        ["YNOTE_LOGIN", "你的YNOTE_LOGIN值", ".note.youdao.com", "/"],
        ["YNOTE_SESS", "你的YNOTE_SESS值", ".note.youdao.com", "/"]
    ]
}
```

> Cookie 会过期，过期后导入笔记会失败，需重新抓取并替换该文件，然后 `docker compose restart app`。

## 附录 B：相关文档

更详细的操作步骤与截图说明见 `.github/` 目录：
- `DEPLOYMENT_GUIDE.md` — 完整部署流程
- `QUICK_START.md` / `QUICK_REFERENCE.md` — 速查
- `SSH_KEY_GUIDE.md` — SSH 免密配置
- `CONFIG_STEPS.md` — 配置项详解
