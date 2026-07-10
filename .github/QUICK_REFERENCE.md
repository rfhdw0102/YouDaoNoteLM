# GitHub Actions CI/CD 快速参考卡

## 🚀 快速开始

### 1. 配置 GitHub Secrets（必需）

在 GitHub 仓库的 `Settings` → `Secrets and variables` → `Actions` 中添加：

| Secret | 值 | 说明 |
|--------|-----|------|
| `SSH_HOST` | `60.205.184.232` | 服务器 IP |
| `SSH_USERNAME` | `root` | SSH 用户名 |
| `SSH_PRIVATE_KEY` | `-----BEGIN OPENSSH PRIVATE KEY-----...` | SSH 私钥内容 |

### 2. 配置 GitHub Variables（可选）

| Variable | 值 | 说明 |
|----------|-----|------|
| `STAGING_URL` | `http://60.205.184.232:8080` | Staging 环境 URL |
| `PRODUCTION_URL` | `http://60.205.184.232:8080` | Production 环境 URL |

### 3. 服务器初始化

```bash
# 上传初始化脚本到服务器
scp .github/scripts/server-setup.sh root@60.205.184.232:/home/flandern/

# 在服务器上执行
ssh root@60.205.184.232
cd /home/flandern
bash server-setup.sh
```

### 4. 上传配置文件

```bash
# 在本地执行
scp configs/config.yaml root@60.205.184.232:/home/flandern/youdaonotelm/configs/
```

---

## 📦 工作流说明

### CI（持续集成）- ci.yml

**触发条件**：PR 合并到 `develop` 分支

**功能**：
- ✅ 代码规范检查（golangci-lint）
- ✅ 单元测试
- ✅ 构建验证

### CD（持续部署）- cd.yml

**触发条件**：代码合并到 `develop` 分支

**功能**：
- ✅ 自动构建 Linux 二进制文件
- ✅ 通过 SSH 部署到服务器
- ✅ 使用 Supervisor 管理进程
- ✅ 自动健康检查

---

## 🔧 常用命令

### GitHub Actions

```bash
# 查看工作流状态
# 在 GitHub 仓库页面 → Actions 选项卡

# 手动触发部署
# 在 GitHub 仓库页面 → Actions → CD → Run workflow
```

### 服务器管理

```bash
# 查看 Supervisor 进程状态
sudo supervisorctl status youdaonotelm-server

# 启动服务
sudo supervisorctl start youdaonotelm-server

# 停止服务
sudo supervisorctl stop youdaonotelm-server

# 重启服务
sudo supervisorctl restart youdaonotelm-server

# 查看实时日志
sudo supervisorctl tail -f youdaonotelm-server

# 查看错误日志
sudo supervisorctl tail youdaonotelm-server stderr

# 查看应用日志
tail -f /home/flandern/youdaonotelm/logs/server.log

# 查看 Supervisor 配置文件
cat /etc/supervisor/conf.d/youdaonotelm-server.conf
```

### 手动部署

```bash
# 本地构建
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o youdaonotelm-server ./cmd/server

# 上传到服务器
scp youdaonotelm-server root@60.205.184.232:/home/flandern/youdaonotelm/

# 重启服务
ssh root@60.205.184.232 "sudo supervisorctl restart youdaonotelm-server"
```

### 回滚操作

```bash
# 在服务器上执行
cd /home/flandern/youdaonotelm

# 查看备份
ls -la youdaonotelm-server.backup.*

# 恢复备份
sudo supervisorctl stop youdaonotelm-server
cp youdaonotelm-server.backup.20240101_120000 youdaonotelm-server
sudo supervisorctl start youdaonotelm-server
```

---

## 📁 服务器目录结构

```
/home/flandern/youdaonotelm/
├── youdaonotelm-server          # 应用程序（二进制文件）
├── configs/
│   ├── config.yaml              # 配置文件
│   └── config.yaml.backup       # 配置备份
├── logs/
│   └── server.log               # 应用日志
└── youdaonotelm-server.backup.* # 版本备份
```

---

## 🔍 故障排查

### 1. 部署失败

```bash
# 查看 GitHub Actions 日志
# 在 GitHub 仓库页面 → Actions → 点击失败的工作流

# 查看服务器日志
sudo supervisorctl tail youdaonotelm-server stderr
tail -n 100 /home/flandern/youdaonotelm/logs/server.log
```

### 2. 服务无法启动

```bash
# 检查进程状态
sudo supervisorctl status youdaonotelm-server

# 手动测试运行
cd /home/flandern/youdaonotelm
./youdaonotelm-server

# 检查端口占用
netstat -tuln | grep 8080
lsof -i :8080
```

### 3. SSH 连接失败

```bash
# 测试 SSH 连接
ssh -v root@60.205.184.232

# 检查 SSH 配置
sudo vi /etc/ssh/sshd_config

# 确保以下配置
PubkeyAuthentication yes
AuthorizedKeysFile .ssh/authorized_keys

# 重启 SSH 服务
sudo systemctl restart sshd
```

### 4. 配置文件问题

```bash
# 检查配置文件是否存在
ls -la /home/flandern/youdaonotelm/configs/

# 从备份恢复
cp /home/flandern/youdaonotelm/configs/config.yaml.backup \
   /home/flandern/youdaonotelm/configs/config.yaml

# 手动上传配置文件
scp configs/config.yaml root@60.205.184.232:/home/flandern/youdaonotelm/configs/
```

---

## 📊 监控

### 健康检查

```bash
# 检查服务是否响应
curl http://60.205.184.232:8080/health

# 检查端口是否监听
netstat -tuln | grep 8080
```

### 日志监控

```bash
# 实时监控日志
tail -f /home/flandern/youdaonotelm/logs/server.log

# 搜索错误日志
grep -i "error\|fatal\|panic" /home/flandern/youdaonotelm/logs/server.log

# 查看最近的日志
tail -n 100 /home/flandern/youdaonotelm/logs/server.log
```

---

## 🔐 安全建议

1. **使用 SSH 密钥认证**：避免使用密码认证
2. **限制 SSH 访问**：只允许特定 IP 访问
3. **定期更新密码**：定期更换服务器密码
4. **监控异常登录**：检查 `/var/log/secure` 日志
5. **使用非 root 用户**：创建专用用户运行应用

---

## 📚 相关文档

- [详细部署指南](DEPLOYMENT_GUIDE.md)
- [Supervisor 配置示例](supervisor/youdaonotelm.ini)
- [服务器初始化脚本](scripts/server-setup.sh)
- [GitHub Actions 官方文档](https://docs.github.com/en/actions)
- [Supervisor 官方文档](http://supervisord.org/)
