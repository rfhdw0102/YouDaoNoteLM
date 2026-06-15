# 对话模块接口文档

> 基础路径：`/api/v1`
>
> Content-Type：`application/json`
>
> 需要认证的接口需在请求头中携带：`Authorization: Bearer <access_token>`

---

## 通用响应格式

```json
{
  "code": 0,
  "message": "成功",
  "data": {}
}
```

---

## SSE 流式响应格式

发送消息接口使用 Server-Sent Events (SSE) 进行流式响应：

```
event: token
data: {"type":"token","content":"你好","data":null}

event: reference
data: {"type":"reference","content":"","data":[{"source_id":1,"source_name":"文档.pdf","parent_block_id":100,"chunk_content":"相关内容...","score":0.95}]}

event: done
data: {"type":"done","content":"","data":null}

event: error
data: {"type":"error","content":"错误信息","data":null}
```

### 事件类型说明

| 事件类型 | 说明 |
|----------|------|
| `token` | 流式生成的文本片段 |
| `reference` | RAG 检索到的引用来源 |
| `done` | 生成完成 |
| `error` | 发生错误 |

---

## 对话管理接口

### 1. 创建对话

创建一个新的对话会话。

**请求**

```
POST /api/v1/chat/conversations
Authorization: Bearer <access_token>
```

```json
{
  "notebook_id": 1,
  "title": "关于机器学习的问题"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|:----:|------|
| notebook_id | uint | ✅ | 所属笔记本 ID |
| title | string | ❌ | 对话标题，默认为"新对话" |

**成功响应**

```json
{
  "code": 0,
  "message": "成功",
  "data": {
    "id": 1
  }
}
```

---

### 2. 获取对话列表

获取指定笔记本下的所有对话，按更新时间降序排列。

**请求**

```
GET /api/v1/chat/notebooks/{nbId}/conversations
Authorization: Bearer <access_token>
```

| 参数 | 类型 | 必填 | 说明 |
|------|------|:----:|------|
| nbId | uint | ✅ | 笔记本 ID（路径参数） |

**成功响应**

```json
{
  "code": 0,
  "message": "成功",
  "data": [
    {
      "id": 2,
      "title": "关于深度学习的问题",
      "notebook_id": 1,
      "created_at": "2026-06-11T12:00:00Z",
      "updated_at": "2026-06-11T12:30:00Z"
    },
    {
      "id": 1,
      "title": "关于机器学习的问题",
      "notebook_id": 1,
      "created_at": "2026-06-11T10:00:00Z",
      "updated_at": "2026-06-11T11:00:00Z"
    }
  ]
}
```

---

### 3. 获取对话详情

**请求**

```
GET /api/v1/chat/conversations/{convId}
Authorization: Bearer <access_token>
```

| 参数 | 类型 | 必填 | 说明 |
|------|------|:----:|------|
| convId | uint | ✅ | 对话 ID（路径参数） |

**成功响应**

```json
{
  "code": 0,
  "message": "成功",
  "data": {
    "id": 1,
    "title": "关于机器学习的问题",
    "notebook_id": 1,
    "created_at": "2026-06-11T10:00:00Z",
    "updated_at": "2026-06-11T11:00:00Z"
  }
}
```

**错误场景**

| code | message | 原因 |
|------|---------|------|
| 404 | 资源不存在 | 对话不存在 |

---

### 4. 更新对话标题

**请求**

```
PUT /api/v1/chat/conversations/{convId}
Authorization: Bearer <access_token>
```

```json
{
  "title": "新的对话标题"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|:----:|------|
| title | string | ✅ | 新的对话标题 |

**成功响应**

```json
{
  "code": 0,
  "message": "成功",
  "data": null
}
```

**错误场景**

| code | message | 原因 |
|------|---------|------|
| 404 | 资源不存在 | 对话不存在 |

---

### 5. 删除对话

删除对话会级联删除所有关联的消息。

**请求**

```
DELETE /api/v1/chat/conversations/{convId}
Authorization: Bearer <access_token>
```

| 参数 | 类型 | 必填 | 说明 |
|------|------|:----:|------|
| convId | uint | ✅ | 对话 ID（路径参数） |

**成功响应**

```json
{
  "code": 0,
  "message": "成功",
  "data": null
}
```

**错误场景**

| code | message | 原因 |
|------|---------|------|
| 404 | 资源不存在 | 对话不存在 |

---

## 消息接口

### 6. 获取消息历史

获取对话的所有消息，按时间正序排列。

**请求**

```
GET /api/v1/chat/conversations/{convId}/messages
Authorization: Bearer <access_token>
```

| 参数 | 类型 | 必填 | 说明 |
|------|------|:----:|------|
| convId | uint | ✅ | 对话 ID（路径参数） |

**成功响应**

```json
{
  "code": 0,
  "message": "成功",
  "data": [
    {
      "id": 1,
      "role": "user",
      "content": "什么是机器学习？",
      "metadata": null,
      "created_at": "2026-06-11T10:00:00Z"
    },
    {
      "id": 2,
      "role": "assistant",
      "content": "机器学习是人工智能的一个分支...",
      "metadata": {
        "references": [
          {
            "source_id": 1,
            "source_name": "机器学习导论.pdf",
            "parent_block_id": 100,
            "chunk_content": "机器学习是人工智能的一个重要分支...",
            "score": 0.95
          }
        ]
      },
      "created_at": "2026-06-11T10:00:05Z"
    }
  ]
}
```

**响应字段说明**

| 字段 | 类型 | 说明 |
|------|------|------|
| id | uint | 消息 ID |
| role | string | 角色：`user` / `assistant` / `system` |
| content | string | 消息内容 |
| metadata | object | 元数据（仅助手消息） |
| metadata.references | array | RAG 引用来源列表 |
| metadata.references[].source_id | uint | 资料来源 ID |
| metadata.references[].source_name | string | 资料来源名称 |
| metadata.references[].parent_block_id | int64 | 父块 ID |
| metadata.references[].chunk_content | string | 匹配的 chunk 内容 |
| metadata.references[].score | float | 相关度分数 |
| created_at | string | 创建时间（ISO 8601） |

---

### 7. 发送消息（流式）

发送用户消息并获取 AI 流式回答。使用 SSE (Server-Sent Events) 进行流式响应。

**请求**

```
POST /api/v1/chat/conversations/{convId}/messages
Authorization: Bearer <access_token>
Content-Type: application/json
```

```json
{
  "content": "请详细解释一下机器学习的分类",
  "source_ids": [1, 2, 3]
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|:----:|------|
| content | string | ✅ | 用户消息内容 |
| source_ids | uint[] | ❌ | 限定的资料来源 ID 列表，为空则搜索所有 |

**响应**

```
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive
```

**SSE 事件流示例**

```
event: reference
data: {"type":"reference","content":"","data":[{"source_id":1,"source_name":"机器学习导论.pdf","parent_block_id":100,"chunk_content":"机器学习主要分为三类...","score":0.92}]}

event: token
data: {"type":"token","content":"机器","data":null}

event: token
data: {"type":"token","content":"学习","data":null}

event: token
data: {"type":"token","content":"主要","data":null}

event: token
data: {"type":"token","content":"分为三类：","data":null}

...

event: done
data: {"type":"done","content":"机器学习主要分为三类：监督学习、无监督学习和强化学习。","data":null}
```

**错误响应**

```
event: error
data: {"type":"error","content":"RAG 检索失败","data":null}
```

---

### 8. 终止回答生成

终止正在进行的回答生成。

**请求**

```
POST /api/v1/chat/conversations/{convId}/stop
Authorization: Bearer <access_token>
```

| 参数 | 类型 | 必填 | 说明 |
|------|------|:----:|------|
| convId | uint | ✅ | 对话 ID（路径参数） |

**成功响应**

```json
{
  "code": 0,
  "message": "成功",
  "data": null
}
```

**错误场景**

| code | message | 原因 |
|------|---------|------|
| 404 | 未找到正在进行的生成任务 | 没有正在生成的回答 |

---

## 数据结构

### ConversationResponse

```json
{
  "id": 1,
  "title": "对话标题",
  "notebook_id": 1,
  "created_at": "2026-06-11T10:00:00Z",
  "updated_at": "2026-06-11T11:00:00Z"
}
```

### MessageResponse

```json
{
  "id": 1,
  "role": "user",
  "content": "消息内容",
  "metadata": null,
  "created_at": "2026-06-11T10:00:00Z"
}
```

### StreamEvent

```json
{
  "type": "token",
  "content": "文本内容",
  "data": null
}
```

### Reference

```json
{
  "source_id": 1,
  "source_name": "文档名称.pdf",
  "parent_block_id": 100,
  "chunk_content": "匹配的文本片段",
  "score": 0.95
}
```

---

## 错误码一览

| 错误码 | 说明 |
|--------|------|
| 0 | 成功 |
| 400 | 请求参数错误 |
| 401 | 未登录 |
| 404 | 资源不存在 |
| 409 | 冲突（如对话正在处理中） |
| 500 | 服务器内部错误 |

---

## 前端集成指南

### SSE 流式接收示例（JavaScript）

```javascript
const eventSource = new EventSource(`/api/v1/chat/conversations/${convId}/messages`, {
  headers: {
    'Authorization': `Bearer ${accessToken}`
  }
});

// 注意：EventSource 不支持自定义 header，需要使用 fetch API
const response = await fetch(`/api/v1/chat/conversations/${convId}/messages`, {
  method: 'POST',
  headers: {
    'Authorization': `Bearer ${accessToken}`,
    'Content-Type': 'application/json'
  },
  body: JSON.stringify({
    content: '用户消息',
    source_ids: []
  })
});

const reader = response.body.getReader();
const decoder = new TextDecoder();

while (true) {
  const { done, value } = await reader.read();
  if (done) break;
  
  const text = decoder.decode(value);
  // 解析 SSE 事件
  const lines = text.split('\n');
  for (const line of lines) {
    if (line.startsWith('data: ')) {
      const data = JSON.parse(line.slice(6));
      switch (data.type) {
        case 'token':
          // 追加文本到 UI
          appendText(data.content);
          break;
        case 'reference':
          // 显示引用来源
          showReferences(data.data);
          break;
        case 'done':
          // 完成
          onComplete(data.content);
          break;
        case 'error':
          // 错误处理
          onError(data.content);
          break;
      }
    }
  }
}
```

### 终止生成示例

```javascript
async function stopGeneration(convId) {
  await fetch(`/api/v1/chat/conversations/${convId}/stop`, {
    method: 'POST',
    headers: {
      'Authorization': `Bearer ${accessToken}`
    }
  });
}
```

---

## 数据关联说明

```
notebooks
  └── conversations (一个笔记本包含多个对话)
        └── messages (一个对话包含多条消息)
```

删除对话时的级联删除关系：

```
conversations (删除)
  └── messages (级联删除)
```
