# GitHub Actions CI/CD 部署指南

## 📋 概述

本项目使用 GitHub Actions 实现持续集成和持续部署（CI/CD）。

### 工作流说明

1. **CI（持续集成）** - `.github/workflows/ci.yml`
   - 触发条件：PR 合并到 `develop` 分支
   - 功能：代码检查、单元测试、构建验证

2. **CD（持续部署）** - `.github/workflows/cd.yml`
   - 触发条件：代码合并到 `develop` 分支
   - 功能：自动构建并部署到 CentOS 7 服务器

### 服务器环境

- **操作系统**: CentOS 7
- **服务器 IP**: 60.205.184.232
- **进程管理**: Supervisor
- **进程名称**: youdaonotelm-server
- **部署路径**: /home/flandern/youdaonotelm

---

## 🔧 配置步骤

### 第一步：GitHub 仓库设置

1. 进入 GitHub 仓库页面
2. 点击 `Settings` → `Environments`
3. 创建两个环境：`staging` 和 `production`

### 第二步：配置 GitHub Secrets

进入 `Settings` → `Secrets and variables` → `Actions`，添加以下 Secrets：

#### Staging 环境 Secrets

| Secret 名称 | 说明 | 示例值 |
|-------------|------|--------|
| `SSH_HOST` | 服务器 IP 地址 | `60.205.184.232` |
| `SSH_USERNAME` | SSH 用户名 | `root` |
| `SSH_PRIVATE_KEY` | SSH 私钥内容 | 见下方说明 |

#### Production 环境 Secrets（可选）

| Secret 名称 | 说明 | 示例值 |
|-------------|------|--------|
| `PRODUCTION_SSH_HOST` | 生产服务器 IP | `60.205.184.232` |
| `PRODUCTION_SSH_USERNAME` | SSH 用户名 | `root` |
| `PRODUCTION_SSH_PRIVATE_KEY` | SSH 私钥 | 见下方说明 |

### 第三步：配置 GitHub Variables

进入 `Settings` → `Secrets and variables` → `Actions` → `Variables`，添加：

| Variable 名称 | 说明 | 示例值 |
|--------------|------|--------|
| `STAGING_URL` | Staging 环境 URL | `http://60.205.184.232:8080` |
| `PRODUCTION_URL` | Production 环境 URL | `http://60.205.184.232:8080` |

---

## 🔑 生成 SSH 密钥

### 在本地生成密钥对

```bash
# 生成 SSH 密钥对
ssh-keygen -t ed25519 -C "github-actions" -f ~/.ssh/github_actions

# 查看公钥（需要添加到服务器）
cat ~/.ssh/github_actions.pub

# 查看私钥（需要添加到 GitHub Secrets）
cat ~/.ssh/github_actions
```

### 将公钥添加到服务器

```bash
# 在服务器上执行
mkdir -p ~/.ssh
chmod 700 ~/.ssh

# 将公钥添加到 authorized_keys
echo "你的公钥内容" >> ~/.ssh/authorized_keys
chmod 600 ~/.ssh/authorized_keys
```

---

## 🖥️ 服务器配置

### 1. 创建部署目录

```bash
mkdir -p /home/flandern/youdaonotelm
mkdir -p /home/flandern/youdaonotelm/configs
mkdir -p /home/flandern/youdaonotelm/logs
```

### 2. 配置 Supervisor

创建 Supervisor 配置文件：

```bash
sudo vi /etc/supervisor/conf.d/youdaonotelm-server.conf
```

配置内容：

```ini
[program:youdaonotelm-server]
command=/home/flandern/youdaonotelm/youdaonotelm-server
directory=/home/flandern/youdaonotelm
user=root
autostart=true
autorestart=true
startsecs=10
startretries=3
redirect_stderr=true
stdout_logfile=/home/flandern/youdaonotelm/logs/server.log
stdout_logfile_maxbytes=50MB
stdout_logfile_backups=10
environment=GIN_MODE="release",GO_ENV="production"
```

### 3. 创建日志目录

```bash
mkdir -p /home/flandern/youdaonotelm/logs
```

### 4. 重启 Supervisor

```bash
sudo systemctl restart supervisord
sudo systemctl enable supervisord
```

### 5. 上传初始配置文件

```bash
# 在本地执行
scp configs/config.yaml root@60.205.184.232:/home/flandern/youdaonotelm/configs/
```

---

## 🚀 使用流程

### 自动部署（推荐）

1. 在本地开发并推送到 `develop` 分支
2. 创建 PR 合并到 `develop` 分支
3. GitHub Actions 自动运行 CI 测试
4. 测试通过后自动触发 CD 部署
5. 部署完成后自动进行健康检查

```bash
# 本地开发流程
git checkout develop
git pull origin develop
# 进行开发...
git add .
git commit -m "feat: 新功能"
git push origin develop
# 创建 PR 并合并
```

### 手动部署

1. 进入 GitHub 仓库页面
2. 点击 `Actions` → `CD`
3. 点击 `Run workflow`
4. 选择环境（staging/production）
5. 点击 `Run workflow` 按钮

---

## 📊 监控和日志

### 查看 Supervisor 状态

```bash
# 查看进程状态
sudo supervisorctl status youdaonotelm-server

# 查看实时日志
sudo supervisorctl tail -f youdaonotelm-server

# 查看错误日志
sudo supervisorctl tail youdaonotelm-server stderr
```

### 查看应用日志

```bash
# 查看应用日志
tail -f /home/flandern/youdaonotelm/logs/server.log

# 查看最近 100 行日志
tail -n 100 /home/flandern/youdaonotelm/logs/server.log
```

### Supervisor 常用命令

```bash
# 启动服务
sudo supervisorctl start youdaonotelm-server

# 停止服务
sudo supervisorctl stop youdaonotelm-server

# 重启服务
sudo supervisorctl restart youdaonotelm-server

# 查看所有进程状态
sudo supervisorctl status

# 重新加载配置
sudo supervisorctl reread
sudo supervisorctl update
```

---

## 🔄 回滚操作

### 自动回滚（部署失败时）

如果部署失败，可以手动触发回滚：

```bash
# 在服务器上执行
cd /home/flandern/youdaonotelm

# 查看备份文件
ls -la youdaonotelm-server.backup.*

# 恢复到最近的备份
cp youdaonotelm-server.backup.20240101_120000 youdaonotelm-server

# 重启服务
sudo supervisorctl restart youdaonotelm-server
```

### 手动回滚到指定版本

```bash
# 停止服务
sudo supervisorctl stop youdaonotelm-server

# 恢复指定版本
cp youdaonotelm-server.backup.20240101_120000 youdaonotelm-server

# 恢复配置（如果需要）
cp configs/config.yaml.backup configs/config.yaml

# 启动服务
sudo supervisorctl start youdaonotelm-server

# 验证服务
sudo supervisorctl status youdaonotelm-server
```

---

## ❓ 常见问题

### 1. SSH 连接失败

**问题**: `Permission denied (publickey,gssapi-keyex,gssapi-with-mic)`

**解决**:
```bash
# 检查 SSH 配置
sudo vi /etc/ssh/sshd_config

# 确保以下配置
PubkeyAuthentication yes
AuthorizedKeysFile .ssh/authorized_keys

# 重启 SSH 服务
sudo systemctl restart sshd
```

### 2. Supervisor 进程启动失败

**问题**: `FATAL: Exited too quickly`

**解决**:
```bash
# 查看详细错误日志
sudo supervisorctl tail youdaonotelm-server stderr

# 检查二进制文件权限
ls -la /home/flandern/youdaonotelm/youdaonotelm-server

# 手动测试运行
cd /home/flandern/youdaonotelm
./youdaonotelm-server
```

### 3. 端口被占用

**问题**: `bind: address already in use`

**解决**:
```bash
# 查看端口占用
netstat -tuln | grep 8080
lsof -i :8080

# 停止占用端口的进程
kill -9 <PID>
```

### 4. 配置文件丢失

**问题**: 部署后配置文件被覆盖

**解决**:
```bash
# 从备份恢复
cp /home/flandern/youdaonotelm/configs/config.yaml.backup /home/flandern/youdaonotelm/configs/config.yaml

# 或者手动上传配置文件
scp configs/config.yaml root@60.205.184.232:/home/flandern/youdaonotelm/configs/
```

---

## 📝 最佳实践

1. **密钥安全**: 不要将 SSH 私钥提交到代码仓库，使用 GitHub Secrets
2. **环境隔离**: 使用不同的环境（staging/production）进行隔离
3. **备份策略**: 每次部署前自动备份，保留最近 5 个版本
4. **健康检查**: 部署后自动进行健康检查，确保服务正常
5. **日志管理**: 使用 Supervisor 管理日志，定期轮转
6. **监控告警**: 配置监控和告警，及时发现和处理问题

---

## 🔗 相关链接

- [GitHub Actions 官方文档](https://docs.github.com/en/actions)
- [Supervisor 官方文档](http://supervisord.org/)
- [CentOS 7 文档](https://wiki.centos.org/Documentation)
