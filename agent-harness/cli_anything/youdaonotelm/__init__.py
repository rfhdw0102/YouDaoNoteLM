#!/usr/bin/env python3
"""
YoudaoNoteLM CLI - RAG-based Youdao Cloud Notes Knowledge Q&A System

A comprehensive CLI wrapper for managing YoudaoNoteLM deployments
and interacting with the business API.
"""

import argparse
import json
import os
import subprocess
import sys
import time
from pathlib import Path
from typing import Optional
from urllib import request as urllib_request
from urllib.error import HTTPError, URLError


# ─── Constants ────────────────────────────────────────────────────────────────

VERSION = "3.0.0"
APP_NAME = "youdaonotelm"
DEFAULT_PORT = 8080
DEFAULT_BASE_URL = "http://localhost:8080"
HEALTH_ENDPOINT = "/api/v1/health"


def _captcha_drag_distance(absolute_x: int, slider_start_x: int) -> int:
    """Convert the displayed absolute gap X to the API's drag distance."""
    return absolute_x - slider_start_x

# Config file for storing auth token
CONFIG_DIR = Path.home() / ".config" / "youdaonotelm"
TOKEN_FILE = CONFIG_DIR / "token.json"


# ─── Auth & API Client ────────────────────────────────────────────────────────

class APIClient:
    """HTTP client for YoudaoNoteLM API."""

    def __init__(self, base_url: str = DEFAULT_BASE_URL, token: str = None):
        self.base_url = base_url.rstrip("/")
        self.token = token or self._load_token()

    def _load_token(self) -> Optional[str]:
        """Load token from config file."""
        if TOKEN_FILE.exists():
            try:
                data = json.loads(TOKEN_FILE.read_text())
                return data.get("access_token")
            except Exception:
                pass
        return None

    def _save_token(self, access_token: str, refresh_token: str = None):
        """Save token to config file."""
        CONFIG_DIR.mkdir(parents=True, exist_ok=True)
        data = {"access_token": access_token}
        if refresh_token:
            data["refresh_token"] = refresh_token
        TOKEN_FILE.write_text(json.dumps(data, indent=2))

    def _request(self, method: str, path: str, data: dict = None,
                 json_body: bool = True) -> dict:
        """Make HTTP request to API."""
        url = f"{self.base_url}{path}"
        headers = {"Content-Type": "application/json"}
        if self.token:
            headers["Authorization"] = f"Bearer {self.token}"

        body = json.dumps(data).encode() if data and json_body else None
        req = urllib_request.Request(url, data=body, headers=headers, method=method)

        try:
            with urllib_request.urlopen(req, timeout=30) as resp:
                return json.loads(resp.read().decode())
        except HTTPError as e:
            error_body = e.read().decode() if e.fp else ""
            try:
                error_data = json.loads(error_body)
                print(f"Error {e.code}: {error_data.get('message', error_body)}")
            except json.JSONDecodeError:
                print(f"Error {e.code}: {error_body}")
            sys.exit(1)
        except URLError as e:
            print(f"Connection error: {e.reason}")
            sys.exit(1)

    def get(self, path: str) -> dict:
        return self._request("GET", path)

    def post(self, path: str, data: dict = None) -> dict:
        return self._request("POST", path, data)

    def put(self, path: str, data: dict = None) -> dict:
        return self._request("PUT", path, data)

    def delete(self, path: str) -> dict:
        return self._request("DELETE", path)

    def login(self, email: str, password: str) -> dict:
        """Login and save token."""
        result = self._request("POST", "/api/v1/auth/login", {
            "email": email,
            "password": password
        })
        if result.get("code") == 0:
            data = result.get("data", {})
            self.token = data.get("access_token")
            self._save_token(self.token, data.get("refresh_token"))
            print(f"✓ Logged in as {email}")
            return data
        else:
            print(f"Login failed: {result.get('message')}")
            sys.exit(1)

    def logout(self):
        """Clear saved token."""
        if TOKEN_FILE.exists():
            TOKEN_FILE.unlink()
        self.token = None
        print("✓ Logged out")


def get_client(args) -> APIClient:
    """Get API client from args."""
    base_url = getattr(args, 'base_url', None) or DEFAULT_BASE_URL
    return APIClient(base_url)


# ─── Utilities ────────────────────────────────────────────────────────────────

def find_project_root() -> Path:
    """Find the project root by looking for docker-compose.yml or go.mod."""
    current = Path.cwd()
    for parent in [current] + list(current.parents):
        if (parent / "docker-compose.yml").exists() and (parent / "go.mod").exists():
            return parent
    print("Error: Not inside a YoudaoNoteLM project (missing docker-compose.yml or go.mod)")
    sys.exit(1)


def run_cmd(cmd: list[str], cwd: Optional[Path] = None, check: bool = True,
            capture: bool = False) -> subprocess.CompletedProcess:
    """Run a shell command with error handling."""
    try:
        return subprocess.run(
            cmd, cwd=cwd, check=check,
            capture_output=capture, text=True
        )
    except subprocess.CalledProcessError as e:
        if not check:
            return e
        print(f"Command failed: {' '.join(cmd)}")
        if e.stderr:
            print(f"Error: {e.stderr}")
        sys.exit(1)
    except FileNotFoundError:
        print(f"Command not found: {cmd[0]}")
        sys.exit(1)


def docker_compose(root: Path, args: list[str], **kwargs) -> subprocess.CompletedProcess:
    """Run docker compose command."""
    return run_cmd(["docker", "compose"] + args, cwd=root, **kwargs)


def check_health(port: int = DEFAULT_PORT) -> bool:
    """Check if the application is healthy."""
    try:
        url = f"http://localhost:{port}{HEALTH_ENDPOINT}"
        req = urllib_request.urlopen(url, timeout=5)
        return req.status == 200
    except Exception:
        return False


def print_json(data):
    """Pretty print JSON data."""
    print(json.dumps(data, indent=2, ensure_ascii=False))


def print_table(headers: list[str], rows: list[list[str]]):
    """Print a formatted table."""
    if not rows:
        print("(empty)")
        return

    # Calculate column widths
    widths = [len(h) for h in headers]
    for row in rows:
        for i, cell in enumerate(row):
            if i < len(widths):
                widths[i] = max(widths[i], len(str(cell)))

    # Print header
    header_line = " | ".join(h.ljust(widths[i]) for i, h in enumerate(headers))
    print(header_line)
    print("-" * len(header_line))

    # Print rows
    for row in rows:
        print(" | ".join(str(cell).ljust(widths[i]) for i, cell in enumerate(row)))


# ─── Auth Commands ────────────────────────────────────────────────────────────

def cmd_login(args, root: Path):
    """Login to YoudaoNoteLM."""
    import base64
    import tempfile
    import webbrowser

    client = get_client(args)
    email = args.email or input("Email: ")
    password = args.password or input("Password: ")

    # 获取验证码
    print("获取验证码...")
    captcha_result = client.get("/api/v1/auth/captcha")
    if captcha_result.get("code") != 0:
        print(f"Error: 获取验证码失败: {captcha_result.get('message')}")
        sys.exit(1)

    captcha_data = captcha_result.get("data", {})
    captcha_id = captcha_data.get("captcha_id")
    background = captcha_data.get("background", "")
    slider = captcha_data.get("slider", "")
    slider_size = captcha_data.get("slider_size", 44)
    bg_width = captcha_data.get("bg_width", 300)
    bg_height = captcha_data.get("bg_height", 150)
    slider_start_x = captcha_data.get("slider_start_x", 0)
    slider_start_y = captcha_data.get("slider_start_y", 0)

    # 创建带滑块交互的 HTML 验证页面（使用字符串拼接避免 f-string 转义问题）
    try:
        # 构建 HTML 各部分
        html_head = '''<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>滑块验证码</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            display: flex;
            justify-content: center;
            align-items: center;
            min-height: 100vh;
            margin: 0;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
        }
        .container {
            background: white;
            padding: 30px;
            border-radius: 16px;
            box-shadow: 0 20px 60px rgba(0,0,0,0.3);
            text-align: center;
            max-width: 400px;
            width: 90%;
        }
        h2 { color: #333; margin-bottom: 10px; font-size: 24px; }
        .subtitle { color: #666; margin-bottom: 20px; font-size: 14px; }
        .captcha-images { display: flex; flex-direction: column; align-items: center; gap: 15px; margin: 20px 0; }
        .image-container { position: relative; border: 2px solid #e0e0e0; border-radius: 8px; overflow: hidden; box-shadow: 0 4px 12px rgba(0,0,0,0.1); }
        .image-label { position: absolute; top: 8px; left: 8px; background: rgba(0,0,0,0.7); color: white; padding: 4px 10px; border-radius: 4px; font-size: 12px; z-index: 10; }
        .captcha-bg { display: block; }
        .captcha-piece { position: absolute; z-index: 5; pointer-events: none; }
        .slider-preview { display: flex; align-items: center; gap: 15px; padding: 15px; background: #f8f9fa; border-radius: 8px; margin: 15px 0; }
        .slider-img-container { border: 2px solid #4CAF50; border-radius: 4px; overflow: hidden; }
        .slider-img { display: block; }
        .slider-info { text-align: left; font-size: 13px; color: #666; }
        .slider-info strong { color: #333; display: block; margin-bottom: 5px; }
        .slider-container { position: relative; width: ''' + str(bg_width) + '''px; height: 50px; background: #e8e8e8; border-radius: 25px; margin: 20px auto; overflow: hidden; border: 2px solid #ddd; }
        .slider-track { position: absolute; top: 0; left: 0; height: 100%; background: linear-gradient(90deg, #4CAF50, #45a049); width: 0%; transition: width 0.05s; }
        .slider-thumb { position: absolute; top: 50%; left: 0; transform: translateY(-50%); width: 46px; height: 46px; background: white; border: 3px solid #4CAF50; border-radius: 50%; cursor: grab; z-index: 10; display: flex; align-items: center; justify-content: center; font-size: 18px; user-select: none; box-shadow: 0 2px 8px rgba(0,0,0,0.2); }
        .slider-thumb:hover { border-color: #45a049; }
        .slider-thumb:active { cursor: grabbing; background: #e8f5e9; border-color: #2e7d32; }
        .slider-hint { text-align: center; color: #999; font-size: 12px; margin-top: 5px; }
        .position-display { margin-top: 15px; font-size: 18px; color: #333; font-weight: bold; }
        .position-display span { color: #4CAF50; font-size: 24px; }
        .result { margin-top: 20px; padding: 15px; border-radius: 8px; display: none; font-size: 14px; }
        .result.show { display: block; }
        .result.success { background: #e8f5e9; color: #2e7d32; border: 1px solid #a5d6a7; }
        .submit-btn { margin-top: 20px; padding: 14px 40px; background: linear-gradient(90deg, #4CAF50, #45a049); color: white; border: none; border-radius: 25px; font-size: 16px; font-weight: bold; cursor: pointer; box-shadow: 0 4px 12px rgba(76, 175, 80, 0.4); }
        .submit-btn:hover { transform: translateY(-2px); box-shadow: 0 6px 16px rgba(76, 175, 80, 0.5); }
        .submit-btn:disabled { background: #ccc; box-shadow: none; cursor: not-allowed; transform: none; }
        .instructions { margin-top: 20px; padding: 15px; background: #fff3e0; border-radius: 8px; font-size: 13px; color: #e65100; border: 1px solid #ffe0b2; }
        .instructions ol { text-align: left; margin-top: 8px; padding-left: 20px; }
        .instructions li { margin: 5px 0; }
    </style>
</head>
<body>
    <div class="container">
        <h2>🔐 滑块验证码</h2>
        <p class="subtitle">请将滑块拖动到图片中缺口的位置</p>

        <div class="captcha-images">
            <div class="image-container">
                <div class="image-label">📷 背景图片（找到缺口位置）</div>
                <img class="captcha-bg" src="''' + background + '''" width="''' + str(bg_width) + '''" height="''' + str(bg_height) + '''" alt="验证码背景">
                <img class="captcha-piece" id="captchaPiece" src="''' + slider + '''" width="''' + str(slider_size) + '''" height="''' + str(slider_size) + '''" style="left: ''' + str(slider_start_x) + '''px; top: ''' + str(slider_start_y) + '''px" alt="可移动滑块">
            </div>
        </div>

        <div class="slider-preview">
            <div class="slider-img-container">
                <img class="slider-img" src="''' + slider + '''" width="''' + str(slider_size) + '''" height="''' + str(slider_size) + '''" alt="滑块">
            </div>
            <div class="slider-info">
                <strong>👆 这是滑块</strong>
                需要拖动到背景图中缺口的位置
            </div>
        </div>

        <div class="slider-container" id="sliderContainer">
            <div class="slider-track" id="sliderTrack"></div>
            <div class="slider-thumb" id="sliderThumb">⇔</div>
        </div>
        <div class="slider-hint">← 拖动滑块到对应位置 →</div>

        <div class="position-display">
            当前位置: <span id="positionValue">0</span> / ''' + str(bg_width) + '''
        </div>

        <div class="instructions">
            <strong>📋 操作步骤：</strong>
            <ol>
                <li>观察上方背景图片中的<strong>缺口位置</strong></li>
                <li>拖动下方滑块到<strong>对应的 X 坐标</strong></li>
                <li>点击「<strong>确认位置</strong>」按钮</li>
                <li>将显示的数字<strong>复制到命令行</strong></li>
            </ol>
        </div>

        <div class="result" id="result"></div>

        <button class="submit-btn" id="submitBtn" onclick="submitPosition()">✓ 确认位置</button>
    </div>

    <script>
        const sliderContainer = document.getElementById('sliderContainer');
        const sliderTrack = document.getElementById('sliderTrack');
        const sliderThumb = document.getElementById('sliderThumb');
        const captchaPiece = document.getElementById('captchaPiece');
        const positionValue = document.getElementById('positionValue');
        const result = document.getElementById('result');
        const submitBtn = document.getElementById('submitBtn');

        let isDragging = false;
        let currentPosition = 0;
        const maxX = ''' + str(bg_width) + ''';

        sliderThumb.addEventListener('mousedown', (e) => {
            isDragging = true;
            e.preventDefault();
        });

        document.addEventListener('mousemove', (e) => {
            if (!isDragging) return;
            updatePosition(e.clientX);
        });

        document.addEventListener('mouseup', () => {
            isDragging = false;
        });

        sliderThumb.addEventListener('touchstart', (e) => {
            isDragging = true;
            e.preventDefault();
        });

        document.addEventListener('touchmove', (e) => {
            if (!isDragging) return;
            updatePosition(e.touches[0].clientX);
        });

        document.addEventListener('touchend', () => {
            isDragging = false;
        });

        sliderContainer.addEventListener('click', (e) => {
            if (e.target === sliderThumb) return;
            const rect = sliderContainer.getBoundingClientRect();
            const x = e.clientX - rect.left;
            const maxSliderX = rect.width - sliderThumb.offsetWidth;
            const sliderX = Math.min(Math.max(x - sliderThumb.offsetWidth / 2, 0), maxSliderX);
            currentPosition = Math.round((sliderX / maxSliderX) * maxX);
            updateSlider();
        });

        function updatePosition(clientX) {
            const rect = sliderContainer.getBoundingClientRect();
            let x = clientX - rect.left - sliderThumb.offsetWidth / 2;
            const maxSliderX = rect.width - sliderThumb.offsetWidth;
            x = Math.max(0, Math.min(x, maxSliderX));
            currentPosition = Math.round((x / maxSliderX) * maxX);
            updateSlider();
        }

        function updateSlider() {
            const maxSliderX = sliderContainer.clientWidth - sliderThumb.offsetWidth;
            const sliderX = (currentPosition / maxX) * maxSliderX;
            sliderThumb.style.left = sliderX + 'px';
            sliderTrack.style.width = (sliderX + sliderThumb.offsetWidth / 2) + 'px';
            captchaPiece.style.left = currentPosition + 'px';
            positionValue.textContent = currentPosition;
        }

        function submitPosition() {
            result.className = 'result show success';
            result.innerHTML = '<strong>✓ 验证码位置: ' + currentPosition + '</strong><br><br>' +
                '请在命令行中输入: <strong style="font-size: 20px; color: #1b5e20;">' + currentPosition + '</strong>' +
                '<br><br><span style="font-size: 12px;">（已自动复制到剪贴板）</span>';

            navigator.clipboard.writeText(currentPosition.toString()).then(() => {
                console.log('已复制到剪贴板');
            }).catch(() => {});

            submitBtn.disabled = true;
            submitBtn.textContent = '✓ 已确认';
        }

        updateSlider();
    </script>
</body>
</html>'''

        # 保存 HTML 文件
        temp_html = tempfile.NamedTemporaryFile(suffix=".html", delete=False, mode='w', encoding='utf-8')
        temp_html.write(html_head)
        temp_html.close()

        print(f"\n验证码 ID: {captcha_id}")
        print(f"验证页面已保存: {temp_html.name}")
        print(f"正在打开验证页面...")

        # 用浏览器打开
        webbrowser.open(f'file://{temp_html.name}')

        print(f"\n" + "="*50)
        print(f"请在浏览器中完成滑块验证：")
        print(f"1. 观察背景图片中的【缺口位置】")
        print(f"2. 拖动滑块到对应的 X 坐标")
        print(f"3. 点击「确认位置」按钮")
        print(f"4. 将显示的数字复制到下方")
        print(f"="*50)

    except Exception as e:
        print(f"警告: 无法创建验证页面: {e}")
        print(f"请手动查看 base64 数据:")
        print(f"  {background[:200]}...")

    captcha_x = input("\n请输入缺口绝对 X 坐标 (0-300): ")

    try:
        captcha_x = int(captcha_x)
    except ValueError:
        print("Error: 请输入有效的数字")
        sys.exit(1)

    # The API expects the distance dragged from the slider's starting point,
    # while the local page displays the gap's absolute X coordinate.
    captcha_x = _captcha_drag_distance(captcha_x, slider_start_x)

    # 登录
    result = client._request("POST", "/api/v1/auth/login", {
        "email": email,
        "password": password,
        "captcha_id": captcha_id,
        "captcha_x": captcha_x
    })

    if result.get("code") == 0:
        data = result.get("data", {})
        client.token = data.get("access_token")
        client._save_token(client.token, data.get("refresh_token"))
        print(f"✓ 登录成功: {email}")
    else:
        print(f"Error: 登录失败: {result.get('message')}")
        sys.exit(1)


def cmd_logout(args, root: Path):
    """Logout from YoudaoNoteLM."""
    client = get_client(args)
    client.logout()


def cmd_whoami(args, root: Path):
    """Show current logged in user."""
    client = get_client(args)
    if not client.token:
        print("Not logged in. Run 'youdaonotelm login' first.")
        sys.exit(1)
    result = client.get("/api/v1/user/profile")
    if result.get("code") == 0:
        user = result.get("data", {})
        print(f"User: {user.get('username', 'N/A')}")
        print(f"Email: {user.get('email', 'N/A')}")
        print(f"Role: {user.get('role', 'user')}")
    else:
        print(f"Error: {result.get('message')}")


def cmd_register(args, root: Path):
    """Register a new user."""
    client = get_client(args)
    email = args.email or input("Email: ")
    password = args.password or input("Password: ")
    username = args.username or input("Username: ")
    code = args.code or input("Verification code (from email): ")

    result = client.post("/api/v1/auth/register", {
        "email": email,
        "password": password,
        "username": username,
        "code": code
    })
    if result.get("code") == 0:
        print(f"✓ Registered successfully as {email}")
        print("  Run 'youdaonotelm login' to login.")
    else:
        print(f"Error: {result.get('message')}")


def cmd_send_code(args, root: Path):
    """Send verification code to email."""
    client = get_client(args)
    result = client.post("/api/v1/auth/send-code", {"email": args.email})
    if result.get("code") == 0:
        print(f"✓ Verification code sent to {args.email}")
    else:
        print(f"Error: {result.get('message')}")


def cmd_reset_password(args, root: Path):
    """Reset password with verification code."""
    client = get_client(args)
    result = client.post("/api/v1/auth/reset-password", {
        "email": args.email,
        "code": args.code,
        "new_password": args.new_password
    })
    if result.get("code") == 0:
        print("✓ Password reset successfully")
    else:
        print(f"Error: {result.get('message')}")


def cmd_refresh_token(args, root: Path):
    """Refresh access token."""
    client = get_client(args)
    if TOKEN_FILE.exists():
        data = json.loads(TOKEN_FILE.read_text())
        refresh_token = data.get("refresh_token")
        if not refresh_token:
            print("No refresh token found. Please login again.")
            sys.exit(1)
    else:
        print("Not logged in. Run 'youdaonotelm login' first.")
        sys.exit(1)

    result = client.post("/api/v1/auth/refresh", {"refresh_token": refresh_token})
    if result.get("code") == 0:
        data = result.get("data", {})
        client.token = data.get("access_token")
        client._save_token(client.token, data.get("refresh_token"))
        print("✓ Token refreshed successfully")
    else:
        print(f"Error: {result.get('message')}")


# ─── User Profile Commands ────────────────────────────────────────────────────

def cmd_profile(args, root: Path):
    """Manage user profile."""
    client = get_client(args)

    if args.subcmd == "get":
        result = client.get("/api/v1/user/profile")
        if result.get("code") == 0:
            user = result.get("data", {})
            print_json(user)
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "update":
        data = {}
        if args.username:
            data["username"] = args.username
        if args.email:
            data["email"] = args.email
        if not data:
            print("Nothing to update. Use --username or --email")
            sys.exit(1)
        result = client.put("/api/v1/user/profile", data)
        if result.get("code") == 0:
            print("✓ Profile updated successfully")
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "update-username":
        result = client.put("/api/v1/user/username", {"username": args.username})
        if result.get("code") == 0:
            print(f"✓ Username updated to '{args.username}'")
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "change-password":
        result = client.post("/api/v1/user/password", {
            "old_password": args.old_password,
            "new_password": args.new_password
        })
        if result.get("code") == 0:
            print("✓ Password changed successfully")
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "delete":
        if not args.yes:
            confirm = input("⚠️  Delete your account? This is IRREVERSIBLE! (y/N): ")
            if confirm.lower() != 'y':
                print("Cancelled.")
                return
        result = client.delete("/api/v1/user/account")
        if result.get("code") == 0:
            print("✓ Account deleted")
            client.logout()
        else:
            print(f"Error: {result.get('message')}")


# ─── User Config Commands ─────────────────────────────────────────────────────

def cmd_config_llm(args, root: Path):
    """Manage LLM configurations."""
    client = get_client(args)

    if args.subcmd == "list":
        result = client.get("/api/v1/user/config/llm")
        if result.get("code") == 0:
            configs = result.get("data", [])
            if not configs:
                print("No LLM configurations found.")
                return
            headers = ["ID", "Name", "Provider", "Model"]
            rows = [[c.get("id"), c.get("name"), c.get("provider"), c.get("model")] for c in configs]
            print_table(headers, rows)
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "create":
        data = {
            "name": args.name,
            "provider": args.provider,
            "api_key": args.api_key,
            "model": args.model
        }
        if args.base_url:
            data["base_url"] = args.base_url
        result = client.post("/api/v1/user/config/llm", data)
        if result.get("code") == 0:
            cfg = result.get("data", {})
            print(f"✓ Created LLM config (ID: {cfg.get('id')})")
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "update":
        data = {}
        if args.name:
            data["name"] = args.name
        if args.api_key:
            data["api_key"] = args.api_key
        if args.model:
            data["model"] = args.model
        if args.base_url:
            data["base_url"] = args.base_url
        result = client.put(f"/api/v1/user/config/llm/{args.id}", data)
        if result.get("code") == 0:
            print(f"✓ Updated LLM config {args.id}")
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "delete":
        result = client.delete(f"/api/v1/user/config/llm/{args.id}")
        if result.get("code") == 0:
            print(f"✓ Deleted LLM config {args.id}")
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "test":
        result = client.post("/api/v1/user/config/llm/test", {"provider": args.provider})
        if result.get("code") == 0:
            print("✓ LLM connection test passed")
        else:
            print(f"✗ Test failed: {result.get('message')}")


def cmd_config_search(args, root: Path):
    """Manage search configurations."""
    client = get_client(args)

    if args.subcmd == "list":
        result = client.get("/api/v1/user/config/search")
        if result.get("code") == 0:
            configs = result.get("data", [])
            if not configs:
                print("No search configurations found.")
                return
            headers = ["ID", "Name", "Provider"]
            rows = [[c.get("id"), c.get("name"), c.get("provider")] for c in configs]
            print_table(headers, rows)
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "create":
        data = {"name": args.name, "provider": args.provider, "api_key": args.api_key}
        if args.base_url:
            data["base_url"] = args.base_url
        result = client.post("/api/v1/user/config/search", data)
        if result.get("code") == 0:
            cfg = result.get("data", {})
            print(f"✓ Created search config (ID: {cfg.get('id')})")
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "update":
        data = {}
        if args.name:
            data["name"] = args.name
        if args.api_key:
            data["api_key"] = args.api_key
        result = client.put(f"/api/v1/user/config/search/{args.id}", data)
        if result.get("code") == 0:
            print(f"✓ Updated search config {args.id}")
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "delete":
        result = client.delete(f"/api/v1/user/config/search/{args.id}")
        if result.get("code") == 0:
            print(f"✓ Deleted search config {args.id}")
        else:
            print(f"Error: {result.get('message')}")


def cmd_config_embedding(args, root: Path):
    """Manage embedding configurations."""
    client = get_client(args)

    if args.subcmd == "list":
        result = client.get("/api/v1/user/config/embedding")
        if result.get("code") == 0:
            configs = result.get("data", [])
            if not configs:
                print("No embedding configurations found.")
                return
            headers = ["ID", "Name", "Provider", "Model"]
            rows = [[c.get("id"), c.get("name"), c.get("provider"), c.get("model")] for c in configs]
            print_table(headers, rows)
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "create":
        data = {
            "name": args.name,
            "provider": args.provider,
            "api_key": args.api_key,
            "model": args.model
        }
        if args.base_url:
            data["base_url"] = args.base_url
        result = client.post("/api/v1/user/config/embedding", data)
        if result.get("code") == 0:
            cfg = result.get("data", {})
            print(f"✓ Created embedding config (ID: {cfg.get('id')})")
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "update":
        data = {}
        if args.name:
            data["name"] = args.name
        if args.api_key:
            data["api_key"] = args.api_key
        if args.model:
            data["model"] = args.model
        result = client.put(f"/api/v1/user/config/embedding/{args.id}", data)
        if result.get("code") == 0:
            print(f"✓ Updated embedding config {args.id}")
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "delete":
        result = client.delete(f"/api/v1/user/config/embedding/{args.id}")
        if result.get("code") == 0:
            print(f"✓ Deleted embedding config {args.id}")
        else:
            print(f"Error: {result.get('message')}")


def cmd_config_asr(args, root: Path):
    """Manage ASR (speech recognition) configurations."""
    client = get_client(args)

    if args.subcmd == "list":
        result = client.get("/api/v1/user/config/asr")
        if result.get("code") == 0:
            configs = result.get("data", [])
            if not configs:
                print("No ASR configurations found.")
                return
            headers = ["ID", "Name", "Provider"]
            rows = [[c.get("id"), c.get("name"), c.get("provider")] for c in configs]
            print_table(headers, rows)
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "create":
        data = {"name": args.name, "provider": args.provider, "api_key": args.api_key}
        if args.base_url:
            data["base_url"] = args.base_url
        result = client.post("/api/v1/user/config/asr", data)
        if result.get("code") == 0:
            cfg = result.get("data", {})
            print(f"✓ Created ASR config (ID: {cfg.get('id')})")
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "update":
        data = {}
        if args.name:
            data["name"] = args.name
        if args.api_key:
            data["api_key"] = args.api_key
        result = client.put(f"/api/v1/user/config/asr/{args.id}", data)
        if result.get("code") == 0:
            print(f"✓ Updated ASR config {args.id}")
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "delete":
        result = client.delete(f"/api/v1/user/config/asr/{args.id}")
        if result.get("code") == 0:
            print(f"✓ Deleted ASR config {args.id}")
        else:
            print(f"Error: {result.get('message')}")


def cmd_config_reranker(args, root: Path):
    """Manage reranker configurations."""
    client = get_client(args)

    if args.subcmd == "list":
        result = client.get("/api/v1/user/config/reranker")
        if result.get("code") == 0:
            configs = result.get("data", [])
            if not configs:
                print("No reranker configurations found.")
                return
            headers = ["ID", "Name", "Provider", "Model"]
            rows = [[c.get("id"), c.get("name"), c.get("provider"), c.get("model")] for c in configs]
            print_table(headers, rows)
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "create":
        data = {
            "name": args.name,
            "provider": args.provider,
            "api_key": args.api_key,
            "model": args.model
        }
        if args.base_url:
            data["base_url"] = args.base_url
        result = client.post("/api/v1/user/config/reranker", data)
        if result.get("code") == 0:
            cfg = result.get("data", {})
            print(f"✓ Created reranker config (ID: {cfg.get('id')})")
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "update":
        data = {}
        if args.name:
            data["name"] = args.name
        if args.api_key:
            data["api_key"] = args.api_key
        if args.model:
            data["model"] = args.model
        result = client.put(f"/api/v1/user/config/reranker/{args.id}", data)
        if result.get("code") == 0:
            print(f"✓ Updated reranker config {args.id}")
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "delete":
        result = client.delete(f"/api/v1/user/config/reranker/{args.id}")
        if result.get("code") == 0:
            print(f"✓ Deleted reranker config {args.id}")
        else:
            print(f"Error: {result.get('message')}")


def cmd_config_active(args, root: Path):
    """Get active configuration for a service type."""
    client = get_client(args)
    result = client.get(f"/api/v1/user/config/active/{args.type}")
    if result.get("code") == 0:
        config = result.get("data", {})
        if config:
            print_json(config)
        else:
            print(f"No active {args.type} configuration found.")
    else:
        print(f"Error: {result.get('message')}")


def cmd_providers(args, root: Path):
    """List available providers."""
    client = get_client(args)

    if args.subcmd == "list":
        result = client.get("/api/v1/providers")
        if result.get("code") == 0:
            data = result.get("data", {})
            providers = data.get("providers", [])
            if not providers:
                print("No providers found.")
                return
            headers = ["Service Type", "Provider", "Display Name", "Implemented"]
            rows = [[p.get("service_type"), p.get("provider"), p.get("display_name"), "✓" if p.get("implemented") else "✗"] for p in providers]
            print_table(headers, rows)
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "active":
        result = client.get("/api/v1/providers/active")
        if result.get("code") == 0:
            active = result.get("data", {})
            print_json(active)
        else:
            print(f"Error: {result.get('message')}")


# ─── Notebook Commands ────────────────────────────────────────────────────────

def cmd_notebook(args, root: Path):
    """Manage notebooks."""
    client = get_client(args)

    if args.subcmd == "list":
        result = client.get("/api/v1/notebooks")
        if result.get("code") == 0:
            notebooks = result.get("data", [])
            if not notebooks:
                print("No notebooks found.")
                return
            headers = ["ID", "Name", "Created At"]
            rows = [[nb.get("id"), nb.get("name"), nb.get("created_at")] for nb in notebooks]
            print_table(headers, rows)
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "create":
        result = client.post("/api/v1/notebooks", {"name": args.name})
        if result.get("code") == 0:
            nb = result.get("data", {})
            print(f"✓ Created notebook: {nb.get('name')} (ID: {nb.get('id')})")
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "rename":
        result = client.put(f"/api/v1/notebooks/{args.id}", {"name": args.name})
        if result.get("code") == 0:
            print(f"✓ Renamed notebook {args.id} to '{args.name}'")
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "delete":
        if not args.yes:
            confirm = input(f"Delete notebook {args.id}? (y/N): ")
            if confirm.lower() != 'y':
                print("Cancelled.")
                return
        result = client.delete(f"/api/v1/notebooks/{args.id}")
        if result.get("code") == 0:
            print(f"✓ Deleted notebook {args.id}")
        else:
            print(f"Error: {result.get('message')}")


# ─── Source Commands ──────────────────────────────────────────────────────────

def cmd_source(args, root: Path):
    """Manage sources in a notebook."""
    client = get_client(args)

    if args.subcmd == "list":
        params = f"?page={args.page}&size={args.size}"
        if args.keyword:
            params += f"&keyword={args.keyword}"
        result = client.get(f"/api/v1/notebooks/{args.nb_id}/sources{params}")
        if result.get("code") == 0:
            data = result.get("data", {})
            sources = data.get("list", [])
            total = data.get("total", 0)
            print(f"Sources in notebook {args.nb_id} (total: {total}):")
            if not sources:
                print("  No sources found.")
                return
            headers = ["ID", "Name", "Type", "Status", "Created At"]
            rows = [[s.get("id"), s.get("name"), s.get("type"), s.get("status"), s.get("created_at")] for s in sources]
            print_table(headers, rows)
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "get":
        result = client.get(f"/api/v1/notebooks/{args.nb_id}/sources/{args.id}")
        if result.get("code") == 0:
            source = result.get("data", {})
            print_json(source)
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "content":
        result = client.get(f"/api/v1/notebooks/{args.nb_id}/sources/{args.id}/content")
        if result.get("code") == 0:
            content = result.get("data", {}).get("content", "")
            print(content)
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "original":
        result = client.get(f"/api/v1/notebooks/{args.nb_id}/sources/{args.id}/original")
        if result.get("code") == 0:
            data = result.get("data", {})
            print(f"Type: {data.get('type', 'N/A')}")
            print(f"Content:\n{data.get('content', '')}")
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "download":
        result = client.get(f"/api/v1/notebooks/{args.nb_id}/sources/{args.id}/download")
        if result.get("code") == 0:
            url = result.get("data", {}).get("url", "")
            print(f"Download URL: {url}")
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "rename":
        result = client.put(f"/api/v1/notebooks/{args.nb_id}/sources/{args.id}", {"name": args.name})
        if result.get("code") == 0:
            print(f"✓ Renamed source {args.id} to '{args.name}'")
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "delete":
        if not args.yes:
            confirm = input(f"Delete source {args.id}? (y/N): ")
            if confirm.lower() != 'y':
                print("Cancelled.")
                return
        result = client.delete(f"/api/v1/notebooks/{args.nb_id}/sources/{args.id}")
        if result.get("code") == 0:
            print(f"✓ Deleted source {args.id}")
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "batch-delete":
        ids = [int(x.strip()) for x in args.ids.split(",")]
        if not args.yes:
            confirm = input(f"Delete {len(ids)} sources? (y/N): ")
            if confirm.lower() != 'y':
                print("Cancelled.")
                return
        result = client.post(f"/api/v1/notebooks/{args.nb_id}/sources/batch-delete", {"ids": ids})
        if result.get("code") == 0:
            print(f"✓ Deleted {len(ids)} sources")
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "delete-failed":
        result = client.post(f"/api/v1/notebooks/{args.nb_id}/sources/delete-failed")
        if result.get("code") == 0:
            count = result.get("data", {}).get("deleted_count", 0)
            print(f"✓ Deleted {count} failed sources")
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "reimport-all":
        result = client.post("/api/v1/sources/reimport-all")
        if result.get("code") == 0:
            count = result.get("data", {}).get("reimported_count", 0)
            print(f"✓ Reimported {count} sources")
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "reimport-selected":
        ids = [int(x.strip()) for x in args.ids.split(",")]
        result = client.post("/api/v1/sources/reimport", {"source_ids": ids})
        if result.get("code") == 0:
            count = result.get("data", {}).get("reimported_count", 0)
            print(f"✓ Reimported {count} sources")
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "from-note":
        result = client.post(f"/api/v1/notebooks/{args.nb_id}/sources/from-note", {
            "title": args.title,
            "content": args.content
        })
        if result.get("code") == 0:
            source = result.get("data", {})
            print(f"✓ Created source from note: {source.get('name')} (ID: {source.get('id')})")
        else:
            print(f"Error: {result.get('message')}")


# ─── Chat Commands ────────────────────────────────────────────────────────────

def cmd_chat(args, root: Path):
    """Manage conversations and chat."""
    client = get_client(args)

    if args.subcmd == "list":
        result = client.get(f"/api/v1/chat/notebooks/{args.nb_id}/conversations")
        if result.get("code") == 0:
            convs = result.get("data", [])
            if not convs:
                print("No conversations found.")
                return
            headers = ["ID", "Title", "Created At"]
            rows = [[c.get("id"), c.get("title"), c.get("created_at")] for c in convs]
            print_table(headers, rows)
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "create":
        result = client.post("/api/v1/chat/conversations", {
            "notebook_id": args.nb_id,
            "title": args.title or "New Conversation"
        })
        if result.get("code") == 0:
            conv_id = result.get("data", {}).get("id")
            print(f"✓ Created conversation (ID: {conv_id})")
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "get":
        result = client.get(f"/api/v1/chat/conversations/{args.id}")
        if result.get("code") == 0:
            conv = result.get("data", {})
            print_json(conv)
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "delete":
        if not args.yes:
            confirm = input(f"Delete conversation {args.id}? (y/N): ")
            if confirm.lower() != 'y':
                print("Cancelled.")
                return
        result = client.delete(f"/api/v1/chat/conversations/{args.id}")
        if result.get("code") == 0:
            print(f"✓ Deleted conversation {args.id}")
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "messages":
        result = client.get(f"/api/v1/chat/conversations/{args.id}/messages")
        if result.get("code") == 0:
            msgs = result.get("data", [])
            if not msgs:
                print("No messages found.")
                return
            for msg in msgs:
                role = msg.get("role", "unknown")
                content = msg.get("content", "")
                prefix = "🤖" if role == "assistant" else "👤"
                print(f"{prefix} [{role}]: {content[:200]}{'...' if len(content) > 200 else ''}")
                print()
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "send":
        # Note: This uses SSE streaming, which is complex in CLI
        # For now, we'll send and get the response
        result = client.post(f"/api/v1/chat/conversations/{args.id}/messages", {
            "content": args.message,
            "notebook_id": args.nb_id
        })
        if result.get("code") == 0:
            print("✓ Message sent (streaming response not shown in CLI)")
        else:
            print(f"Error: {result.get('message')}")


# ─── Generation Commands ──────────────────────────────────────────────────────

def cmd_generation(args, root: Path):
    """Manage content generation tasks."""
    client = get_client(args)

    if args.subcmd == "submit":
        result = client.post("/api/v1/generations", {
            "notebook_id": args.nb_id,
            "type": args.type,
            "source_ids": args.source_ids or [],
            "prompt": args.prompt or ""
        })
        if result.get("code") == 0:
            task = result.get("data", {})
            print(f"✓ Submitted generation task (ID: {task.get('id')})")
            print(f"  Type: {args.type}")
            print(f"  Status: {task.get('status')}")
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "list":
        params = f"?limit={args.limit}"
        if args.nb_id:
            params += f"&notebook_id={args.nb_id}"
        result = client.get(f"/api/v1/generations/tasks{params}")
        if result.get("code") == 0:
            tasks = result.get("data", [])
            if not tasks:
                print("No generation tasks found.")
                return
            headers = ["ID", "Type", "Status", "Created At"]
            rows = [[t.get("id"), t.get("type"), t.get("status"), t.get("created_at")] for t in tasks]
            print_table(headers, rows)
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "get":
        result = client.get(f"/api/v1/generations/tasks/{args.id}")
        if result.get("code") == 0:
            task = result.get("data", {})
            print_json(task)
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "delete":
        if not args.yes:
            confirm = input(f"Delete generation task {args.id}? (y/N): ")
            if confirm.lower() != 'y':
                print("Cancelled.")
                return
        result = client.delete(f"/api/v1/generations/tasks/{args.id}")
        if result.get("code") == 0:
            print(f"✓ Deleted generation task {args.id}")
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "export":
        print("Export generates a file download.")
        print(f"Use: curl -X POST {client.base_url}/api/v1/generations/export \\")
        print(f"  -H 'Authorization: Bearer <token>' \\")
        print(f"  -H 'Content-Type: application/json' \\")
        print(f"  -d '{{\"type\":\"{args.type}\",\"content\":\"...\",\"title\":\"{args.title}\"}}' \\")
        print(f"  -o {args.type}_export.html")


# ─── Youdao Commands ──────────────────────────────────────────────────────────

def cmd_youdao(args, root: Path):
    """Manage Youdao Cloud Notes integration."""
    client = get_client(args)

    if args.subcmd == "bind":
        result = client.post("/api/v1/youdao/bind", {"api_key": args.api_key})
        if result.get("code") == 0:
            print("✓ Youdao account bound successfully")
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "unbind":
        if not args.yes:
            confirm = input("Unbind Youdao account? (y/N): ")
            if confirm.lower() != 'y':
                print("Cancelled.")
                return
        result = client.delete("/api/v1/youdao/bind")
        if result.get("code") == 0:
            print("✓ Youdao account unbound")
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "status":
        result = client.get("/api/v1/youdao/bind")
        if result.get("code") == 0:
            data = result.get("data", {})
            print(f"Bound: {data.get('bound', False)}")
            print(f"Status: {data.get('status', 'N/A')}")
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "notes":
        folder_id = args.folder_id or ""
        result = client.get(f"/api/v1/youdao/notes?folderId={folder_id}")
        if result.get("code") == 0:
            items = result.get("data", [])
            if not items:
                print("No notes found.")
                return
            headers = ["ID", "Name", "Type"]
            rows = [[item.get("id"), item.get("name"), item.get("type")] for item in items]
            print_table(headers, rows)
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "import":
        result = client.post("/api/v1/youdao/import", {
            "notebook_id": args.nb_id,
            "file_id": args.file_id
        })
        if result.get("code") == 0:
            source = result.get("data", {})
            print(f"✓ Imported note: {source.get('name')} (ID: {source.get('id')})")
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "import-batch":
        file_ids = args.file_ids.split(",") if args.file_ids else []
        if not file_ids:
            print("Error: --file-ids is required (comma-separated)")
            sys.exit(1)
        result = client.post("/api/v1/youdao/import/batch", {
            "notebook_id": args.nb_id,
            "file_ids": file_ids,
            "file_names": []
        })
        if result.get("code") == 0:
            data = result.get("data", {})
            print(f"✓ Batch import started (Task ID: {data.get('task_id')})")
            print(f"  Source IDs: {data.get('source_ids')}")
        else:
            print(f"Error: {result.get('message')}")


# ─── Import Commands ──────────────────────────────────────────────────────────

def cmd_import(args, root: Path):
    """Import files into notebook."""
    client = get_client(args)

    if args.subcmd == "file":
        # File upload requires multipart/form-data - use curl as fallback
        print("File upload requires multipart/form-data.")
        print(f"Use: curl -X POST {client.base_url}/api/v1/notebooks/{args.nb_id}/import/file \\")
        print(f"  -H 'Authorization: Bearer <token>' \\")
        print(f"  -F 'file=@{args.file}'")

    elif args.subcmd == "url":
        result = client.post(f"/api/v1/notebooks/{args.nb_id}/search/url", {
            "url": args.url
        })
        if result.get("code") == 0:
            data = result.get("data", {})
            print(f"✓ URL import started (Task ID: {data.get('task_id')})")
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "task":
        result = client.get(f"/api/v1/import/tasks/{args.task_id}")
        if result.get("code") == 0:
            task = result.get("data", {})
            print_json(task)
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "delete-task":
        result = client.delete(f"/api/v1/import/tasks/{args.task_id}")
        if result.get("code") == 0:
            print(f"✓ Deleted import task {args.task_id}")
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "audio-preview":
        print("Audio preview requires multipart/form-data file upload.")
        print(f"Use: curl -X POST {client.base_url}/api/v1/notebooks/{args.nb_id}/import/audio/preview \\")
        print(f"  -H 'Authorization: Bearer <token>' \\")
        print(f"  -F 'file=@{args.file}'")

    elif args.subcmd == "audio-status":
        result = client.get(f"/api/v1/import/audio/preview/{args.preview_id}")
        if result.get("code") == 0:
            preview = result.get("data", {})
            print_json(preview)
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "audio-confirm":
        result = client.post("/api/v1/import/audio/confirm", {
            "preview_id": args.preview_id,
            "content": args.content
        })
        if result.get("code") == 0:
            source = result.get("data", {})
            print(f"✓ Audio imported as source: {source.get('name')} (ID: {source.get('id')})")
        else:
            print(f"Error: {result.get('message')}")


# ─── Search Commands ──────────────────────────────────────────────────────────

def cmd_search(args, root: Path):
    """Search in notebook."""
    client = get_client(args)

    if args.subcmd == "query":
        result = client.post(f"/api/v1/notebooks/{args.nb_id}/search", {
            "query": args.query
        })
        if result.get("code") == 0:
            data = result.get("data", {})
            print_json(data)
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "import-results":
        urls = [u.strip() for u in args.urls.split(",")] if args.urls else []
        if not urls:
            print("Error: --urls is required (comma-separated)")
            sys.exit(1)
        items = [{"url": u} for u in urls]
        result = client.post(f"/api/v1/notebooks/{args.nb_id}/search/import", {"items": items})
        if result.get("code") == 0:
            data = result.get("data", {})
            print(f"✓ Import started (Task ID: {data.get('task_id')})")
            print(f"  Source IDs: {data.get('source_ids')}")
        else:
            print(f"Error: {result.get('message')}")


# ─── Memory Commands ──────────────────────────────────────────────────────────

def cmd_memory(args, root: Path):
    """Manage user preferences/memory."""
    client = get_client(args)

    if args.subcmd == "list":
        result = client.get("/api/v1/user/memories")
        if result.get("code") == 0:
            memories = result.get("data", [])
            if not memories:
                print("No preferences saved.")
                return
            headers = ["Type", "Content", "Updated At"]
            rows = [[m.get("type"), m.get("content", "")[:50], m.get("updated_at")] for m in memories]
            print_table(headers, rows)
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "set":
        result = client.put(f"/api/v1/user/memories/{args.type}", {"content": args.content})
        if result.get("code") == 0:
            print(f"✓ Set preference: {args.type}")
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "delete":
        result = client.delete(f"/api/v1/user/memories/{args.type}")
        if result.get("code") == 0:
            print(f"✓ Deleted preference: {args.type}")
        else:
            print(f"Error: {result.get('message')}")


# ─── Feedback Commands ────────────────────────────────────────────────────────

def cmd_feedback(args, root: Path):
    """Manage answer feedback."""
    client = get_client(args)

    if args.subcmd == "up":
        result = client.put(f"/api/v1/chat/messages/{args.message_id}/feedback", {
            "rating": "up",
            "reason": args.reason or "helpful"
        })
        if result.get("code") == 0:
            print(f"✓ Liked message {args.message_id}")
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "down":
        result = client.put(f"/api/v1/chat/messages/{args.message_id}/feedback", {
            "rating": "down",
            "reason": args.reason or "not_helpful"
        })
        if result.get("code") == 0:
            print(f"✓ Disliked message {args.message_id}")
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "delete":
        result = client.delete(f"/api/v1/chat/messages/{args.message_id}/feedback")
        if result.get("code") == 0:
            print(f"✓ Deleted feedback for message {args.message_id}")
        else:
            print(f"Error: {result.get('message')}")


# ─── Admin Commands ───────────────────────────────────────────────────────────

def cmd_admin(args, root: Path):
    """Admin operations."""
    client = get_client(args)

    if args.subcmd == "users":
        result = client.get("/api/v1/admin/users")
        if result.get("code") == 0:
            data = result.get("data", {})
            users = data.get("list", [])
            if not users:
                print("No users found.")
                return
            headers = ["ID", "Username", "Email", "Role", "Status", "Enabled"]
            rows = [[u.get("id"), u.get("username"), u.get("email"), u.get("role"), "✓" if u.get("status") == 1 else "✗", "✓" if u.get("enabled") else "✗"] for u in users]
            print_table(headers, rows)
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "update-user-status":
        result = client.put(f"/api/v1/admin/users/{args.user_id}/status", {"status": args.status})
        if result.get("code") == 0:
            print(f"✓ User {args.user_id} status updated to '{args.status}'")
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "config-status":
        result = client.get("/api/v1/admin/config/status")
        if result.get("code") == 0:
            data = result.get("data", {})
            print_json(data)
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "config-get":
        result = client.get(f"/api/v1/admin/config/{args.group}")
        if result.get("code") == 0:
            configs = result.get("data", [])
            if not configs:
                print(f"No configurations found in group '{args.group}'.")
                return
            headers = ["Key", "Value", "Updated At"]
            rows = [[c.get("key"), c.get("value", "")[:50], c.get("updated_at")] for c in configs]
            print_table(headers, rows)
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "config-add":
        result = client.post(f"/api/v1/admin/config/{args.group}", {
            "key": args.key,
            "value": args.value
        })
        if result.get("code") == 0:
            print(f"✓ Added config: {args.group}/{args.key}")
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "config-update":
        result = client.put(f"/api/v1/admin/config/{args.group}/{args.key}", {"value": args.value})
        if result.get("code") == 0:
            print(f"✓ Updated config: {args.group}/{args.key}")
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "config-delete":
        result = client.delete(f"/api/v1/admin/config/{args.group}/{args.key}")
        if result.get("code") == 0:
            print(f"✓ Deleted config: {args.group}/{args.key}")
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "feedback-overview":
        params = f"?from={args.from_date}&to={args.to_date}"
        result = client.get(f"/api/v1/admin/feedback/overview{params}")
        if result.get("code") == 0:
            data = result.get("data", {})
            print_json(data)
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "feedback-list":
        params = f"?from={args.from_date}&to={args.to_date}&page={args.page}&size={args.size}"
        if args.rating:
            params += f"&rating={args.rating}"
        if args.reason:
            params += f"&reason={args.reason}"
        result = client.get(f"/api/v1/admin/feedback{params}")
        if result.get("code") == 0:
            data = result.get("data", {})
            print_json(data)
        else:
            print(f"Error: {result.get('message')}")

    elif args.subcmd == "feedback-export":
        print(f"Exporting feedback CSV...")
        print(f"Use: curl -X GET '{client.base_url}/api/v1/admin/feedback/export.csv?from={args.from_date}&to={args.to_date}' \\")
        print(f"  -H 'Authorization: Bearer <token>' \\")
        print(f"  -o feedback.csv")

    elif args.subcmd == "mysql":
        print("Connecting to MySQL...")
        docker_compose(root, ["exec", "mysql", "mysql", "-uroot", "-p"])


# ─── Infra Commands ───────────────────────────────────────────────────────────

def cmd_server(args, root: Path):
    """Manage the Go backend server."""
    if args.subcmd == "run":
        print(f"Starting YoudaoNoteLM server on port {args.port}...")
        run_cmd(["go", "run", "./cmd/server"], cwd=root)

    elif args.subcmd == "build":
        print("Building server binary...")
        output = args.output or "bin/server"
        run_cmd(["go", "build", "-o", output, "./cmd/server"], cwd=root)
        print(f"Binary built: {output}")

    elif args.subcmd == "status":
        port = args.port or DEFAULT_PORT
        if check_health(port):
            print(f"✓ Server is healthy (port {port})")
        else:
            print(f"✗ Server is not responding (port {port})")
            sys.exit(1)


def cmd_docker(args, root: Path):
    """Manage Docker Compose services."""
    if args.subcmd == "up":
        print("Starting all services...")
        cmd = ["up", "-d"]
        if args.build:
            cmd.append("--build")
        if args.pull:
            cmd.extend(["--pull", "always"])
        if args.services:
            cmd.extend(args.services)
        docker_compose(root, cmd)
        print("Services started.")

    elif args.subcmd == "down":
        print("Stopping all services...")
        cmd = ["down"]
        if args.volumes:
            cmd.append("-v")
            print("⚠️  Warning: This will delete all data volumes!")
        docker_compose(root, cmd)

    elif args.subcmd == "restart":
        cmd = ["restart"]
        if args.services:
            cmd.extend(args.services)
        docker_compose(root, cmd)

    elif args.subcmd == "logs":
        cmd = ["logs", "-f"]
        if args.tail:
            cmd.extend(["--tail", str(args.tail)])
        if args.services:
            cmd.extend(args.services)
        docker_compose(root, cmd)

    elif args.subcmd == "ps":
        docker_compose(root, ["ps"])

    elif args.subcmd == "pull":
        docker_compose(root, ["pull"])

    elif args.subcmd == "build":
        docker_compose(root, ["build"])


def cmd_config(args, root: Path):
    """Manage configuration files."""
    config_dir = root / "configs"

    if args.subcmd == "show":
        config_file = config_dir / (args.file or "config.yaml")
        if not config_file.exists():
            print(f"Config file not found: {config_file}")
            sys.exit(1)
        print(config_file.read_text())

    elif args.subcmd == "validate":
        config_file = config_dir / (args.file or "config.yaml")
        if not config_file.exists():
            print(f"✗ Config file not found: {config_file}")
            sys.exit(1)
        try:
            import yaml
            with open(config_file) as f:
                yaml.safe_load(f)
            print(f"✓ {config_file.name} is valid YAML")
        except Exception as e:
            print(f"✗ Invalid YAML: {e}")
            sys.exit(1)


def cmd_health(args, root: Path):
    """Check application health."""
    port = args.port or DEFAULT_PORT
    if check_health(port):
        print("✓ Application is healthy")
    else:
        print("✗ Application is not responding")
        sys.exit(1)


def cmd_version(args, root: Path):
    """Show version information."""
    print(f"{APP_NAME} CLI v{VERSION}")


# ─── Argument Parser ──────────────────────────────────────────────────────────

def build_parser() -> argparse.ArgumentParser:
    """Build the CLI argument parser."""
    parser = argparse.ArgumentParser(
        prog=APP_NAME,
        description="YoudaoNoteLM CLI - RAG-based Youdao Cloud Notes Knowledge Q&A System",
        formatter_class=argparse.RawDescriptionHelpFormatter
    )
    parser.add_argument("--version", action="version", version=f"{APP_NAME} CLI v{VERSION}")
    parser.add_argument("--base-url", default=DEFAULT_BASE_URL, help="API base URL")
    parser.add_argument("-C", "--chdir", help="Change to directory before executing")

    subparsers = parser.add_subparsers(dest="command", help="Available commands")

    # ── Auth ──
    login_parser = subparsers.add_parser("login", help="Login to YoudaoNoteLM")
    login_parser.add_argument("email", nargs="?", help="Email address")
    login_parser.add_argument("password", nargs="?", help="Password")

    subparsers.add_parser("logout", help="Logout from YoudaoNoteLM")
    subparsers.add_parser("whoami", help="Show current user")

    reg_parser = subparsers.add_parser("register", help="Register new user")
    reg_parser.add_argument("email", nargs="?", help="Email address")
    reg_parser.add_argument("password", nargs="?", help="Password")
    reg_parser.add_argument("username", nargs="?", help="Username")
    reg_parser.add_argument("code", nargs="?", help="Verification code")

    send_code = subparsers.add_parser("send-code", help="Send verification code")
    send_code.add_argument("email", help="Email address")

    reset_pw = subparsers.add_parser("reset-password", help="Reset password")
    reset_pw.add_argument("email", help="Email address")
    reset_pw.add_argument("code", help="Verification code")
    reset_pw.add_argument("new_password", help="New password")

    subparsers.add_parser("refresh", help="Refresh access token")

    # ── Profile ──
    profile_parser = subparsers.add_parser("profile", help="Manage user profile")
    profile_sub = profile_parser.add_subparsers(dest="subcmd")

    profile_sub.add_parser("get", help="Get profile details")

    profile_update = profile_sub.add_parser("update", help="Update profile")
    profile_update.add_argument("--username", help="New username")
    profile_update.add_argument("--email", help="New email")

    profile_username = profile_sub.add_parser("update-username", help="Update username")
    profile_username.add_argument("username", help="New username")

    profile_pw = profile_sub.add_parser("change-password", help="Change password")
    profile_pw.add_argument("old_password", help="Current password")
    profile_pw.add_argument("new_password", help="New password")

    profile_del = profile_sub.add_parser("delete", help="Delete account")
    profile_del.add_argument("-y", "--yes", action="store_true", help="Skip confirmation")

    # ── Config (User) ──
    cfg_parser = subparsers.add_parser("config-llm", help="Manage LLM configs")
    cfg_sub = cfg_parser.add_subparsers(dest="subcmd")
    cfg_sub.add_parser("list", help="List LLM configs")
    cfg_create = cfg_sub.add_parser("create", help="Create LLM config")
    cfg_create.add_argument("name", help="Config name")
    cfg_create.add_argument("provider", help="Provider name")
    cfg_create.add_argument("api_key", help="API key")
    cfg_create.add_argument("model", help="Model name")
    cfg_create.add_argument("--base-url", help="Base URL (optional)")
    cfg_update = cfg_sub.add_parser("update", help="Update LLM config")
    cfg_update.add_argument("id", type=int, help="Config ID")
    cfg_update.add_argument("--name", help="New name")
    cfg_update.add_argument("--api-key", help="New API key")
    cfg_update.add_argument("--model", help="New model")
    cfg_update.add_argument("--base-url", help="New base URL")
    cfg_del = cfg_sub.add_parser("delete", help="Delete LLM config")
    cfg_del.add_argument("id", type=int, help="Config ID")
    cfg_test = cfg_sub.add_parser("test", help="Test LLM connection")
    cfg_test.add_argument("provider", help="Provider name")

    cfg_search_parser = subparsers.add_parser("config-search", help="Manage search configs")
    cfg_search_sub = cfg_search_parser.add_subparsers(dest="subcmd")
    cfg_search_sub.add_parser("list", help="List search configs")
    cfg_search_create = cfg_search_sub.add_parser("create", help="Create search config")
    cfg_search_create.add_argument("name", help="Config name")
    cfg_search_create.add_argument("provider", help="Provider name")
    cfg_search_create.add_argument("api_key", help="API key")
    cfg_search_create.add_argument("--base-url", help="Base URL (optional)")
    cfg_search_update = cfg_search_sub.add_parser("update", help="Update search config")
    cfg_search_update.add_argument("id", type=int, help="Config ID")
    cfg_search_update.add_argument("--name", help="New name")
    cfg_search_update.add_argument("--api-key", help="New API key")
    cfg_search_del = cfg_search_sub.add_parser("delete", help="Delete search config")
    cfg_search_del.add_argument("id", type=int, help="Config ID")

    cfg_emb_parser = subparsers.add_parser("config-embedding", help="Manage embedding configs")
    cfg_emb_sub = cfg_emb_parser.add_subparsers(dest="subcmd")
    cfg_emb_sub.add_parser("list", help="List embedding configs")
    cfg_emb_create = cfg_emb_sub.add_parser("create", help="Create embedding config")
    cfg_emb_create.add_argument("name", help="Config name")
    cfg_emb_create.add_argument("provider", help="Provider name")
    cfg_emb_create.add_argument("api_key", help="API key")
    cfg_emb_create.add_argument("model", help="Model name")
    cfg_emb_create.add_argument("--base-url", help="Base URL (optional)")
    cfg_emb_update = cfg_emb_sub.add_parser("update", help="Update embedding config")
    cfg_emb_update.add_argument("id", type=int, help="Config ID")
    cfg_emb_update.add_argument("--name", help="New name")
    cfg_emb_update.add_argument("--api-key", help="New API key")
    cfg_emb_update.add_argument("--model", help="New model")
    cfg_emb_del = cfg_emb_sub.add_parser("delete", help="Delete embedding config")
    cfg_emb_del.add_argument("id", type=int, help="Config ID")

    cfg_asr_parser = subparsers.add_parser("config-asr", help="Manage ASR configs")
    cfg_asr_sub = cfg_asr_parser.add_subparsers(dest="subcmd")
    cfg_asr_sub.add_parser("list", help="List ASR configs")
    cfg_asr_create = cfg_asr_sub.add_parser("create", help="Create ASR config")
    cfg_asr_create.add_argument("name", help="Config name")
    cfg_asr_create.add_argument("provider", help="Provider name")
    cfg_asr_create.add_argument("api_key", help="API key")
    cfg_asr_create.add_argument("--base-url", help="Base URL (optional)")
    cfg_asr_update = cfg_asr_sub.add_parser("update", help="Update ASR config")
    cfg_asr_update.add_argument("id", type=int, help="Config ID")
    cfg_asr_update.add_argument("--name", help="New name")
    cfg_asr_update.add_argument("--api-key", help="New API key")
    cfg_asr_del = cfg_asr_sub.add_parser("delete", help="Delete ASR config")
    cfg_asr_del.add_argument("id", type=int, help="Config ID")

    cfg_rr_parser = subparsers.add_parser("config-reranker", help="Manage reranker configs")
    cfg_rr_sub = cfg_rr_parser.add_subparsers(dest="subcmd")
    cfg_rr_sub.add_parser("list", help="List reranker configs")
    cfg_rr_create = cfg_rr_sub.add_parser("create", help="Create reranker config")
    cfg_rr_create.add_argument("name", help="Config name")
    cfg_rr_create.add_argument("provider", help="Provider name")
    cfg_rr_create.add_argument("api_key", help="API key")
    cfg_rr_create.add_argument("model", help="Model name")
    cfg_rr_create.add_argument("--base-url", help="Base URL (optional)")
    cfg_rr_update = cfg_rr_sub.add_parser("update", help="Update reranker config")
    cfg_rr_update.add_argument("id", type=int, help="Config ID")
    cfg_rr_update.add_argument("--name", help="New name")
    cfg_rr_update.add_argument("--api-key", help="New API key")
    cfg_rr_update.add_argument("--model", help="New model")
    cfg_rr_del = cfg_rr_sub.add_parser("delete", help="Delete reranker config")
    cfg_rr_del.add_argument("id", type=int, help="Config ID")

    cfg_active = subparsers.add_parser("config-active", help="Get active config")
    cfg_active.add_argument("type", choices=["llm", "search", "asr", "embedding", "reranker"], help="Config type")

    # ── Providers ──
    prov_parser = subparsers.add_parser("providers", help="List providers")
    prov_sub = prov_parser.add_subparsers(dest="subcmd")
    prov_sub.add_parser("list", help="List all providers")
    prov_sub.add_parser("active", help="Get active providers")

    # ── Notebook ──
    nb_parser = subparsers.add_parser("notebook", help="Manage notebooks")
    nb_sub = nb_parser.add_subparsers(dest="subcmd")

    nb_sub.add_parser("list", help="List all notebooks")

    nb_create = nb_sub.add_parser("create", help="Create notebook")
    nb_create.add_argument("name", help="Notebook name")

    nb_rename = nb_sub.add_parser("rename", help="Rename notebook")
    nb_rename.add_argument("id", type=int, help="Notebook ID")
    nb_rename.add_argument("name", help="New name")

    nb_delete = nb_sub.add_parser("delete", help="Delete notebook")
    nb_delete.add_argument("id", type=int, help="Notebook ID")
    nb_delete.add_argument("-y", "--yes", action="store_true", help="Skip confirmation")

    # ── Source ──
    src_parser = subparsers.add_parser("source", help="Manage sources")
    src_sub = src_parser.add_subparsers(dest="subcmd")

    src_list = src_sub.add_parser("list", help="List sources in notebook")
    src_list.add_argument("nb_id", type=int, help="Notebook ID")
    src_list.add_argument("-k", "--keyword", help="Search keyword")
    src_list.add_argument("-p", "--page", type=int, default=1, help="Page number")
    src_list.add_argument("-s", "--size", type=int, default=10, help="Page size")

    src_get = src_sub.add_parser("get", help="Get source details")
    src_get.add_argument("nb_id", type=int, help="Notebook ID")
    src_get.add_argument("id", type=int, help="Source ID")

    src_content = src_sub.add_parser("content", help="Get source content")
    src_content.add_argument("nb_id", type=int, help="Notebook ID")
    src_content.add_argument("id", type=int, help="Source ID")

    src_rename = src_sub.add_parser("rename", help="Rename source")
    src_rename.add_argument("nb_id", type=int, help="Notebook ID")
    src_rename.add_argument("id", type=int, help="Source ID")
    src_rename.add_argument("name", help="New name")

    src_delete = src_sub.add_parser("delete", help="Delete source")
    src_delete.add_argument("nb_id", type=int, help="Notebook ID")
    src_delete.add_argument("id", type=int, help="Source ID")
    src_delete.add_argument("-y", "--yes", action="store_true", help="Skip confirmation")

    src_batch_del = src_sub.add_parser("batch-delete", help="Batch delete sources")
    src_batch_del.add_argument("nb_id", type=int, help="Notebook ID")
    src_batch_del.add_argument("ids", help="Comma-separated source IDs")
    src_batch_del.add_argument("-y", "--yes", action="store_true", help="Skip confirmation")

    src_del_failed = src_sub.add_parser("delete-failed", help="Delete failed sources")
    src_del_failed.add_argument("nb_id", type=int, help="Notebook ID")

    src_original = src_sub.add_parser("original", help="Get original format content")
    src_original.add_argument("nb_id", type=int, help="Notebook ID")
    src_original.add_argument("id", type=int, help="Source ID")

    src_download = src_sub.add_parser("download", help="Get download URL")
    src_download.add_argument("nb_id", type=int, help="Notebook ID")
    src_download.add_argument("id", type=int, help="Source ID")

    src_reimport_sel = src_sub.add_parser("reimport-selected", help="Reimport selected sources")
    src_reimport_sel.add_argument("ids", help="Comma-separated source IDs")

    src_from_note = src_sub.add_parser("from-note", help="Create source from note")
    src_from_note.add_argument("nb_id", type=int, help="Notebook ID")
    src_from_note.add_argument("title", help="Note title")
    src_from_note.add_argument("content", help="Note content")

    src_sub.add_parser("reimport-all", help="Reimport all unvectorized sources")

    # ── Chat ──
    chat_parser = subparsers.add_parser("chat", help="Manage conversations")
    chat_sub = chat_parser.add_subparsers(dest="subcmd")

    chat_list = chat_sub.add_parser("list", help="List conversations in notebook")
    chat_list.add_argument("nb_id", type=int, help="Notebook ID")

    chat_create = chat_sub.add_parser("create", help="Create conversation")
    chat_create.add_argument("nb_id", type=int, help="Notebook ID")
    chat_create.add_argument("-t", "--title", help="Conversation title")

    chat_get = chat_sub.add_parser("get", help="Get conversation details")
    chat_get.add_argument("id", type=int, help="Conversation ID")

    chat_delete = chat_sub.add_parser("delete", help="Delete conversation")
    chat_delete.add_argument("id", type=int, help="Conversation ID")
    chat_delete.add_argument("-y", "--yes", action="store_true", help="Skip confirmation")

    chat_msgs = chat_sub.add_parser("messages", help="Get conversation messages")
    chat_msgs.add_argument("id", type=int, help="Conversation ID")

    chat_send = chat_sub.add_parser("send", help="Send message (streaming)")
    chat_send.add_argument("id", type=int, help="Conversation ID")
    chat_send.add_argument("nb_id", type=int, help="Notebook ID")
    chat_send.add_argument("message", help="Message content")

    # ── Generation ──
    gen_parser = subparsers.add_parser("generation", help="Manage content generation")
    gen_sub = gen_parser.add_subparsers(dest="subcmd")

    gen_submit = gen_sub.add_parser("submit", help="Submit generation task")
    gen_submit.add_argument("nb_id", type=int, help="Notebook ID")
    gen_submit.add_argument("type", choices=["mindmap", "ppt", "quiz", "note"], help="Generation type")
    gen_submit.add_argument("--source-ids", nargs="*", type=int, help="Source IDs")
    gen_submit.add_argument("--prompt", help="Custom prompt")

    gen_list = gen_sub.add_parser("list", help="List generation tasks")
    gen_list.add_argument("--nb-id", type=int, help="Filter by notebook")
    gen_list.add_argument("--limit", type=int, default=100, help="Max results")

    gen_get = gen_sub.add_parser("get", help="Get task details")
    gen_get.add_argument("id", help="Task ID")

    gen_delete = gen_sub.add_parser("delete", help="Delete task")
    gen_delete.add_argument("id", help="Task ID")
    gen_delete.add_argument("-y", "--yes", action="store_true", help="Skip confirmation")

    gen_export = gen_sub.add_parser("export", help="Export generated content")
    gen_export.add_argument("type", choices=["mindmap", "ppt", "quiz", "note"], help="Content type")
    gen_export.add_argument("title", help="Export title")

    # ── Youdao ──
    yd_parser = subparsers.add_parser("youdao", help="Youdao Cloud Notes integration")
    yd_sub = yd_parser.add_subparsers(dest="subcmd")

    yd_bind = yd_sub.add_parser("bind", help="Bind Youdao account")
    yd_bind.add_argument("api_key", help="Youdao API Key")

    yd_sub.add_parser("unbind", help="Unbind Youdao account")
    yd_sub.add_parser("status", help="Check binding status")

    yd_notes = yd_sub.add_parser("notes", help="Browse Youdao notes")
    yd_notes.add_argument("--folder-id", help="Folder ID")

    yd_import = yd_sub.add_parser("import", help="Import single note")
    yd_import.add_argument("nb_id", type=int, help="Target notebook ID")
    yd_import.add_argument("file_id", help="Youdao file ID")

    yd_batch = yd_sub.add_parser("import-batch", help="Import multiple notes")
    yd_batch.add_argument("nb_id", type=int, help="Target notebook ID")
    yd_batch.add_argument("--file-ids", required=True, help="Comma-separated file IDs")

    # ── Import ──
    imp_parser = subparsers.add_parser("import", help="Import files")
    imp_sub = imp_parser.add_subparsers(dest="subcmd")

    imp_file = imp_sub.add_parser("file", help="Import file (shows curl command)")
    imp_file.add_argument("nb_id", type=int, help="Notebook ID")
    imp_file.add_argument("file", help="File path")

    imp_url = imp_sub.add_parser("url", help="Import from URL")
    imp_url.add_argument("nb_id", type=int, help="Notebook ID")
    imp_url.add_argument("url", help="URL to import")

    imp_task = imp_sub.add_parser("task", help="Check import task status")
    imp_task.add_argument("task_id", help="Task ID")

    imp_del_task = imp_sub.add_parser("delete-task", help="Delete/cancel import task")
    imp_del_task.add_argument("task_id", help="Task ID")

    imp_audio = imp_sub.add_parser("audio-preview", help="Preview audio (shows curl)")
    imp_audio.add_argument("nb_id", type=int, help="Notebook ID")
    imp_audio.add_argument("file", help="Audio file path")

    imp_audio_status = imp_sub.add_parser("audio-status", help="Check audio preview status")
    imp_audio_status.add_argument("preview_id", help="Preview ID")

    imp_audio_confirm = imp_sub.add_parser("audio-confirm", help="Confirm audio import")
    imp_audio_confirm.add_argument("preview_id", help="Preview ID")
    imp_audio_confirm.add_argument("content", help="Transcribed content")

    # ── Search ──
    search_parser = subparsers.add_parser("search", help="Search in notebook")
    search_sub = search_parser.add_subparsers(dest="subcmd")

    search_query = search_sub.add_parser("query", help="Search in notebook")
    search_query.add_argument("nb_id", type=int, help="Notebook ID")
    search_query.add_argument("query", help="Search query")

    search_import = search_sub.add_parser("import-results", help="Import search results")
    search_import.add_argument("nb_id", type=int, help="Notebook ID")
    search_import.add_argument("--urls", required=True, help="Comma-separated URLs")

    # ── Memory ──
    mem_parser = subparsers.add_parser("memory", help="Manage user preferences")
    mem_sub = mem_parser.add_subparsers(dest="subcmd")

    mem_sub.add_parser("list", help="List all preferences")

    mem_set = mem_sub.add_parser("set", help="Set preference")
    mem_set.add_argument("type", help="Preference type")
    mem_set.add_argument("content", help="Preference content")

    mem_del = mem_sub.add_parser("delete", help="Delete preference")
    mem_del.add_argument("type", help="Preference type")

    # ── Feedback ──
    fb_parser = subparsers.add_parser("feedback", help="Manage answer feedback")
    fb_sub = fb_parser.add_subparsers(dest="subcmd")

    fb_up = fb_sub.add_parser("up", help="Like an answer")
    fb_up.add_argument("message_id", type=int, help="Message ID")
    fb_up.add_argument("--reason", help="Reason (default: helpful)")

    fb_down = fb_sub.add_parser("down", help="Dislike an answer")
    fb_down.add_argument("message_id", type=int, help="Message ID")
    fb_down.add_argument("--reason", help="Reason (default: not_helpful)")

    fb_del = fb_sub.add_parser("delete", help="Delete feedback")
    fb_del.add_argument("message_id", type=int, help="Message ID")

    # ── Admin ──
    admin_parser = subparsers.add_parser("admin", help="Admin operations")
    admin_sub = admin_parser.add_subparsers(dest="subcmd")

    admin_sub.add_parser("users", help="List all users")

    admin_status = admin_sub.add_parser("update-user-status", help="Update user status")
    admin_status.add_argument("user_id", type=int, help="User ID")
    admin_status.add_argument("status", choices=["active", "disabled"], help="New status")

    admin_sub.add_parser("config-status", help="Get config status")

    admin_cfg_get = admin_sub.add_parser("config-get", help="Get config group")
    admin_cfg_get.add_argument("group", help="Config group name")

    admin_cfg_add = admin_sub.add_parser("config-add", help="Add config")
    admin_cfg_add.add_argument("group", help="Config group")
    admin_cfg_add.add_argument("key", help="Config key")
    admin_cfg_add.add_argument("value", help="Config value")

    admin_cfg_update = admin_sub.add_parser("config-update", help="Update config")
    admin_cfg_update.add_argument("group", help="Config group")
    admin_cfg_update.add_argument("key", help="Config key")
    admin_cfg_update.add_argument("value", help="New value")

    admin_cfg_del = admin_sub.add_parser("config-delete", help="Delete config")
    admin_cfg_del.add_argument("group", help="Config group")
    admin_cfg_del.add_argument("key", help="Config key")

    fb_overview = admin_sub.add_parser("feedback-overview", help="Feedback statistics")
    fb_overview.add_argument("--from-date", required=True, help="Start date (RFC3339)")
    fb_overview.add_argument("--to-date", required=True, help="End date (RFC3339)")

    fb_list = admin_sub.add_parser("feedback-list", help="List feedback details")
    fb_list.add_argument("--from-date", required=True, help="Start date (RFC3339)")
    fb_list.add_argument("--to-date", required=True, help="End date (RFC3339)")
    fb_list.add_argument("--rating", choices=["up", "down"], help="Filter by rating")
    fb_list.add_argument("--reason", help="Filter by reason")
    fb_list.add_argument("--page", type=int, default=1, help="Page number")
    fb_list.add_argument("--size", type=int, default=20, help="Page size")

    fb_export = admin_sub.add_parser("feedback-export", help="Export feedback CSV")
    fb_export.add_argument("--from-date", required=True, help="Start date (RFC3339)")
    fb_export.add_argument("--to-date", required=True, help="End date (RFC3339)")

    admin_sub.add_parser("mysql", help="Connect to MySQL console")

    # ── Infra ──
    server_parser = subparsers.add_parser("server", help="Manage Go backend server")
    server_sub = server_parser.add_subparsers(dest="subcmd")

    server_run = server_sub.add_parser("run", help="Start server")
    server_run.add_argument("-p", "--port", type=int, default=DEFAULT_PORT)

    server_build = server_sub.add_parser("build", help="Build binary")
    server_build.add_argument("-o", "--output", help="Output path")

    server_status = server_sub.add_parser("status", help="Check status")
    server_status.add_argument("-p", "--port", type=int, default=DEFAULT_PORT)

    docker_parser = subparsers.add_parser("docker", help="Manage Docker services")
    docker_sub = docker_parser.add_subparsers(dest="subcmd")

    docker_up = docker_sub.add_parser("up", help="Start services")
    docker_up.add_argument("--build", action="store_true")
    docker_up.add_argument("--pull", action="store_true")
    docker_up.add_argument("services", nargs="*")

    docker_down = docker_sub.add_parser("down", help="Stop services")
    docker_down.add_argument("-v", "--volumes", action="store_true")

    docker_restart = docker_sub.add_parser("restart", help="Restart services")
    docker_restart.add_argument("services", nargs="*")

    docker_logs = docker_sub.add_parser("logs", help="View logs")
    docker_logs.add_argument("-n", "--tail", type=int)
    docker_logs.add_argument("services", nargs="*")

    docker_sub.add_parser("ps", help="List running services")
    docker_sub.add_parser("pull", help="Pull images")
    docker_sub.add_parser("build", help="Build images")

    config_parser = subparsers.add_parser("config", help="Manage configuration")
    config_sub = config_parser.add_subparsers(dest="subcmd")

    config_show = config_sub.add_parser("show", help="Show config")
    config_show.add_argument("-f", "--file", help="Config filename")

    config_validate = config_sub.add_parser("validate", help="Validate config")
    config_validate.add_argument("-f", "--file", help="Config filename")

    health_parser = subparsers.add_parser("health", help="Check health")
    health_parser.add_argument("-p", "--port", type=int, default=DEFAULT_PORT)

    subparsers.add_parser("version", help="Show version")

    return parser


# ─── Main ─────────────────────────────────────────────────────────────────────

def main():
    """Main entry point."""
    parser = build_parser()
    args = parser.parse_args()

    if not args.command:
        parser.print_help()
        sys.exit(0)

    if args.chdir:
        os.chdir(args.chdir)

    # Find project root for infra commands
    root = Path.cwd()
    if args.command in ["server", "docker", "config", "health"]:
        root = find_project_root()

    # Command dispatch
    commands = {
        "login": cmd_login,
        "logout": cmd_logout,
        "whoami": cmd_whoami,
        "register": cmd_register,
        "send-code": cmd_send_code,
        "reset-password": cmd_reset_password,
        "refresh": cmd_refresh_token,
        "profile": cmd_profile,
        "config-llm": cmd_config_llm,
        "config-search": cmd_config_search,
        "config-embedding": cmd_config_embedding,
        "config-asr": cmd_config_asr,
        "config-reranker": cmd_config_reranker,
        "config-active": cmd_config_active,
        "providers": cmd_providers,
        "notebook": cmd_notebook,
        "source": cmd_source,
        "chat": cmd_chat,
        "generation": cmd_generation,
        "youdao": cmd_youdao,
        "import": cmd_import,
        "search": cmd_search,
        "memory": cmd_memory,
        "feedback": cmd_feedback,
        "admin": cmd_admin,
        "server": cmd_server,
        "docker": cmd_docker,
        "config": cmd_config,
        "health": cmd_health,
        "version": cmd_version,
    }

    handler = commands.get(args.command)
    if handler:
        handler(args, root)
    else:
        parser.print_help()


if __name__ == "__main__":
    main()
