# YoudaoNoteLM CLI

CLI wrapper for YoudaoNoteLM - RAG-based Youdao Cloud Notes Knowledge Q&A System.

## Installation

```bash
cd agent-harness
pip install -e .
```

使用前请先在项目根目录启动 API（默认 `http://localhost:8080`）：

```bash
cd ..
docker compose up -d
cd agent-harness
```

Or run directly:
```bash
python -m cli_anything.youdaonotelm <command>
```

## Quick Start

```bash
# Login
youdaonotelm login your@email.com password

# List notebooks
youdaonotelm notebook list

# Create a notebook
youdaonotelm notebook create "My Knowledge Base"

# List sources in notebook
youdaonotelm source list 1

# Search in notebook
youdaonotelm search query 1 "what is RAG?"

# Start a chat
youdaonotelm chat create 1 --title "RAG Discussion"

# Generate content
youdaonotelm generation submit 1 mindmap
```

## Commands

### Authentication

| Command | Description |
|---------|-------------|
| `login [email] [password]` | Login and save token |
| `logout` | Clear saved token |
| `whoami` | Show current user |
| `register [email] [pw] [user] [code]` | Register new user |
| `send-code <email>` | Send verification code |
| `reset-password <email> <code> <new_pw>` | Reset password |
| `refresh` | Refresh access token |

### User Profile

| Command | Description |
|---------|-------------|
| `profile get` | Get profile details |
| `profile update --username <name>` | Update profile |
| `profile update-username <name>` | Update username |
| `profile change-password <old> <new>` | Change password |
| `profile delete` | Delete account |

### User Configuration

| Command | Description |
|---------|-------------|
| `config-llm list/create/update/delete/test` | Manage LLM configs |
| `config-search list/create/update/delete` | Manage search configs |
| `config-embedding list/create/update/delete` | Manage embedding configs |
| `config-asr list/create/update/delete` | Manage ASR configs |
| `config-reranker list/create/update/delete` | Manage reranker configs |
| `config-active <type>` | Get active config |

### Providers

| Command | Description |
|---------|-------------|
| `providers list` | List all providers |
| `providers active` | Get active providers |

### Notebooks

| Command | Description |
|---------|-------------|
| `notebook list` | List all notebooks |
| `notebook create <name>` | Create notebook |
| `notebook rename <id> <name>` | Rename notebook |
| `notebook delete <id>` | Delete notebook |

### Sources

| Command | Description |
|---------|-------------|
| `source list <nb_id>` | List sources |
| `source get <nb_id> <id>` | Get source details |
| `source content <nb_id> <id>` | Get source content |
| `source original <nb_id> <id>` | Get original format |
| `source download <nb_id> <id>` | Get download URL |
| `source rename <nb_id> <id> <name>` | Rename source |
| `source delete <nb_id> <id>` | Delete source |
| `source batch-delete <nb_id> <ids>` | Batch delete |
| `source delete-failed <nb_id>` | Delete failed sources |
| `source reimport-all` | Reimport all |
| `source reimport-selected <ids>` | Reimport selected |
| `source from-note <nb_id> <title> <content>` | Create from note |

### Chat

| Command | Description |
|---------|-------------|
| `chat list <nb_id>` | List conversations |
| `chat create <nb_id>` | Create conversation |
| `chat get <id>` | Get conversation |
| `chat delete <id>` | Delete conversation |
| `chat messages <id>` | Get messages |
| `chat send <id> <nb_id> <msg>` | Send message |

### Content Generation

| Command | Description |
|---------|-------------|
| `generation submit <nb_id> <type>` | Submit task (mindmap/ppt/quiz/note) |
| `generation list` | List tasks |
| `generation get <id>` | Get task details |
| `generation delete <id>` | Delete task |
| `generation export <type> <title>` | Export content |

### Youdao Cloud Notes

| Command | Description |
|---------|-------------|
| `youdao bind <api_key>` | Bind account |
| `youdao unbind` | Unbind account |
| `youdao status` | Check binding |
| `youdao notes` | Browse notes |
| `youdao import <nb_id> <file_id>` | Import note |
| `youdao import-batch <nb_id> --file-ids <ids>` | Batch import |

### Import

| Command | Description |
|---------|-------------|
| `import file <nb_id> <file>` | Import file |
| `import url <nb_id> <url>` | Import from URL |
| `import task <task_id>` | Check task status |
| `import delete-task <task_id>` | Delete task |
| `import audio-preview <nb_id> <file>` | Preview audio |
| `import audio-status <preview_id>` | Check audio status |
| `import audio-confirm <preview_id> <content>` | Confirm audio |

### Search

| Command | Description |
|---------|-------------|
| `search query <nb_id> <query>` | Search in notebook |
| `search import-results <nb_id> --urls <urls>` | Import results |

### Memory

| Command | Description |
|---------|-------------|
| `memory list` | List preferences |
| `memory set <type> <content>` | Set preference |
| `memory delete <type>` | Delete preference |

### Feedback

| Command | Description |
|---------|-------------|
| `feedback up <msg_id>` | Like answer |
| `feedback down <msg_id>` | Dislike answer |
| `feedback delete <msg_id>` | Delete feedback |

### Admin

| Command | Description |
|---------|-------------|
| `admin users` | List users |
| `admin update-user-status <id> <status>` | Update user status |
| `admin config-status` | Get config status |
| `admin config-get <group>` | Get config group |
| `admin config-add <group> <key> <value>` | Add config |
| `admin config-update <group> <key> <value>` | Update config |
| `admin config-delete <group> <key>` | Delete config |
| `admin feedback-overview` | Feedback statistics |
| `admin feedback-list` | List feedback |
| `admin feedback-export` | Export CSV |
| `admin mysql` | MySQL console |

### Infrastructure

| Command | Description |
|---------|-------------|
| `server run` | Start server |
| `server build` | Build binary |
| `server status` | Check status |
| `docker up/down/restart/logs/ps/pull/build` | Docker management |
| `config show/validate` | Config management |
| `health` | Health check |
| `version` | Show version |

## Global Options

- `--base-url <url>` - API base URL (default: http://localhost:8080)
- `-C, --chdir <dir>` - Change directory before executing
- `--version` - Show version

全局选项必须放在子命令之前，例如：

```bash
python -m cli_anything.youdaonotelm --base-url http://localhost:8081 health
```

登录出现 `WinError 10061` 时，请先检查目标 API 地址是否有服务监听。

## Examples

```bash
# Login and save token
youdaonotelm login user@example.com mypassword

# Create LLM config
youdaonotelm config-llm create "My GPT" openai sk-xxx gpt-4

# List all LLM configs
youdaonotelm config-llm list

# Import from URL
youdaonotelm import url 1 https://example.com/article

# Generate mindmap
youdaonotelm generation submit 1 mindmap --source-ids 1 2 3

# Admin: export feedback CSV
youdaonotelm admin feedback-export --from-date 2024-01-01T00:00:00Z --to-date 2024-12-31T23:59:59Z
```

## Version

v3.0.0 - Full API coverage
