# GitHub Actions CI/CD 配置说明

## 📋 项目概述

本项目使用 GitHub Actions 实现自动化 CI/CD 流程，支持将 Go 应用自动部署到 CentOS 7 服务器。

### 服务器信息

- **操作系统**: CentOS 7
- **服务器 IP**: 60.205.184.232
- **进程管理**: Supervisor
- **进程名称**: youdaonotelm-server
- **部署路径**: /home/flandern/youdaonotelm

---

## 📁 文件结构

```
.github/
├── workflows/
│   ├── ci.yml                    # CI 工作流配置
│   └── cd.yml                    # CD 工作流配置
├── supervisor/
│   └── youdaonotelm.ini         # Supervisor 配置示例
├── scripts/
│   └── server-setup.sh          # 服务器初始化脚本
├── DEPLOYMENT_GUIDE.md          # 详细部署指南
├── SSH_KEY_GUIDE.md             # SSH 密钥配置指南
├── QUICK_REFERENCE.md           # 快速参考卡
└── README.md                    # 本文件
```

---

## 🚀 快速开始

### 1. 配置 GitHub Secrets

在 GitHub 仓库的 `Settings` → `Secrets and variables` → `Actions` 中添加：

| Secret | 值 | 说明 |
|--------|-----|------|
| `SSH_HOST` | `60.205.184.232` | 服务器 IP |
| `SSH_USERNAME` | `root` | SSH 用户名 |
| `SSH_PRIVATE_KEY` | SSH 私钥内容 | 见 SSH_KEY_GUIDE.md |

### 2. 初始化服务器

```bash
# 上传初始化脚本
scp .github/scripts/server-setup.sh root@60.205.184.232:/home/flandern/

# 在服务器上执行
ssh root@60.205.184.232 "cd /home/flandern && bash server-setup.sh"
```

### 3. 上传配置文件

```bash
scp configs/config.yaml root@60.205.184.232:/home/flandern/youdaonotelm/configs/
```

### 4. 开始使用

1. 在本地开发并推送到 `develop` 分支
2. 创建 PR 并合并到 `develop` 分支
3. GitHub Actions 自动运行 CI 测试
4. 测试通过后自动触发 CD 部署
5. 部署完成后自动进行健康检查

---

## 📦 工作流说明

### CI（持续集成）

**文件**: `.github/workflows/ci.yml`

**触发条件**: PR 合并到 `develop` 分支

**功能**:
- ✅ 代码规范检查（golangci-lint）
- ✅ 单元测试
- ✅ 构建验证

### CD（持续部署）

**文件**: `.github/workflows/cd.yml`

**触发条件**: 代码合并到 `develop` 分支

**功能**:
- ✅ 自动构建 Linux 二进制文件
- ✅ 通过 SSH 部署到服务器
- ✅ 使用 Supervisor 管理进程
- ✅ 自动健康检查
- ✅ 自动备份和回滚支持

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
# 查看进程状态
sudo supervisorctl status youdaonotelm-server

# 启动服务
sudo supervisorctl start youdaonotelm-server

# 停止服务
sudo supervisorctl stop youdaonotelm-server

# 重启服务
sudo supervisorctl restart youdaonotelm-server

# 查看日志
tail -f /home/flandern/youdaonotelm/logs/server.log
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

---

## 📚 文档说明

### 详细文档

- **[DEPLOYMENT_GUIDE.md](DEPLOYMENT_GUIDE.md)**: 完整的部署指南，包含详细步骤和故障排查
- **[SSH_KEY_GUIDE.md](SSH_KEY_GUIDE.md)**: SSH 密钥生成和配置指南
- **[QUICK_REFERENCE.md](QUICK_REFERENCE.md)**: 快速参考卡，常用命令速查

### 配置文件

- **[supervisor/youdaonotelm.ini](supervisor/youdaonotelm.ini)**: Supervisor 配置示例
- **[scripts/server-setup.sh](scripts/server-setup.sh)**: 服务器初始化脚本

---

## 🔍 故障排查

### 常见问题

1. **SSH 连接失败**
   - 参考 [SSH_KEY_GUIDE.md](SSH_KEY_GUIDE.md) 中的常见问题部分

2. **部署失败**
   - 查看 GitHub Actions 日志
   - 检查服务器日志：`sudo supervisorctl tail youdaonotelm-server stderr`

3. **服务无法启动**
   - 手动测试运行：`cd /home/flandern/youdaonotelm && ./youdaonotelm-server`
   - 检查配置文件：`cat /home/flandern/youdaonotelm/configs/config.yaml`

4. **端口被占用**
   - 检查端口占用：`netstat -tuln | grep 8080`
   - 停止占用进程：`kill -9 <PID>`

### 获取帮助

```bash
# 查看 Supervisor 状态
sudo supervisorctl status

# 查看 Supervisor 日志
sudo journalctl -u supervisord -f

# 查看应用日志
tail -f /home/flandern/youdaonotelm/logs/server.log

# 检查系统资源
top
df -h
free -h
```

---

## 🔐 安全建议

1. **使用 SSH 密钥认证**: 避免使用密码认证
2. **定期轮换密钥**: 每 3-6 个月更换一次 SSH 密钥
3. **限制 SSH 访问**: 只允许特定 IP 访问
4. **使用非 root 用户**: 创建专用用户运行应用（可选）
5. **监控异常登录**: 检查 `/var/log/secure` 日志
6. **配置防火墙**: 只开放必要的端口

---

## 📊 监控和日志

### 日志位置

- **应用日志**: `/home/flandern/youdaonotelm/logs/server.log`
- **Supervisor 日志**: `/var/log/supervisor/supervisord.log`
- **系统日志**: `/var/log/messages` 或 `/var/log/syslog`

### 监控命令

```bash
# 实时监控日志
tail -f /home/flandern/youdaonotelm/logs/server.log

# 搜索错误日志
grep -i "error\|fatal\|panic" /home/flandern/youdaonotelm/logs/server.log

# 检查系统资源
top
df -h
free -h

# 检查网络连接
netstat -tuln | grep 8080
```

---

## 🔄 回滚操作

### 自动回滚

如果部署失败，可以手动触发回滚：

```bash
cd /home/flandern/youdaonotelm
ls -la youdaonotelm-server.backup.*
sudo supervisorctl stop youdaonotelm-server
cp youdaonotelm-server.backup.20240101_120000 youdaonotelm-server
sudo supervisorctl start youdaonotelm-server
```

### 手动回滚

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

## 📝 最佳实践

1. **代码审查**: 所有 PR 都需要代码审查
2. **测试覆盖**: 保持高测试覆盖率
3. **小步提交**: 频繁提交小的改动
4. **清晰的提交信息**: 使用语义化提交信息
5. **环境隔离**: 使用不同的环境（staging/production）
6. **备份策略**: 每次部署前自动备份
7. **健康检查**: 部署后自动进行健康检查
8. **监控告警**: 配置监控和告警

---

## 🔗 相关链接

- [GitHub Actions 官方文档](https://docs.github.com/en/actions)
- [Supervisor 官方文档](http://supervisord.org/)
- [CentOS 7 文档](https://wiki.centos.org/Documentation)
- [Go 官方文档](https://golang.org/doc/)
- [Gin 框架文档](https://gin-gonic.com/docs/)
- [GORM 文档](https://gorm.io/docs/)

---

## 📞 支持

如果遇到问题，请：

1. 查看相关文档
2. 检查 GitHub Issues
3. 联系维护者

---

**最后更新**: 2026-06-08
