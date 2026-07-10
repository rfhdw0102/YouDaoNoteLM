# 🚀 GitHub Actions CI/CD 快速开始

## 5 分钟快速配置

### 第一步：生成 SSH 密钥（2 分钟）

在本地终端执行：

```bash
# 生成密钥对
ssh-keygen -t ed25519 -C "github-actions" -f ~/.ssh/github_actions

# 查看公钥（复制到服务器）
cat ~/.ssh/github_actions.pub

# 查看私钥（复制到 GitHub）
cat ~/.ssh/github_actions
```

### 第二步：配置服务器（1 分钟）

```bash
# 将公钥添加到服务器
ssh-copy-id -i ~/.ssh/github_actions.pub root@60.205.184.232

# 测试连接
ssh -i ~/.ssh/github_actions root@60.205.184.232 "echo '连接成功！'"
```

### 第三步：配置 GitHub Secrets（1 分钟）

1. 进入 GitHub 仓库页面
2. 点击 `Settings` → `Secrets and variables` → `Actions`
3. 点击 `New repository secret`
4. 添加以下 3 个 Secrets：

| Name | Value |
|------|-------|
| `SSH_HOST` | `60.205.184.232` |
| `SSH_USERNAME` | `root` |
| `SSH_PRIVATE_KEY` | 粘贴私钥内容（包括 `-----BEGIN...` 和 `-----END...`） |

### 第四步：初始化服务器（1 分钟）

```bash
# 上传初始化脚本
scp .github/scripts/server-setup.sh root@60.205.184.232:/home/flandern/

# 在服务器上执行
ssh root@60.205.184.232 "cd /home/flandern && bash server-setup.sh"
```

### 第五步：上传配置文件

```bash
# 上传你的配置文件
scp configs/config.yaml root@60.205.184.232:/home/flandern/youdaonotelm/configs/
```

---

## ✅ 配置完成！

现在，每次你将代码合并到 `develop` 分支时，GitHub Actions 会自动：

1. ✅ 运行代码检查
2. ✅ 执行单元测试
3. ✅ 构建应用
4. ✅ 部署到服务器
5. ✅ 启动服务
6. ✅ 进行健康检查

---

## 🎯 使用方法

### 自动部署（推荐）

```bash
# 1. 在本地开发
git checkout develop
git pull origin develop

# 2. 进行开发...

# 3. 提交代码
git add .
git commit -m "feat: 新功能"
git push origin develop

# 4. 创建 PR 并合并到 develop 分支
# GitHub Actions 会自动部署！
```

### 手动部署

1. 进入 GitHub 仓库页面
2. 点击 `Actions` → `CD`
3. 点击 `Run workflow`
4. 选择环境（staging/production）
5. 点击 `Run workflow` 按钮

---

## 🔧 常用命令

### 服务器管理

```bash
# 查看服务状态
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

## ❓ 遇到问题？

### 快速排查

```bash
# 1. 检查 SSH 连接
ssh -i ~/.ssh/github_actions root@60.205.184.232 "echo 'SSH 正常'"

# 2. 检查 Supervisor 状态
sudo supervisorctl status youdaonotelm-server

# 3. 查看错误日志
sudo supervisorctl tail youdaonotelm-server stderr

# 4. 手动测试运行
cd /home/flandern/youdaonotelm
./youdaonotelm-server
```

### 详细文档

- **[SSH 密钥配置](SSH_KEY_GUIDE.md)**: SSH 密钥生成和配置
- **[详细部署指南](DEPLOYMENT_GUIDE.md)**: 完整的部署步骤
- **[快速参考卡](QUICK_REFERENCE.md)**: 常用命令速查
- **[完整说明](README.md)**: 项目整体说明

---

## 📞 需要帮助？

1. 查看 GitHub Actions 日志
2. 检查服务器日志
3. 参考相关文档
4. 联系维护者

---

**祝你部署顺利！🎉**
