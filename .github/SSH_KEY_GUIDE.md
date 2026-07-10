# SSH 密钥配置指南

## 📋 概述

GitHub Actions 需要通过 SSH 连接到你的服务器进行部署。本指南将帮助你生成和配置 SSH 密钥。

---

## 🔑 第一步：生成 SSH 密钥对

### 在本地计算机上执行

```bash
# 生成 ED25519 密钥对（推荐，更安全）
ssh-keygen -t ed25519 -C "github-actions" -f ~/.ssh/github_actions

# 或者生成 RSA 密钥对（兼容性更好）
ssh-keygen -t rsa -b 4096 -C "github-actions" -f ~/.ssh/github_actions
```

**执行过程**：
```
Generating public/private ed25519 key pair.
Enter file in which to save the key (/home/your_user/.ssh/github_actions): [按 Enter 使用默认路径]
Enter passphrase (empty for no passphrase): [直接按 Enter，不设置密码]
Enter same passphrase again: [再次按 Enter]
Your identification has been saved in /home/your_user/.ssh/github_actions.
Your public key has been saved in /home/your_user/.ssh/github_actions.pub.
```

---

## 📤 第二步：查看生成的密钥

### 查看私钥（用于 GitHub Secrets）

```bash
cat ~/.ssh/github_actions
```

**输出示例**：
```
-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW
QyNTUxOQAAACBhK7PJSKJM3JM3JM3JM3JM3JM3JM3JM3JM3JM3JM3JM3JMAAAAJ...
...（很长的内容）...
-----END OPENSSH PRIVATE KEY-----
```

**⚠️ 重要**：这是你的私钥，需要添加到 GitHub Secrets。**不要分享给任何人！**

### 查看公钥（用于服务器）

```bash
cat ~/.ssh/github_actions.pub
```

**输出示例**：
```
ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGE7s8lIokzckzckzckzckzckzckzckzckzckzckzckzck github-actions
```

---

## 🖥️ 第三步：将公钥添加到服务器

### 方法一：使用 ssh-copy-id（推荐）

```bash
# 将公钥复制到服务器
ssh-copy-id -i ~/.ssh/github_actions.pub root@60.205.184.232
```

### 方法二：手动添加

```bash
# 1. 登录到服务器
ssh root@60.205.184.232

# 2. 创建 .ssh 目录（如果不存在）
mkdir -p ~/.ssh
chmod 700 ~/.ssh

# 3. 将公钥添加到 authorized_keys
echo "你的公钥内容" >> ~/.ssh/authorized_keys
chmod 600 ~/.ssh/authorized_keys

# 4. 退出服务器
exit
```

### 方法三：使用脚本自动添加

```bash
# 在本地执行以下命令
PUB_KEY=$(cat ~/.ssh/github_actions.pub)
ssh root@60.205.184.232 "mkdir -p ~/.ssh && chmod 700 ~/.ssh && echo '$PUB_KEY' >> ~/.ssh/authorized_keys && chmod 600 ~/.ssh/authorized_keys"
```

---

## ✅ 第四步：测试 SSH 连接

### 测试密钥认证

```bash
# 使用私钥连接服务器
ssh -i ~/.ssh/github_actions root@60.205.184.232
```

如果成功登录，说明 SSH 密钥配置正确！

### 测试 GitHub Actions 用法

```bash
# 测试非交互式命令
ssh -i ~/.ssh/github_actions root@60.205.184.232 "echo 'SSH 连接成功！'"
```

---

## 🔧 第五步：配置 GitHub Secrets

### 1. 获取私钥内容

```bash
# 复制私钥内容
cat ~/.ssh/github_actions
```

### 2. 添加到 GitHub

1. 进入 GitHub 仓库页面
2. 点击 `Settings` → `Secrets and variables` → `Actions`
3. 点击 `New repository secret`
4. 添加以下 Secrets：

| Name | Value |
|------|-------|
| `SSH_HOST` | `60.205.184.232` |
| `SSH_USERNAME` | `root` |
| `SSH_PRIVATE_KEY` | 粘贴整个私钥内容（包括 `-----BEGIN OPENSSH PRIVATE KEY-----` 和 `-----END OPENSSH PRIVATE KEY-----`） |

---

## 🔒 安全最佳实践

### 1. 使用专用密钥

- 为 GitHub Actions 创建专用密钥，不要使用个人密钥
- 密钥注释中说明用途（如 `github-actions`）

### 2. 限制密钥权限

在服务器上限制 `authorized_keys` 的权限：

```bash
# 在服务器上执行
chmod 600 ~/.ssh/authorized_keys
chmod 700 ~/.ssh
```

### 3. 配置 SSH 限制

编辑服务器的 SSH 配置文件：

```bash
sudo vi /etc/ssh/sshd_config
```

添加或修改以下配置：

```bash
# 禁用密码认证（推荐）
PasswordAuthentication no

# 启用公钥认证
PubkeyAuthentication yes

# 指定授权密钥文件
AuthorizedKeysFile .ssh/authorized_keys

# 限制 SSH 用户（可选）
AllowUsers root
```

重启 SSH 服务：

```bash
sudo systemctl restart sshd
```

### 4. 定期轮换密钥

建议每 3-6 个月更换一次 SSH 密钥：

```bash
# 1. 生成新密钥
ssh-keygen -t ed25519 -C "github-actions-new" -f ~/.ssh/github_actions_new

# 2. 将新公钥添加到服务器
ssh-copy-id -i ~/.ssh/github_actions_new.pub root@60.205.184.232

# 3. 更新 GitHub Secrets

# 4. 测试新密钥
ssh -i ~/.ssh/github_actions_new root@60.205.184.232

# 5. 删除旧密钥
rm ~/.ssh/github_actions ~/.ssh/github_actions.pub
```

---

## ❓ 常见问题

### 1. 权限被拒绝

**错误**：`Permission denied (publickey,gssapi-keyex,gssapi-with-mic)`

**解决**：
```bash
# 检查服务器 SSH 配置
sudo vi /etc/ssh/sshd_config

# 确保以下配置
PubkeyAuthentication yes
PasswordAuthentication yes  # 临时启用，配置完成后禁用

# 重启 SSH
sudo systemctl restart sshd

# 检查 authorized_keys 权限
ls -la ~/.ssh/
chmod 700 ~/.ssh
chmod 600 ~/.ssh/authorized_keys
```

### 2. 密钥格式错误

**错误**：`Load key "xxx": invalid format`

**解决**：
```bash
# 重新生成密钥
ssh-keygen -t ed25519 -C "github-actions" -f ~/.ssh/github_actions

# 确保复制完整内容
cat ~/.ssh/github_actions
```

### 3. 连接超时

**错误**：`Connection timed out`

**解决**：
```bash
# 检查服务器防火墙
sudo firewall-cmd --list-all

# 开放 SSH 端口（默认 22）
sudo firewall-cmd --permanent --add-port=22/tcp
sudo firewall-cmd --reload

# 检查 SSH 服务状态
sudo systemctl status sshd
```

### 4. 主机密钥验证失败

**错误**：`Host key verification failed`

**解决**：
```bash
# 在本地添加服务器到已知主机
ssh-keyscan -H 60.205.184.232 >> ~/.ssh/known_hosts

# 或者在 SSH 命令中禁用严格主机密钥检查（不推荐用于生产）
ssh -o StrictHostKeyChecking=no root@60.205.184.232
```

---

## 📝 快速参考

### 生成密钥

```bash
ssh-keygen -t ed25519 -C "github-actions" -f ~/.ssh/github_actions
```

### 查看公钥

```bash
cat ~/.ssh/github_actions.pub
```

### 查看私钥

```bash
cat ~/.ssh/github_actions
```

### 复制公钥到服务器

```bash
ssh-copy-id -i ~/.ssh/github_actions.pub root@60.205.184.232
```

### 测试连接

```bash
ssh -i ~/.ssh/github_actions root@60.205.184.232 "echo '成功！'"
```

---

## 🔗 相关资源

- [GitHub Actions SSH 部署指南](https://docs.github.com/en/actions/deployment/deploying-to-your-cloud-provider/deploying-to-azure/deploying-to-azure-vm)
- [OpenSSH 官方文档](https://www.openssh.com/manual.html)
- [SSH 密钥管理最佳实践](https://www.ssh.com/academy/ssh/key-management)
