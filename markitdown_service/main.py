from fastapi import FastAPI, UploadFile, File, Form, HTTPException
from markitdown import MarkItDown
import io
import requests
import logging
import traceback
import uvicorn
from requests.adapters import HTTPAdapter
from urllib3.util.retry import Retry

# 日志配置
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

app = FastAPI(title="文档+网页转Markdown服务")
md = MarkItDown()

# 更完整的浏览器 headers
HEADERS = {
    "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36",
    "Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8",
    "Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
    "Accept-Encoding": "gzip, deflate, br",
    "Connection": "keep-alive",
    "Upgrade-Insecure-Requests": "1",
}

# 创建带重试的 session
def create_session():
    session = requests.Session()
    retry_strategy = Retry(
        total=3,
        backoff_factor=0.5,
        status_forcelist=[429, 500, 502, 503, 504],
    )
    adapter = HTTPAdapter(max_retries=retry_strategy)
    session.mount("http://", adapter)
    session.mount("https://", adapter)
    session.headers.update(HEADERS)
    return session

session = create_session()


def fetch_webpage(url: str) -> tuple[bytes, str]:
    """获取网页内容，返回 (content, error_msg)"""
    try:
        resp = session.get(url, timeout=20, allow_redirects=True)
        if resp.status_code == 403:
            from urllib.parse import urlparse
            parsed = urlparse(url)
            referer = f"{parsed.scheme}://{parsed.netloc}/"
            resp = session.get(url, timeout=20, allow_redirects=True, headers={"Referer": referer})

        if resp.status_code in [429, 403, 404] or resp.status_code >= 500:
            return b"", f"网页请求失败，状态码：{resp.status_code}"

        resp.raise_for_status()
        return resp.content, ""

    except requests.exceptions.RequestException as e:
        return b"", f"网络请求失败: {str(e)}"


@app.post("/convert")
async def convert(file: UploadFile = File(...)):
    """文件转 Markdown"""
    try:
        logger.info(f"处理文件: {file.filename}")
        content = await file.read()
        stream = io.BytesIO(content)
        stream.name = file.filename or "unknown"

        try:
            result = md.convert(stream)
            markdown_text = result.markdown if hasattr(result, 'markdown') else str(result)
        except Exception as e:
            raise HTTPException(status_code=400, detail="文件格式不支持或无法转换")

        return {"filename": file.filename, "markdown": markdown_text}

    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"服务错误：{str(e)}")
        raise HTTPException(status_code=500, detail="服务内部错误")


@app.post("/convert_url")
async def convert_url(url: str = Form(...)):
    """网页 URL 转 Markdown"""
    try:
        logger.info(f"处理网页: {url}")
        if not url.startswith(("http://", "https://")):
            url = "https://" + url

        content, error = fetch_webpage(url)
        if error:
            raise HTTPException(status_code=502, detail=error)

        stream = io.BytesIO(content)
        stream.name = "page.html"

        try:
            result = md.convert(stream)
            markdown_text = result.markdown if hasattr(result, 'markdown') else str(result)
        except Exception as e:
            logger.warning(f"网页无法转换: {url}")
            return {
                "url": url,
                "markdown": "",
                "message": "该网站为JS渲染/反爬页面，无法自动转换（无需处理，属于正常情况）"
            }

        return {"url": url, "markdown": markdown_text}

    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"服务错误：{str(e)}")
        raise HTTPException(status_code=500, detail="服务内部错误")


if __name__ == "__main__":
    uvicorn.run(app, host="0.0.0.0", port=8085)
