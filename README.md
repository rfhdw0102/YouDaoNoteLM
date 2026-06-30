# YouDaoNoteLM 部署指南

本项目是一个基于 RAG 的有道云笔记知识问答系统，采用前后端一体的 Docker 部署方案。
本文档面向**部署人员**，目标是在一台干净的 Linux 服务器上把项目跑起来并通过健康检查。

---

## 一、架构与服务拓扑

整个系统由一组 Docker 容器组成，全部通过 `docker compose` 编排：

| 服务 | 容器名 | 作用 | 宿主机端口 |
|------|--------|------|-----------|
| **app** | youdaonotelm-app | Nginx(8080) + Go 后端(8081) + 前端静态资源 + 有道 CLI | `18080` |
| **mysql** | youdaonotelm-mysql | 业务数据库 | 不暴露 |
| **redis** | youdaonotelm-redis | 缓存 / 会话 | 不暴露 |
| **minio** | youdaonotelm-minio | 对象存储（笔记附件、音频等） | `19000` / `19001` |
| **etcd** | youdaonotelm-etcd | Milvus 依赖 | 不暴露 |
| **milvus-minio** | youdaonotelm-milvus-minio | Milvus 内部存储 | 不暴露 |
| **milvus** | youdaonotelm-milvus | 向量数据库 | 不暴露 |
| **markitdown** | youdaonotelm-markitdown | 文档转 Markdown 服务 | `8085` |

> 容器内部：Nginx 监听 `8080`，反向代理 `/api/` 到 Go 后端的 `8081`，前端静态文件由 Nginx 直接提供。
> 宿主机访问入口统一是 `http://<服务器IP>:18080`。

### 部署目录结构（服务器上）

```
/home/flandern/youdaonotelm/        # 部署根目录（可改）
├── docker-compose.yml
├── .env                            # 端口、密码等环境变量
└── configs/
    ├── docker_config.yaml          # 应用配置（挂载为容器内 /app/configs/config.yaml）
    └── youdao_cookies.json         # 有道云笔记登录 Cookie
```

---

## 二、前置准备

### 1. 服务器要求
- **系统**：Linux（推荐 Ubuntu 20.04+ / CentOS 7+）
- **配置**：≥ 4 核 CPU、≥ 8 GB 内存、≥ 40 GB 磁盘（Milvus 较吃资源）
- **已安装**：Docker ≥ 20.10、docker compose v2（`docker compose version` 能看到版本号）
- **权限**：以 `root` 或具备 docker 权限的用户登录

### 2. 端口放行（安全组 / 防火墙）

| 端口 | 用途 | 是否必须对公网开放 |
|------|------|------------------|
| 18080 | 应用访问 | ✅ 必须 |
| 19000 | MinIO API（文件上传/下载） | ✅ 必须（前端直连） |
| 19001 | MinIO 控制台 | ⚠️ 建议仅限管理 IP，或不开 |
| 8085 | MarkItDown | ❌ 仅容器内网使用，可不开放 |
| 22 | SSH | ✅ |

### 3. 必备配置文件
从仓库获取以下两个文件并放到服务器的 `configs/` 目录：
- `configs/docker_config.yaml`（应用主配置）
- `configs/youdao_cookies.json`（有道云笔记 Cookie，见 [附录 A](#附录-a-youdao_cookiesjson-说明)）

---

## 三、部署方式（二选一）

### 方式 A：本地一键远程部署（推荐）

在**本地开发机**执行 `deploy.sh`，通过 SSH 把配置推送到服务器并拉起服务：

```bash
# 1. 修改 deploy.sh 顶部的三个变量，改成你自己的服务器
SERVER="你的服务器IP"
SERVER_USER="root"
DEPLOY_DIR="/home/flandern/youdaonotelm"

# 2. 确保本地能免密 SSH 登录服务器
ssh root@你的服务器IP   # 能直接进去即可

# 3. 执行
bash deploy.sh
```

脚本会自动完成：创建目录 → 生成 `docker-compose.yml` 与 `.env` → 上传配置 → 修正 MinIO 公网端点 → 拉取镜像 → `docker compose up -d` → 健康检查。

### 方式 B：在服务器上直接部署

把 `server-setup.sh` 传到服务器后直接运行（脚本内已内嵌 `docker-compose.yml` 与 `.env`）：

```bash
# 1. 上传脚本和配置
scp server-setup.sh root@<服务器IP>:/root/
scp -r configs/ root@<服务器IP>:/root/configs/

# 2. 登录服务器执行
ssh root@<服务器IP>
bash /root/server-setup.sh
```

> ⚠️ 无论哪种方式，**部署前必须修改脚本里的硬编码 IP/密码**（见下节）。

---

## 四、部署前必改项

以下值在 `deploy.sh` / `server-setup.sh` / `.env` 中以**示例值**存在，正式部署请替换为你自己的：

### 1. 服务器 IP
脚本中所有 `60.205.184.232` 需替换为你的服务器公网 IP。最关键的一处是 **MinIO 公网端点**——前端通过它访问上传的文件：

```bash
# deploy.sh 里的这行会把 docker_config.yaml 中的 public_endpoint 改成服务器IP:19000
ssh ... "sed -i 's|public_endpoint:.*|public_endpoint: \"你的IP:19000\"|' .../docker_config.yaml"
```

如果你用方式 B，记得手工把 `configs/docker_config.yaml` 里的 `public_endpoint` 改成 `你的IP:19000`。

### 2. 密码与密钥（务必更换）
`.env` 中的默认密码仅作示例，**生产环境必须改强**：

| 变量 | 默认示例值 | 说明 |
|------|-----------|------|
| `MYSQL_ROOT_PASSWORD` | `20041211wzwaicjW.` | MySQL root 密码 |
| `REDIS_PASSWORD` | `redis123` | Redis 密码 |
| `MINIO_ROOT_USER` / `MINIO_ROOT_PASSWORD` | `minioadmin` / `minio123` | MinIO 管理员账号 |

> 改了 `.env` 里的密码后，**必须同步修改 `configs/docker_config.yaml` 中对应的密码字段**（`database.mysql.password`、`database.redis.password`、`external.minio.access_key/secret_key`），因为应用读的是 yaml 配置，不是 `.env`。

### 3. encryption_key（API Key 加密密钥）
`docker_config.yaml` 中：
```yaml
security:
  encryption_key: "YouDaoNoteLM32ByteEncryptKey!!!!"
```
- 用于 AES-256-GCM 加密用户存入数据库的 LLM API Key。
- **硬性约束：必须恰好 32 字节**，否则应用启动时校验失败。
- 生产环境请换成 32 字节高熵随机串。
- ⚠️ **首次部署后不要随便改它**：一旦更换，历史加密的 API Key 将无法解密。换 key 需配套数据迁移。

### 4. JWT secret
`jwt.secret` 建议改成一段随机字符串，用于签发登录 Token。

---

## 五、启动与验证

### 1. 启动
```bash
cd /home/flandern/youdaonotelm
docker compose up -d
```

### 2. 查看状态
```bash
docker compose ps
# 期望所有服务均为 Up / (healthy)
```

### 3. 健康检查
```bash
curl -i http://localhost:18080/api/v1/health
# 期望返回 HTTP 200
```

### 4. 访问入口
- 应用首页：`http://<服务器IP>:18080`
- MinIO 控制台：`http://<服务器IP>:19001`（账号见 `.env`）
- MinIO API：`http://<服务器IP>:19000`

---

## 六、常见问题排查

### 1. 健康检查返回非 200 / 应用起不来
```bash
cd /home/flandern/youdaonotelm
docker compose logs --tail=100 app     # 看后端日志
docker compose logs --tail=50 mysql   # 看依赖是否就绪
```
常见原因：
- `encryption_key` 不是 32 字节 → 启动报错 `security.encryption_key 必须为32字节`。
- `docker_config.yaml` 里的 MySQL/Redis 密码与 `.env` 不一致 → 连接失败。
- Milvus 未就绪就启动 app → 重启 app 即可：`docker compose restart app`。

### 2. 前端能打开，但上传/播放文件 404
检查 `docker_config.yaml` 的 `external.minio.public_endpoint` 是否为 `<服务器IP>:19000`，以及安全组是否放行了 `19000`。

### 3. 端口冲突
默认选了 `18080 / 19000 / 19001 / 8085` 避开常见端口。如仍冲突，改 `.env` 里的 `APP_PORT / MINIO_API_PORT / MINIO_CONSOLE_PORT`，并同步改 `public_endpoint`。

### 4. 完全重来
```bash
docker compose down -v        # 注意 -v 会删除所有数据卷（数据丢失！）
docker compose up -d
```

---

## 七、日常运维命令

```bash
# 启停
docker compose start
docker compose stop

# 重启单个服务
docker compose restart app

# 查看实时日志
docker compose logs -f app

# 升级镜像
docker compose pull && docker compose up -d

# 查看数据卷占用
docker system df
```

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
