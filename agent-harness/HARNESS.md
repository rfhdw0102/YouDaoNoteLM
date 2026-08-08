# YoudaoNoteLM CLI Harness

## Architecture

The CLI is built as a Python package using `argparse` for command parsing and `urllib` for HTTP requests.

The wrapper targets `http://localhost:8080` by default. Start the API before using
authenticated commands, and pass `--base-url` before the command when the API is
hosted elsewhere.

### Directory Structure

```
agent-harness/
├── setup.py                          # Package installation config
├── HARNESS.md                        # This file
└── cli_anything/
    ├── __init__.py                   # Package marker
    └── youdaonotelm/
        ├── __init__.py               # CLI main logic (~1800 lines)
        └── __main__.py               # Module entry point
```

### Key Components

1. **APIClient** - HTTP client for YoudaoNoteLM API
   - Token management (load/save from `~/.config/youdaonotelm/token.json`)
   - Auto-attaches Bearer token to requests
   - Handles HTTP errors with user-friendly messages

2. **Command Functions** - Each `cmd_*` function handles a command group
   - Pattern: `if args.subcmd == "xxx": ...`
   - Always check `result.get("code") == 0` for success
   - Use `print_json()` for raw output, `print_table()` for tabular data

3. **Argument Parser** - `build_parser()` defines all commands
   - Each command group has a subparser with `dest="subcmd"`
   - Required args are positional, optional args use `--flag`

## Patterns

### Adding a New Command

1. Add command function:
```python
def cmd_newfeature(args, root: Path):
    """New feature description."""
    client = get_client(args)

    if args.subcmd == "action":
        result = client.post("/api/v1/endpoint", {"key": "value"})
        if result.get("code") == 0:
            print(f"✓ Success message")
        else:
            print(f"Error: {result.get('message')}")
```

2. Add to argument parser in `build_parser()`:
```python
nf_parser = subparsers.add_parser("newfeature", help="Feature description")
nf_sub = nf_parser.add_subparsers(dest="subcmd")

nf_action = nf_sub.add_parser("action", help="Action description")
nf_action.add_argument("param", help="Parameter description")
```

3. Add to command dispatch in `main()`:
```python
commands = {
    ...
    "newfeature": cmd_newfeature,
}
```

### Error Handling

- Always check `result.get("code") == 0` for API success
- Print error message from `result.get("message")`
- Use `sys.exit(1)` for fatal errors
- For confirmation prompts: `if not args.yes: confirm = input("...")`

### Output Formats

- **Success**: `print(f"✓ {message}")`
- **Error**: `print(f"Error: {message}")`
- **JSON**: `print_json(data)` - pretty prints JSON
- **Table**: `print_table(headers, rows)` - formatted table
- **Info**: Direct `print()` for informational messages

### File Upload Commands

For multipart/form-data (file uploads), show curl command instead:
```python
print(f"Use: curl -X POST {client.base_url}/api/v1/endpoint \\")
print(f"  -H 'Authorization: Bearer <token>' \\")
print(f"  -F 'file=@{args.file}'")
```

## Coverage Map

### Auth (Complete)
- login, logout, whoami, register, send-code, reset-password, refresh

### User Profile (Complete)
- get, update, update-username, change-password, delete

### User Config (Complete)
- LLM: list, create, update, delete, test
- Search: list, create, update, delete
- Embedding: list, create, update, delete
- ASR: list, create, update, delete
- Reranker: list, create, update, delete
- Active: get active config by type

### Providers (Complete)
- list, active

### Notebook (Complete)
- list, create, rename, delete

### Source (Complete)
- list, get, content, rename, delete, batch-delete, delete-failed
- original, download, reimport-all, reimport-selected, from-note

### Chat (Complete)
- list, create, get, delete, messages, send

### Generation (Complete)
- submit, list, get, delete, export

### Youdao (Complete)
- bind, unbind, status, notes, import, import-batch

### Import (Complete)
- file, url, task, delete-task
- audio-preview, audio-status, audio-confirm

### Search (Complete)
- query, import-results

### Memory (Complete)
- list, set, delete

### Feedback (Complete)
- up, down, delete

### Admin (Complete)
- users, update-user-status
- config-status, config-get, config-add, config-update, config-delete
- feedback-overview, feedback-list, feedback-export, mysql

### Infrastructure (Complete)
- server: run, build, status
- docker: up, down, restart, logs, ps, pull, build
- config: show, validate
- health, version

## Testing

Run tests with:
```bash
cd agent-harness
python -m cli_anything.youdaonotelm --help
python -m cli_anything.youdaonotelm <command> --help
python -m cli_anything.youdaonotelm <command> <subcommand> --help
```

## Version History

- v3.0.0 - Added all missing API endpoints (auth, profile, config, admin, source, import, search)
- v2.0.0 - Added business operations (notebook, source, chat, generation, youdao, memory, feedback)
- v1.0.0 - Initial infrastructure-only CLI (server, docker, config, health)
