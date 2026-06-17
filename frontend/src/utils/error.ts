// 统一错误处理工具

// 字段名中文映射（兜底用，后端已处理大部分情况）
const FIELD_LABEL_MAP: Record<string, string> = {
  Name: '配置名称',
  Provider: '服务商',
  APIKey: 'API Key',
  APIURL: 'API 地址',
  Model: '模型名称',
  Dimensions: '向量维度',
  DailyQuota: '每日配额',
  ExtraConfig: '扩展配置',
  Enabled: "启用状态",
  Email: '邮箱',
  Password: '密码',
  Nickname: '昵称',
  Username: '用户名',
  Code: '验证码',
  RefreshToken: '刷新令牌',
};

/**
 * 解析后端 validator 原始错误格式（兜底）
 * 格式: Key: 'UserConfigRequest.Name' Error:Field validation for 'Name' failed on the 'required' tag
 */
function parseRawValidatorError(message: string): string | null {
  // 匹配所有 "Key: '...' Error:Field validation for '...' failed on the '...' tag" 模式
  const pattern = /Key:\s*'[^']*\.(\w+)'\s*Error:Field validation for '\w+' failed on the '(\w+)'/g;
  const parts: string[] = [];
  let match: RegExpExecArray | null;

  while ((match = pattern.exec(message)) !== null) {
    const fieldName = match[1];
    const tag = match[2];
    const label = FIELD_LABEL_MAP[fieldName] || fieldName;

    switch (tag) {
      case 'required':
        parts.push(`${label}不能为空`);
        break;
      case 'min':
        parts.push(`${label}长度不足`);
        break;
      case 'max':
        parts.push(`${label}长度超出限制`);
        break;
      case 'email':
        parts.push(`${label}格式不正确`);
        break;
      default:
        parts.push(`${label}验证失败`);
    }
  }

  return parts.length > 0 ? parts.join('；') : null;
}

/**
 * 从 API 错误响应中提取错误信息
 * 支持后端返回的标准格式: { code: number, message: string }
 */
export function getErrorMessage(error: any, fallback: string = '操作失败'): string {
  // 1. 后端返回的标准错误格式
  if (error?.response?.data?.message) {
    const msg = error.response.data.message;
    // 兜底：检测是否为原始 validator 错误格式
    const parsed = parseRawValidatorError(msg);
    return parsed || msg;
  }

  // 2. 后端返回的标准错误格式（非 HTTP 错误）
  if (error?.message && typeof error.message === 'string') {
    // 过滤掉 axios 内部错误信息
    if (error.message === 'token_refresh_failed') {
      return '登录已过期，请重新登录';
    }
    // 兜底：检测是否为原始 validator 错误格式
    const parsed = parseRawValidatorError(error.message);
    return parsed || error.message;
  }

  // 3. 网络错误
  if (error?.code === 'ERR_NETWORK') {
    return '网络连接失败，请检查网络';
  }

  // 4. 超时错误
  if (error?.code === 'ECONNABORTED') {
    return '请求超时，请稍后重试';
  }

  return fallback;
}

/**
 * 从 API 响应中检查是否为错误
 * 后端返回格式: { code: 0 表示成功, 其他表示错误 }
 */
export function isApiError(response: { code: number }): boolean {
  return response.code !== 0;
}

/**
 * 常见错误码映射
 */
export const ERROR_CODES: Record<number, string> = {
  0: '成功',
  400: '请求参数错误',
  401: '未授权',
  403: '禁止访问',
  404: '资源不存在',
  408: '请求超时',
  409: '资源冲突',
  500: '服务器内部错误',
  501: '功能未实现',
  503: '服务不可用',
  // 业务错误码
  1001: '用户不存在',
  1002: '用户已存在',
  1003: '邮箱或密码错误',
  1004: '用户已被禁用',
  1005: '无效的令牌',
  1006: '令牌已过期',
  1007: '账户已被锁定',
  1101: '验证码已过期',
  1102: '验证码错误',
  1103: '验证码输入错误次数过多',
  1104: '验证码发送过于频繁',
  2001: '参数错误',
  2002: '缺少必要参数',
  2003: '参数格式错误',
  3001: '资源不存在',
  3002: '资源已存在',
  3003: '资源已被锁定',
  40001: '不支持的文件格式',
  40002: '文件大小超限',
  40003: '文件解析失败',
  40004: '网页抓取失败',
  40005: '音频转写失败',
  40006: '搜索API配额耗尽',
  40007: '有道API Key无效',
  40008: '重复导入',
  40009: '预览已过期',
  40010: '请先在设置中配置 LLM 服务',
  40011: 'LLM 服务调用失败',
  40012: 'LLM 返回结果格式异常',
  40013: '搜索 Agent 执行超时',
  50001: '内部服务错误',
  // 配置健康检查错误码
  60001: '配置连通性测试失败',
  60002: '配置连通性测试超时',
  60003: '配置参数无效',
};

/**
 * 根据错误码获取错误信息
 */
export function getErrorCodeMessage(code: number): string {
  return ERROR_CODES[code] || '未知错误';
}

/**
 * 聊天/对话相关的原始错误 → 友好提示映射
 * 后端可能返回原始 Go 错误信息，需要转换为用户可读的提示
 */
const CHAT_ERROR_PATTERNS: Array<[RegExp, string]> = [
  // LLM 服务相关
  [/LLM.*配置.*不存在/i, '所选模型配置不存在，请在设置中重新配置'],
  [/LLM.*调用.*失败/i, '模型服务调用失败，请检查模型配置或稍后重试'],
  [/LLM.*返回.*异常/i, '模型返回结果异常，请稍后重试'],
  [/未.*配置.*LLM/i, '请先在设置中配置 LLM 服务'],
  [/no.*llm.*config/i, '请先在设置中配置 LLM 服务'],
  [/api.*key.*无效/i, 'API Key 无效，请在设置中更新'],
  [/api.*key.*过期/i, 'API Key 已过期，请在设置中更新'],
  [/quota.*exceed/i, 'API 配额已耗尽，请稍后重试'],
  [/rate.*limit/i, '请求过于频繁，请稍后重试'],
  [/context.*length/i, '对话内容过长，请新建对话或减少输入'],
  [/token.*limit/i, '对话内容超出长度限制，请新建对话'],

  // 网络/超时相关
  [/timeout/i, '请求超时，请稍后重试'],
  [/connection.*refused/i, '无法连接到模型服务，请稍后重试'],
  [/network.*error/i, '网络连接异常，请检查网络后重试'],
  [/context.*cancel/i, '请求已取消'],
  [/context.*deadline/i, '请求超时，请稍后重试'],

  // 向量/RAG 相关
  [/vector/i, '知识库检索异常，请稍后重试'],
  [/embedding/i, '向量化处理失败，请稍后重试'],
  [/milvus/i, '知识库服务异常，请稍后重试'],

  // 通用
  [/内部.*错误/i, '服务内部异常，请稍后重试'],
  [/internal.*error/i, '服务内部异常，请稍后重试'],
];

/**
 * 将聊天场景下的原始错误转换为友好提示
 * 如果无法匹配已知模式，返回默认的友好提示
 */
export function getChatErrorMessage(error: string): string {
  if (!error) return '回答生成失败，请重试';

  for (const [pattern, friendly] of CHAT_ERROR_PATTERNS) {
    if (pattern.test(error)) {
      return friendly;
    }
  }

  // 如果错误信息已经是中文且较短（≤20字），可能是后端已处理过的友好提示，直接使用
  if (/^[一-龥]+$/.test(error) && error.length <= 20) {
    return error;
  }

  // 兜底：不暴露原始错误
  return '回答生成失败，请重试';
}
