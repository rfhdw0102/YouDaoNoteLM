# 🔧 GitHub Actions 配置步骤

## 已完成

✅ 生成 SSH 密钥对

## 接下来的步骤

### 第一步：配置 GitHub Secrets（必需）

1. 进入 GitHub 仓库页面
2. 点击 `Settings` → `Secrets and variables` → `Actions`
3. 点击 `New repository secret`
4. 添加以下 3 个 Secrets：

#### SSH_HOST
```
60.205.184.232
```

#### SSH_USERNAME
```
root
```

#### SSH_PRIVATE_KEY
复制以下完整内容（包括开头和结尾）：
```
-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW
QyNTUxOQAAACAlzy0wXHom4YJFNvHeKBNGaLo3gW2WRiI5SED2DFdOPAAAAJj12/V99dv1
fQAAAAtzc2gtZWQyNTUxOQAAACAlzy0wXHom4YJFNvHeKBNGaLo3gW2WRiI5SED2DFdOPA
AAAEBgxAfVMHMaMjrhAoKuvUXlU0p6v82oCEgd6ntzwJL1wCXPLTBceibhgkU28d4oE0Zo
ujeBbZZGIjlIQPYMV048AAAADmdpdGh1Yi1hY3Rpb25zAQIDBAUGBw==
-----END OPENSSH PRIVATE KEY-----
```

### 第二步：配置服务器

在本地终端执行：

```bash
# 复制公钥到服务器
ssh-copy-id -i ~/.ssh/github_actions.pub root@60.205.184.232

# 测试连接
ssh -i ~/.ssh/github_actions root@60.205.184.232 "echo '连接成功！'"
```

### 第三步：初始化服务器

```bash
# 上传初始化脚本
scp .github/scripts/server-setup.sh root@60.205.184.232:/home/flandern/

# 在服务器上执行
ssh root@60.205.184.232 "cd /home/flandern && bash server-setup.sh"
```

### 第四步：上传配置文件

```bash
# 上传你的配置文件
scp configs/config.yaml root@60.205.184.232:/home/flandern/youdaonotelm/configs/
```

### 第五步：验证配置

```bash
# 测试 SSH 连接
ssh -i ~/.ssh/github_actions root@60.205.184.232 "echo 'SSH 配置成功！'"

# 查看服务器目录
ssh -i ~/.ssh/github_actions root@60.205.184.232 "ls -la /home/flandern/youdaonotelm/"
```

---

## 🎯 快速命令参考

### 本地命令

```bash
# 查看公钥
cat ~/.ssh/github_actions.pub

# 查看私钥
cat ~/.ssh/github_actions

# 测试 SSH 连接
ssh -i ~/.ssh/github_actions root@60.205.184.232 "echo '连接成功'"

# 上传配置文件
scp configs/config.yaml root@60.205.184.232:/home/flandern/youdaonotelm/configs/

# 上传初始化脚本
scp .github/scripts/server-setup.sh root@60.205.184.232:/home/flandern/
```

### 服务器命令

```bash
# 查看部署目录
ls -la /home/flandern/youdaonotelm/

# 查看 Supervisor 状态
sudo supervisorctl status youdaonotelm-server

# 启动服务
sudo supervisorctl start youdaonotelm-server

# 查看日志
tail -f /home/flandern/youdaonotelm/logs/server.log
```

---

## ✅ 验证清单

- [ ] SSH 密钥已生成
- [ ] GitHub Secrets 已配置
- [ ] 服务器 SSH 连接正常
- [ ] 服务器初始化完成
- [ ] 配置文件已上传
- [ ] 可以手动部署成功

---

## 🚀 开始使用

完成上述步骤后，每次将代码合并到 `develop` 分支时，GitHub Actions 会自动：

1. ✅ 运行代码检查
2. ✅ 执行单元测试
3. ✅ 构建应用
4. ✅ 部署到服务器
5. ✅ 启动服务
6. ✅ 进行健康检查

---

## ❓ 遇到问题？

### SSH 连接失败

```bash
# 检查 SSH 配置
sudo vi /etc/ssh/sshd_config

# 确保以下配置
PubkeyAuthentication yes
PasswordAuthentication yes  # 临时启用

# 重启 SSH 服务
sudo systemctl restart sshd
```

### 部署失败

```bash
# 查看 GitHub Actions 日志
# 在 GitHub 仓库页面 → Actions → 点击失败的工作流

# 查看服务器日志
sudo supervisorctl tail youdaonotelm-server stderr
```

---

## 📚 相关文档

- [SSH 密钥配置指南](SSH_KEY_GUIDE.md)
- [详细部署指南](DEPLOYMENT_GUIDE.md)
- [快速参考卡](QUICK_REFERENCE.md)
- [完整说明](README.md)

---

**配置完成后，你就可以开始使用 CI/CD 了！🎉**
