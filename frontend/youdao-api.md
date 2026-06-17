# 有道云笔记 API 文档

## 概述

有道云笔记模块提供绑定/解绑有道云笔记账号、浏览笔记目录、以及将有道云笔记导入到本系统的功能。所有接口均需要认证（Bearer Token）。

**基础路径:** `/api/v1/youdao`

**认证方式:** 所有接口需要在请求头中携带 `Authorization: Bearer {token}`

---

## 统一响应格式

```json
{
  "code": 0,
  "message": "成功",
  "data": {}
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `code` | int | 状态码，0 表示成功 |
| `message` | string | 响应消息 |
| `data` | object | 响应数据（可选） |

---

## API 端点

### 1. 绑定有道云笔记 API Key

**端点:** `POST /api/v1/youdao/bind`

**描述:** 验证并绑定用户的有道云笔记 API Key。绑定前会通过 CLI 调用验证 Key 的有效性。

**认证:** 需要

**请求体:**

```json
{
  "api_key": "your_youdao_api_key"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `api_key` | string | 是 | 有道云笔记 API Key |

**响应示例:**

```json
{
  "code": 0,
  "message": "绑定成功",
  "data": null
}
```

**错误响应:**

| code | message | 说明 |
|------|---------|------|
| 400 | 请提供有效的 API Key | 请求体缺少 api_key 字段 |
| 1001 | API Key 无效，请检查后重试 | CLI 验证 Key 失败 |
| 1001 | youdaonote CLI 不可用 | CLI 未安装或不可用 |

---

### 2. 解绑有道云笔记账号

**端点:** `DELETE /api/v1/youdao/bind`

**描述:** 解除用户的有道云笔记绑定关系。

**认证:** 需要

**请求参数:** 无

**响应示例:**

```json
{
  "code": 0,
  "message": "解绑成功",
  "data": null
}
```

---

### 3. 查询绑定状态

**端点:** `GET /api/v1/youdao/bind`

**描述:** 获取当前用户的有道云笔记绑定信息。

**认证:** 需要

**请求参数:** 无

**响应示例（已绑定）:**

```json
{
  "code": 0,
  "message": "成功",
  "data": {
    "bound": true,
    "status": "active"
  }
}
```

**响应示例（未绑定）:**

```json
{
  "code": 0,
  "message": "成功",
  "data": {
    "bound": false
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `bound` | bool | 是否已绑定 |
| `status` | string | 绑定状态，`active` 表示有效（仅 bound=true 时返回） |

---

### 4. 浏览有道云笔记目录

**端点:** `GET /api/v1/youdao/notes`

**描述:** 列出有道云笔记的目录和笔记列表。支持通过 `folderId` 参数浏览子目录。

**认证:** 需要

**查询参数:**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `folderId` | string | 否 | 目录 ID，为空则列出根目录 |

**响应示例:**

```json
{
  "code": 0,
  "message": "成功",
  "data": [
    {
      "id": "note_abc123",
      "name": "我的笔记",
      "type": "file",
      "parentId": ""
    },
    {
      "id": "folder_xyz789",
      "name": "学习资料",
      "type": "dir",
      "parentId": ""
    }
  ]
}
```

**数据结构 — YoudaoNoteItem:**

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | string | 笔记/目录 ID |
| `name` | string | 名称 |
| `type` | string | 类型：`file`（笔记）或 `dir`（目录） |
| `parentId` | string | 父目录 ID（根目录时为空） |

---

### 5. 导入单篇有道云笔记

**端点:** `POST /api/v1/youdao/import`

**描述:** 将有道云笔记导入到本系统指定笔记本。导入成功后会自动触发异步向量化。

**认证:** 需要

**请求体:**

```json
{
  "file_id": "note_abc123",
  "notebook_id": 1
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `file_id` | string | 是 | 有道云笔记 ID（从 list 接口获取） |
| `notebook_id` | uint | 是 | 目标笔记本 ID（本系统的笔记本） |

**响应示例:**

```json
{
  "code": 0,
  "message": "成功",
  "data": {
    "id": 42,
    "created_at": "2026-06-10T12:00:00Z",
    "updated_at": "2026-06-10T12:00:00Z",
    "user_id": 1,
    "notebook_id": 1,
    "name": "我的笔记",
    "type": "youdao",
    "external_id": "note_abc123",
    "status": "ready",
    "vectorized": false,
    "markdown_content": "# 我的笔记\n\n这是笔记内容..."
  }
}
```

**数据结构 — Source（导入结果）:**

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | uint | Source 记录 ID |
| `user_id` | uint | 用户 ID |
| `notebook_id` | uint | 笔记本 ID |
| `name` | string | 笔记名称 |
| `type` | string | 来源类型，固定为 `youdao` |
| `external_id` | string | 有道云笔记 ID |
| `status` | string | 状态：`ready`（就绪）、`pending`（待处理）、`processing`（处理中）、`failed`（失败） |
| `vectorized` | bool | 是否已向量化 |
| `markdown_content` | string | Markdown 格式的笔记内容 |
| `created_at` | string | 创建时间（ISO 8601） |
| `updated_at` | string | 更新时间（ISO 8601） |

**错误响应:**

| code | message | 说明 |
|------|---------|------|
| 400 | 请提供有效的导入参数 | 缺少 file_id 或 notebook_id |
| 1001 | 请先绑定有道云笔记账号 | 用户未绑定或绑定已失效 |
| 1001 | 读取笔记内容失败 | CLI 读取笔记失败 |

---

### 6. 批量导入有道云笔记

**端点:** `POST /api/v1/youdao/import/batch`

**描述:** 批量将有道云笔记导入到本系统指定笔记本。返回任务 ID 和 Source ID 列表，实际导入在后台异步执行（并发度为 3）。

**认证:** 需要

**请求体:**

```json
{
  "file_ids": ["note_abc123", "note_def456", "note_ghi789"],
  "notebook_id": 1
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `file_ids` | []string | 是 | 有道云笔记 ID 列表（至少 1 个，自动去重） |
| `notebook_id` | uint | 是 | 目标笔记本 ID |

**响应示例:**

```json
{
  "code": 0,
  "message": "成功",
  "data": {
    "task_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "source_ids": [42, 43, 44]
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `task_id` | string | 批量导入任务 ID（用于取消任务） |
| `source_ids` | []uint | 创建的 Source 记录 ID 列表 |

**批量导入状态流转:**

```
pending → processing → ready
                    → failed（带失败原因）
pending → cancelled（任务被取消）
```

**错误响应:**

| code | message | 说明 |
|------|---------|------|
| 400 | 请提供有效的导入参数 | file_ids 为空或缺少 notebook_id |
| 1001 | 请先绑定有道云笔记账号 | 用户未绑定或绑定已失效 |
| 1001 | 创建导入记录失败 | 所有 Source 记录创建失败 |

---

## 数据结构汇总

### YoudaoNoteItem

有道云笔记列表项，用于目录浏览和搜索结果。

```json
{
  "id": "note_abc123",
  "name": "我的笔记.md",
  "type": "file",
  "parentId": "folder_xyz789"
}
```

### Source

导入后的资料来源记录。

```json
{
  "id": 42,
  "created_at": "2026-06-10T12:00:00Z",
  "updated_at": "2026-06-10T12:00:00Z",
  "user_id": 1,
  "notebook_id": 1,
  "name": "我的笔记",
  "type": "youdao",
  "external_id": "note_abc123",
  "status": "ready",
  "vectorized": false,
  "markdown_content": "# 我的笔记"
}
```

---

## 错误码说明

| 错误码 | 说明 |
|--------|------|
| 0 | 成功 |
| 400 | 请求参数错误 |
| 1001 | 业务错误（如未绑定、CLI 不可用、导入失败等） |
| 1002 | 内部错误 |
| 401 | 未认证或 Token 已过期 |

---

## 使用流程

### 典型流程

```
1. 查询绑定状态  →  GET  /api/v1/youdao/bind
2. 绑定 API Key  →  POST /api/v1/youdao/bind
3. 浏览根目录    →  GET  /api/v1/youdao/notes
4. 浏览子目录    →  GET  /api/v1/youdao/notes?folderId=xxx
5. 导入单篇笔记  →  POST /api/v1/youdao/import
6. 批量导入笔记  →  POST /api/v1/youdao/import/batch
```

### 前端集成建议

1. **绑定页面**: 先调用 `GET /bind` 检查绑定状态，未绑定时显示 API Key 输入框
2. **浏览页面**: 使用树形组件，点击目录时传入 `folderId` 加载子目录
3. **导入确认**: 单篇导入直接调用；批量导入先收集选中的 `file_id` 列表
4. **状态轮询**: 批量导入后可通过 Source 列表接口查询各条目的 `status` 字段

---

## cURL 示例

```bash
# 查询绑定状态
curl -X GET "http://localhost:8080/api/v1/youdao/bind" \
  -H "Authorization: Bearer YOUR_TOKEN"

# 绑定 API Key
curl -X POST "http://localhost:8080/api/v1/youdao/bind" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"api_key": "your_youdao_api_key"}'

# 解绑
curl -X DELETE "http://localhost:8080/api/v1/youdao/bind" \
  -H "Authorization: Bearer YOUR_TOKEN"

# 浏览根目录
curl -X GET "http://localhost:8080/api/v1/youdao/notes" \
  -H "Authorization: Bearer YOUR_TOKEN"

# 浏览子目录
curl -X GET "http://localhost:8080/api/v1/youdao/notes?folderId=folder_xyz789" \
  -H "Authorization: Bearer YOUR_TOKEN"

# 导入单篇笔记
curl -X POST "http://localhost:8080/api/v1/youdao/import" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"file_id": "note_abc123", "notebook_id": 1}'

# 批量导入笔记
curl -X POST "http://localhost:8080/api/v1/youdao/import/batch" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"file_ids": ["note_abc123", "note_def456"], "notebook_id": 1}'
```
