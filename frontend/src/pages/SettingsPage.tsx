import { useState, useEffect } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import {
  Settings, Cpu, Search, Mic, Database, Plus, Trash2,
  Check, AlertCircle, ArrowLeft, Save, X, BookOpen,
  Loader2, Plug
} from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import { cn } from '../utils/cn';
import Button from '../components/ui/Button';
import Input from '../components/ui/Input';
import Badge from '../components/ui/Badge';
import * as userConfigApi from '../api/userConfig';
import * as providersApi from '../api/providers';
import * as youdaoApi from '../api/youdao';
import type { UserConfig, UserLLMConfig, UserConfigRequest } from '../api/userConfig';
import type { ProviderInfo } from '../api/providers';
import type { YoudaoBindStatus } from '../api/youdao';
import { getErrorMessage } from '../utils/error';

type ConfigTab = 'llm' | 'search' | 'asr' | 'embedding' | 'youdao';

// 默认 API 地址映射
const DEFAULT_API_URLS: Record<string, string> = {
  openai: 'https://api.openai.com/v1',
  anthropic: 'https://api.anthropic.com',
  deepseek: 'https://api.deepseek.com/v1',
  doubao: 'https://ark.cn-beijing.volces.com/api/v3',
  volcengine: 'https://ark.cn-beijing.volces.com/api/v3',
  zhipu: 'https://open.bigmodel.cn/api/paas/v4',
  qwen: 'https://dashscope.aliyuncs.com/compatible-mode/v1',
  baichuan: 'https://api.baichuan-ai.com/v1',
  moonshot: 'https://api.moonshot.cn/v1',
  minimax: 'https://api.minimax.chat/v1',
};

export default function SettingsPage() {
  const navigate = useNavigate();
  const [activeTab, setActiveTab] = useState<ConfigTab>('llm');
  const [configs, setConfigs] = useState<UserConfig[]>([]);
  const [loading, setLoading] = useState(false);
  const [showAddForm, setShowAddForm] = useState(false);
  const [editingId, setEditingId] = useState<number | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [llmConfigs, setLlmConfigs] = useState<UserLLMConfig[]>([]);
  const [providers, setProviders] = useState<ProviderInfo[]>([]);
  const [activeProvider, setActiveProvider] = useState<{source: string; provider: string; display_name: string} | null>(null);

  // 健康检查状态
  const [testing, setTesting] = useState(false);
  const [saving, setSaving] = useState(false);
  const [testResult, setTestResult] = useState<{ healthy: boolean; message: string; latency_ms: number; detail?: string } | null>(null);

  // 有道云配置状态
  const [youdaoBindStatus, setYoudaoBindStatus] = useState<YoudaoBindStatus | null>(null);
  const [youdaoApiKey, setYoudaoApiKey] = useState('');
  const [youdaoLoading, setYoudaoLoading] = useState(false);
  const [youdaoError, setYoudaoError] = useState<string | null>(null);

  // 删除确认弹窗状态
  const [deleteConfirmId, setDeleteConfirmId] = useState<number | null>(null);
  const [deleting, setDeleting] = useState(false);

  // Form state
  const [formData, setFormData] = useState<UserConfigRequest>({
    name: '',
    provider: '',
    api_key: '',
    api_url: '',
    model: '',
    daily_quota: 100,
    extra_config: {},
    dimensions: 2048,
  });

  // 获取当前选中 provider 的配置要求
  const getSelectedProviderInfo = (): ProviderInfo | undefined => {
    return providers.find(p => p.provider === formData.provider);
  };

  // 获取字段的中文标签
  const getFieldLabel = (fieldName: string): string => {
    const providerInfo = getSelectedProviderInfo();
    if (providerInfo?.key_labels?.[fieldName]) {
      return providerInfo.key_labels[fieldName];
    }
    // 默认标签映射
    const defaultLabels: Record<string, string> = {
      api_key: 'API Key',
      api_url: 'API 地址',
      model: '模型名称',
      access_key_id: 'Access Key ID',
      access_key_secret: 'Access Key Secret',
      app_key: 'App Key',
    };
    return defaultLabels[fieldName] || fieldName;
  };

  // 检查字段是否为必填
  const isFieldRequired = (fieldName: string): boolean => {
    const providerInfo = getSelectedProviderInfo();
    if (!providerInfo || providerInfo.required_keys === null) return false;
    return providerInfo.required_keys.includes(fieldName);
  };

  // 检查 provider 是否需要配置
  const providerNeedsConfig = (): boolean => {
    const providerInfo = getSelectedProviderInfo();
    if (!providerInfo) return true; // 默认需要配置
    return (providerInfo.required_keys !== null && providerInfo.required_keys.length > 0) ||
           (providerInfo.optional_keys !== null && providerInfo.optional_keys.length > 0);
  };

  // 获取所有需要显示的字段（必填 + 可选）
  const getConfigFields = (): string[] => {
    const providerInfo = getSelectedProviderInfo();
    if (!providerInfo) return [];
    const fields = new Set<string>();
    if (providerInfo.required_keys) {
      providerInfo.required_keys.forEach(k => fields.add(k));
    }
    if (providerInfo.optional_keys) {
      providerInfo.optional_keys.forEach(k => fields.add(k));
    }
    return Array.from(fields);
  };

  // Fetch LLM configs on mount
  useEffect(() => {
    fetchLLMConfigs();
  }, []);

  // Fetch configs based on active tab
  useEffect(() => {
    // 切换标签页时关闭添加/编辑表单
    setShowAddForm(false);
    setEditingId(null);
    resetForm();

    if (activeTab === 'youdao') {
      fetchYoudaoBindStatus();
    } else {
      fetchConfigs();
      fetchActiveProvider();
    }
  }, [activeTab]);

  // Fetch providers when active tab changes
  useEffect(() => {
    if (activeTab !== 'youdao') {
      fetchProviders();
    }
  }, [activeTab]);

  const fetchLLMConfigs = async () => {
    try {
      const res = await userConfigApi.listLLMConfigs();
      if (res && res.code === 0) {
        setLlmConfigs(res.data);
      }
    } catch (error) {
      console.error('Failed to fetch LLM configs:', error);
    }
  };

  const fetchProviders = async () => {
    try {
      const serviceType = activeTab === 'llm' ? 'llm' : activeTab;
      const providersList = await providersApi.getProvidersByType(serviceType);
      setProviders(providersList);
    } catch (error) {
      console.error('Failed to fetch providers:', error);
      setProviders([]);
    }
  };

  // 有道云配置相关函数
  const fetchYoudaoBindStatus = async () => {
    setYoudaoLoading(true);
    setYoudaoError(null);
    try {
      const res = await youdaoApi.getBindStatus();
      if (res.code === 0) {
        setYoudaoBindStatus(res.data);
      } else {
        setYoudaoError(res.message || '获取绑定状态失败');
      }
    } catch (error) {
      setYoudaoError('获取绑定状态失败');
      console.error('Failed to fetch youdao bind status:', error);
    } finally {
      setYoudaoLoading(false);
    }
  };

  const handleYoudaoBind = async () => {
    if (!youdaoApiKey.trim()) {
      setYoudaoError('请输入 API Key');
      return;
    }

    setYoudaoLoading(true);
    setYoudaoError(null);
    try {
      const res = await youdaoApi.bindApiKey(youdaoApiKey);
      if (res.code === 0) {
        setYoudaoApiKey('');
        fetchYoudaoBindStatus();
      } else {
        setYoudaoError(res.message || '绑定失败');
      }
    } catch (error) {
      setYoudaoError('绑定失败');
      console.error('Failed to bind youdao:', error);
    } finally {
      setYoudaoLoading(false);
    }
  };

  const handleYoudaoUnbind = async () => {
    setYoudaoLoading(true);
    setYoudaoError(null);
    try {
      const res = await youdaoApi.unbind();
      if (res.code === 0) {
        fetchYoudaoBindStatus();
      } else {
        setYoudaoError(res.message || '解绑失败');
      }
    } catch (error) {
      setYoudaoError('解绑失败');
      console.error('Failed to unbind youdao:', error);
    } finally {
      setYoudaoLoading(false);
    }
  };

  const fetchActiveProvider = async () => {
    try {
      const serviceType = activeTab === 'llm' ? 'llm' : activeTab;
      const res = await providersApi.getActiveConfig(serviceType);
      if (res && res.code === 0) {
        setActiveProvider(res.data);
      }
    } catch (error) {
      console.error('Failed to fetch active provider:', error);
      setActiveProvider(null);
    }
  };

  const fetchConfigs = async () => {
    setLoading(true);
    try {
      let res;
      switch (activeTab) {
        case 'llm':
          res = await userConfigApi.listLLMConfigs();
          break;
        case 'search':
          res = await userConfigApi.listSearchConfigs();
          break;
        case 'asr':
          res = await userConfigApi.listASRConfigs();
          break;
        case 'embedding':
          res = await userConfigApi.listEmbeddingConfigs();
          break;
      }
      if (res && res.code === 0) {
        if (activeTab === 'llm') {
          // LLM 配置有专门的类型，需要单独处理
          setLlmConfigs(res.data as UserLLMConfig[]);
          // 同步设置 configs，确保列表渲染正常
          setConfigs(res.data as unknown as UserConfig[]);
        } else {
          setConfigs(res.data as UserConfig[]);
        }
      }
    } catch (error) {
      console.error('Failed to fetch configs:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleAdd = async () => {
    setError(null);

    // 验证必填字段
    if (!formData.name.trim()) {
      setError('请输入配置名称');
      return;
    }
    if (!formData.provider) {
      setError('请选择服务商');
      return;
    }

    // 验证 provider 特定的必填字段
    const providerInfo = getSelectedProviderInfo();
    if (providerInfo && providerInfo.required_keys) {
      for (const key of providerInfo.required_keys) {
        // 先查 formData 直接属性，再查 extra_config
        let value = formData[key as keyof UserConfigRequest];
        if (value === undefined || value === null || value === '') {
          value = formData.extra_config?.[key];
        }
        if (!value || (typeof value === 'string' && !value.trim())) {
          const fieldNames: Record<string, string> = {
            api_key: 'API Key',
            api_url: 'API 地址',
            model: '模型名称',
            access_key_id: 'Access Key ID',
            access_key_secret: 'Access Key Secret',
            app_key: 'App Key'
          };
          setError(`请输入${fieldNames[key] || key}`);
          return;
        }
      }
    }

    setSaving(true);
    try {
      let res;
      switch (activeTab) {
        case 'llm':
          res = await userConfigApi.createLLMConfig(formData);
          break;
        case 'search':
          res = await userConfigApi.createSearchConfig(formData);
          break;
        case 'asr':
          res = await userConfigApi.createASRConfig(formData);
          break;
        case 'embedding':
          res = await userConfigApi.createEmbeddingConfig(formData);
          break;
      }
      if (res && res.code === 0) {
        setShowAddForm(false);
        resetForm();
        fetchConfigs();
        fetchActiveProvider();
      } else if (res && res.message) {
        // 如果后端返回了健康检查结果，展示详情
        const data = res.data as any;
        if (data && typeof data === 'object' && 'healthy' in data) {
          setTestResult(data);
        }
        setError(res.message);
      }
    } catch (error: any) {
      console.error('Failed to add config:', error);
      const errData = error?.response?.data;
      if (errData?.data && typeof errData.data === 'object' && 'healthy' in errData.data) {
        setTestResult(errData.data);
      }
      setError(getErrorMessage(error, '添加配置失败'));
    } finally {
      setSaving(false);
    }
  };

  const handleUpdate = async (id: number) => {
    // 验证必填字段
    if (!formData.name.trim()) {
      setError('请输入配置名称');
      return;
    }
    if (!formData.provider) {
      setError('请选择服务商');
      return;
    }

    // 验证 provider 特定的必填字段
    const providerInfo = getSelectedProviderInfo();
    if (providerInfo && providerInfo.required_keys) {
      for (const key of providerInfo.required_keys) {
        // 先查 formData 直接属性，再查 extra_config
        let value = formData[key as keyof UserConfigRequest];
        if (value === undefined || value === null || value === '') {
          value = formData.extra_config?.[key];
        }
        if (!value || (typeof value === 'string' && !value.trim())) {
          const fieldNames: Record<string, string> = {
            api_key: 'API Key',
            api_url: 'API 地址',
            model: '模型名称',
            access_key_id: 'Access Key ID',
            access_key_secret: 'Access Key Secret',
            app_key: 'App Key'
          };
          setError(`请输入${fieldNames[key] || key}`);
          return;
        }
      }
    }

    setSaving(true);
    try {
      let res;
      switch (activeTab) {
        case 'llm':
          res = await userConfigApi.updateLLMConfig(id, formData);
          break;
        case 'search':
          res = await userConfigApi.updateSearchConfig(id, formData);
          break;
        case 'asr':
          res = await userConfigApi.updateASRConfig(id, formData);
          break;
        case 'embedding':
          res = await userConfigApi.updateEmbeddingConfig(id, formData);
          break;
      }
      if (res && res.code === 0) {
        setEditingId(null);
        resetForm();
        fetchConfigs();
        fetchActiveProvider();
      } else if (res && res.message) {
        const data = res.data as any;
        if (data && typeof data === 'object' && 'healthy' in data) {
          setTestResult(data);
        }
        setError(res.message);
      }
    } catch (error: any) {
      console.error('Failed to update config:', error);
      const errData = error?.response?.data;
      if (errData?.data && typeof errData.data === 'object' && 'healthy' in errData.data) {
        setTestResult(errData.data);
      }
      setError(getErrorMessage(error, '更新配置失败'));
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async (id: number) => {
    // 如果是向量配置，显示确认弹窗
    if (activeTab === 'embedding') {
      setDeleteConfirmId(id);
      return;
    }

    setDeleting(true);
    try {
      let res;
      switch (activeTab) {
        case 'llm':
          res = await userConfigApi.deleteLLMConfig(id);
          break;
        case 'search':
          res = await userConfigApi.deleteSearchConfig(id);
          break;
        case 'asr':
          res = await userConfigApi.deleteASRConfig(id);
          break;
      }
      if (res && res.code === 0) {
        fetchConfigs();
        fetchActiveProvider();
      }
    } catch (error) {
      console.error('Failed to delete config:', error);
      setError('删除配置失败，请重试');
    } finally {
      setDeleting(false);
    }
  };

  // 确认删除向量配置（包含删除 Milvus Collection）
  const handleConfirmDeleteEmbedding = async () => {
    if (!deleteConfirmId) return;

    setDeleting(true);
    try {
      const res = await userConfigApi.deleteEmbeddingAndCollection(deleteConfirmId);
      if (res && res.code === 0) {
        setDeleteConfirmId(null);
        fetchConfigs();
        fetchActiveProvider();
      } else if (res && res.message) {
        setError(res.message);
        setDeleteConfirmId(null);
      }
    } catch (error) {
      console.error('Failed to delete embedding config:', error);
      setError('删除配置失败');
      setDeleteConfirmId(null);
    } finally {
      setDeleting(false);
    }
  };

  const handleTest = async () => {
    setError(null);
    setTestResult(null);

    if (!formData.provider) {
      setError('请先选择服务商');
      return;
    }

    setTesting(true);
    try {
      const res = await userConfigApi.testConfig(activeTab === 'llm' ? 'llm' : activeTab, formData);
      if (res.code === 0 && res.data) {
        setTestResult(res.data);
      } else {
        // 测试失败，也展示结果
        setTestResult(res.data || { healthy: false, message: res.message || '测试失败', latency_ms: 0 });
      }
    } catch (error: any) {
      const errData = error?.response?.data;
      if (errData?.data) {
        setTestResult(errData.data);
      } else {
        setTestResult({ healthy: false, message: getErrorMessage(error, '测试失败'), latency_ms: 0 });
      }
    } finally {
      setTesting(false);
    }
  };

  const resetForm = () => {
    setFormData({
      name: '',
      provider: '',
      api_key: '',
      api_url: '',
      model: '',
      daily_quota: 100,
      dimensions: 2048,
      extra_config: {},
    });
    setTestResult(null);
  };

  const startEdit = (config: UserConfig) => {
    setEditingId(config.id);
    let extraConfig: Record<string, any> = {};
    if (config.extra_config) {
      try {
        extraConfig = typeof config.extra_config === 'string'
          ? JSON.parse(config.extra_config)
          : config.extra_config;
      } catch {
        extraConfig = {};
      }
    }
    setFormData({
      name: config.name,
      provider: config.provider,
      api_key: config.api_key,
      api_url: config.api_url,
      model: config.model || '',
      daily_quota: config.daily_quota || 100,
      dimensions: config.dimensions || 2048,
      extra_config: extraConfig,
    });
  };

  const tabs = [
    { key: 'search', label: '搜索引擎', icon: Search },
    { key: 'asr', label: '语音识别', icon: Mic },
    { key: 'youdao', label: '有道云笔记', icon: BookOpen },
  ];

  // 从 API 获取的动态 provider 列表（只返回已实现的）
  const getProviderOptions = (): { value: string; label: string }[] => {
    if (providers.length > 0) {
      return providers.map(p => ({
        value: p.provider,
        label: p.display_name,
      }));
    }
    return [];
  };

  const providerOptions = getProviderOptions() ?? [];

  return (
    <div className="h-full overflow-y-auto bg-bg-primary">
      <div className="max-w-4xl mx-auto px-8 py-8">
        {/* Header */}
        <div className="flex items-center gap-3 mb-8">
          <button
            onClick={() => navigate(-1)}
            className="p-2 rounded-lg text-text-muted hover:text-text-primary hover:bg-bg-hover transition-colors cursor-pointer"
          >
            <ArrowLeft size={18} />
          </button>
          <Settings size={22} className="text-accent" />
          <h1 className="text-xl font-bold text-text-primary">设置</h1>
        </div>

        {/* Error message */}
        {error && (
          <motion.div
            initial={{ opacity: 0, y: -8 }}
            animate={{ opacity: 1, y: 0 }}
            className="mb-6 p-4 rounded-xl bg-error/5 border border-error/20 flex items-center gap-3"
          >
            <AlertCircle size={18} className="text-error flex-shrink-0" />
            <div className="flex-1">
              <p className="text-sm text-error font-medium">{error}</p>
            </div>
            <button
              onClick={() => setError(null)}
              className="text-text-muted hover:text-text-primary cursor-pointer"
            >
              <X size={16} />
            </button>
          </motion.div>
        )}

        {/* LLM not configured warning */}
        {activeTab !== 'llm' && !loading && llmConfigs.length === 0 && (
          <motion.div
            initial={{ opacity: 0, y: -8 }}
            animate={{ opacity: 1, y: 0 }}
            className="mb-6 p-4 rounded-xl bg-warning/5 border border-warning/20 flex items-center gap-3"
          >
            <AlertCircle size={18} className="text-warning flex-shrink-0" />
            <div>
              <p className="text-sm text-warning font-medium">请先配置基础模型</p>
              <p className="text-xs text-text-muted mt-0.5">
                所有 AI 功能（搜索、语音识别等）都需要依赖基础大语言模型，请先在"基础模型"标签页完成配置
              </p>
            </div>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setActiveTab('llm')}
              className="ml-auto"
            >
              去配置
            </Button>
          </motion.div>
        )}

        {/* Tabs */}
        <div className="flex gap-1 mb-6 bg-bg-tertiary rounded-lg p-1 w-fit">
          <button
            onClick={() => {
              setActiveTab('llm');
              setShowAddForm(false);
              setEditingId(null);
            }}
            className={cn(
              'flex items-center gap-2 px-4 py-2 rounded-md text-sm font-medium transition-all cursor-pointer',
              activeTab === 'llm' ? 'bg-accent text-white' : 'text-text-muted hover:text-text-primary'
            )}
          >
            <Cpu size={14} /> 基础模型
            <span className="text-xs opacity-70">必需</span>
          </button>
          <button
            onClick={() => {
              setActiveTab('embedding');
              setShowAddForm(false);
              setEditingId(null);
            }}
            className={cn(
              'flex items-center gap-2 px-4 py-2 rounded-md text-sm font-medium transition-all cursor-pointer',
              activeTab === 'embedding' ? 'bg-accent text-white' : 'text-text-muted hover:text-text-primary'
            )}
          >
            <Database size={14} /> 向量化
            <span className="text-xs opacity-70">必需</span>
          </button>
          {tabs.map((tab) => (
            <button
              key={tab.key}
              onClick={() => {
                setActiveTab(tab.key as ConfigTab);
                setShowAddForm(false);
                setEditingId(null);
              }}
              className={cn(
                'flex items-center gap-2 px-4 py-2 rounded-md text-sm font-medium transition-all cursor-pointer',
                activeTab === tab.key ? 'bg-accent text-white' : 'text-text-muted hover:text-text-primary'
              )}
            >
              <tab.icon size={14} /> {tab.label}
            </button>
          ))}
        </div>

        {/* 当前生效的服务 - 仅搜索和语音识别有系统默认配置 */}
        {activeProvider && (activeTab === 'search' || activeTab === 'asr') && (
          <div className="mb-4 p-4 rounded-xl bg-accent/5 border border-accent/20">
            <div className="flex items-center gap-2">
              <span className="text-xs text-text-muted">当前使用:</span>
              <span className="text-sm font-semibold text-accent">
                {activeProvider.display_name}
              </span>
              <span className="text-xs text-text-muted">
                ({activeProvider.source === 'user' ? '用户配置' : '系统默认'})
              </span>
            </div>
          </div>
        )}

        {/* 向量配置特殊提示 */}
        {activeTab === 'embedding' && configs.length > 0 && (
          <div className="mb-4 p-3 rounded-xl bg-warning/5 border border-warning/20">
            <p className="text-xs text-warning">
              ⚠️ 建议配置好向量模型之后不要再进行更换。更换向量模型将导致原有知识库不可用，需要重新导入所有资料。
            </p>
          </div>
        )}

        {/* Config List */}
        {activeTab === 'youdao' ? (
          /* 有道云配置 */
          <div className="space-y-4">
            {youdaoLoading ? (
              <div className="text-center py-8 text-text-muted">加载中...</div>
            ) : youdaoError ? (
              <div className="p-4 rounded-xl bg-error/5 border border-error/20 flex items-center gap-3">
                <AlertCircle size={18} className="text-error flex-shrink-0" />
                <div className="flex-1">
                  <p className="text-sm text-error font-medium">{youdaoError}</p>
                </div>
                <button
                  onClick={() => setYoudaoError(null)}
                  className="text-text-muted hover:text-text-primary cursor-pointer"
                >
                  <X size={16} />
                </button>
              </div>
            ) : youdaoBindStatus?.bound ? (
              /* 已绑定状态 */
              <motion.div
                initial={{ opacity: 0, y: 10 }}
                animate={{ opacity: 1, y: 0 }}
                className="bg-bg-card rounded-xl border border-border-light p-5"
              >
                <div className="flex items-center justify-between mb-4">
                  <div className="flex items-center gap-3">
                    <div className="w-10 h-10 rounded-lg bg-success/10 flex items-center justify-center">
                      <BookOpen size={20} className="text-success" />
                    </div>
                    <div>
                      <h3 className="text-sm font-semibold text-text-primary">有道云笔记已绑定</h3>
                      <p className="text-xs text-text-muted">状态: {youdaoBindStatus.status || '活跃'}</p>
                    </div>
                  </div>
                  <Badge variant="success">已绑定</Badge>
                </div>
                <div className="flex justify-end">
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={handleYoudaoUnbind}
                    disabled={youdaoLoading}
                  >
                    {youdaoLoading ? '解绑中...' : '解绑账号'}
                  </Button>
                </div>
              </motion.div>
            ) : (
              /* 未绑定状态 */
              <motion.div
                initial={{ opacity: 0, y: 10 }}
                animate={{ opacity: 1, y: 0 }}
                className="bg-bg-card rounded-xl border border-border-light p-5"
              >
                <div className="flex items-center gap-3 mb-4">
                  <div className="w-10 h-10 rounded-lg bg-accent/10 flex items-center justify-center">
                    <BookOpen size={20} className="text-accent" />
                  </div>
                  <div>
                    <h3 className="text-sm font-semibold text-text-primary">绑定有道云笔记</h3>
                    <p className="text-xs text-text-muted">绑定后即可导入有道云笔记到当前系统</p>
                  </div>
                </div>
                <div className="space-y-4">
                  <Input
                    label="API Key"
                    type="password"
                    placeholder="请输入有道云笔记 API Key"
                    value={youdaoApiKey}
                    onChange={(e) => setYoudaoApiKey(e.target.value)}
                  />
                  <div className="flex justify-end gap-2">
                    <Button
                      size="sm"
                      onClick={handleYoudaoBind}
                      disabled={youdaoLoading || !youdaoApiKey.trim()}
                    >
                      {youdaoLoading ? '绑定中...' : '绑定账号'}
                    </Button>
                  </div>
                </div>
                <div className="mt-4 p-3 bg-bg-tertiary rounded-lg">
                  <p className="text-xs text-text-muted">
                    💡 如何获取 API Key：登录有道云笔记网页版，在设置中找到 API Key 选项
                  </p>
                </div>
              </motion.div>
            )}
          </div>
        ) : loading ? (
          <div className="text-center py-8 text-text-muted">加载中...</div>
        ) : configs.length === 0 ? (
          <div className="text-center py-8 text-text-muted">暂无配置，点击下方按钮添加</div>
        ) : (
          <div className="space-y-4">
            {configs.map((config) => (
              <motion.div
                key={config.id}
                layout
                className="bg-bg-card rounded-xl border border-border-light p-5"
              >
                {editingId === config.id ? (
                  /* Edit/View Form */
                  <div className="space-y-4">
                    {/* 向量模型配置只读提示 */}
                    {activeTab === 'embedding' && (
                      <div className="p-3 bg-warning/5 border border-warning/20 rounded-lg">
                        <p className="text-xs text-warning">
                          ⚠️ 向量模型配置不可修改，如需更换请删除后重新配置
                        </p>
                      </div>
                    )}
                    <div className="grid grid-cols-2 gap-4">
                      <Input
                        label="配置名称"
                        value={formData.name}
                        disabled={activeTab === 'embedding'}
                        onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                      />
                      <div>
                        <label className="block text-sm font-medium text-text-primary mb-1.5">
                          服务商
                        </label>
                        <select
                          value={formData.provider}
                          disabled={activeTab === 'embedding'}
                          onChange={(e) => {
                            const newProvider = e.target.value;
                            setFormData({
                              ...formData,
                              provider: newProvider,
                              api_url: DEFAULT_API_URLS[newProvider] || '',
                              api_key: '',
                              model: '',
                              extra_config: {},
                            });
                          }}
                          className={cn(
                            "w-full h-10 px-3 rounded-lg bg-bg-tertiary border border-border-light text-sm focus:outline-none focus:border-accent",
                            activeTab === 'embedding' && "opacity-60 cursor-not-allowed"
                          )}
                        >
                          <option value="">选择服务商</option>
                          {providerOptions.map((opt) => (
                            <option key={opt.value} value={opt.value}>{opt.label}</option>
                          ))}
                        </select>
                      </div>
                    </div>
                    {providerNeedsConfig() ? (
                      <>
                        {getConfigFields().map(field => {
                          const required = isFieldRequired(field);
                          const label = getFieldLabel(field);
                          const isPassword = field.includes('key') || field.includes('secret');

                          // 获取当前值
                          const getValue = () => {
                            if (field === 'api_key') return formData.api_key;
                            if (field === 'api_url') return formData.api_url;
                            if (field === 'model') return formData.model;
                            if (field === 'dimensions') return formData.dimensions;
                            if (field === 'daily_quota') return formData.daily_quota;
                            return formData.extra_config?.[field] ?? '';
                          };

                          // 设置值
                          const setValue = (val: string) => {
                            if (field === 'api_key' || field === 'api_url' || field === 'model') {
                              setFormData({ ...formData, [field]: val });
                            } else if (field === 'dimensions' || field === 'daily_quota') {
                              setFormData({ ...formData, [field]: parseInt(val) || (field === 'dimensions' ? 2048 : 100) });
                            } else {
                              setFormData({
                                ...formData,
                                extra_config: { ...(formData.extra_config || {}), [field]: val },
                              });
                            }
                          };

                          return (
                            <Input
                              key={field}
                              label={required ? `${label} *` : `${label} (可选)`}
                              type={isPassword ? 'password' : field === 'dimensions' || field === 'daily_quota' ? 'number' : 'text'}
                              value={getValue()}
                              disabled={activeTab === 'embedding'}
                              onChange={(e) => setValue(e.target.value)}
                            />
                          );
                        })}
                      </>
                    ) : (
                      <div className="p-3 bg-bg-tertiary rounded-lg">
                        <p className="text-sm text-text-muted">
                          ✨ {getSelectedProviderInfo()?.display_name || formData.provider} 无需任何配置即可使用
                        </p>
                      </div>
                    )}

                    {/* 测试结果展示 - 编辑模式 */}
                    {testResult && (
                      <div className={cn(
                        'p-3 rounded-lg flex items-start gap-2 text-sm',
                        testResult.healthy
                          ? 'bg-success/5 border border-success/20 text-success'
                          : 'bg-error/5 border border-error/20 text-error'
                      )}>
                        {testResult.healthy ? <Check size={16} className="mt-0.5 flex-shrink-0" /> : <AlertCircle size={16} className="mt-0.5 flex-shrink-0" />}
                        <div>
                          <p className="font-medium">{testResult.message}</p>
                          {testResult.latency_ms > 0 && (
                            <p className="text-xs opacity-70 mt-0.5">耗时 {testResult.latency_ms}ms</p>
                          )}
                          {testResult.detail && (
                            <p className="text-xs opacity-70 mt-0.5 break-all">{testResult.detail}</p>
                          )}
                        </div>
                      </div>
                    )}

                    <div className="flex justify-end gap-2">
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => {
                          setEditingId(null);
                          resetForm();
                        }}
                      >
                        <X size={14} /> {activeTab === 'embedding' ? '关闭' : '取消'}
                      </Button>
                      {/* 非向量模型配置才显示测试和保存按钮 */}
                      {activeTab !== 'embedding' && (
                        <>
                          <Button
                            variant="ghost"
                            size="sm"
                            onClick={handleTest}
                            disabled={testing || !formData.provider}
                          >
                            {testing ? <Loader2 size={14} className="animate-spin" /> : <Plug size={14} />}
                            {testing ? '测试中...' : '测试连接'}
                          </Button>
                          <Button
                            size="sm"
                            onClick={() => handleUpdate(config.id)}
                            disabled={saving}
                          >
                            {saving ? <Loader2 size={14} className="animate-spin" /> : <Save size={14} />}
                            {saving ? '验证中...' : '保存'}
                          </Button>
                        </>
                      )}
                    </div>
                  </div>
                ) : (
                  /* Display Mode */
                  <>
                    <div className="flex items-center justify-between mb-4">
                      <div className="flex items-center gap-3">
                        <div className={cn(
                          'w-2.5 h-2.5 rounded-full transition-all duration-300',
                          config.enabled ? 'bg-success animate-pulse' : 'bg-text-muted'
                        )} />
                        <h3 className="text-sm font-semibold text-text-primary">{config.name}</h3>
                        <Badge variant={config.enabled ? 'success' : 'default'}>
                          {config.enabled ? '已启用' : '已禁用'}
                        </Badge>
                      </div>
                      <div className="flex items-center gap-2">
                        <button
                          onClick={() => startEdit(config)}
                          className="p-1.5 rounded-lg text-text-muted hover:text-accent hover:bg-accent/5 transition-colors cursor-pointer"
                        >
                          <Settings size={14} />
                        </button>
                        <button
                          onClick={() => handleDelete(config.id)}
                          className="p-1.5 rounded-lg text-text-muted hover:text-error hover:bg-error/5 transition-colors cursor-pointer"
                        >
                          <Trash2 size={14} />
                        </button>
                      </div>
                    </div>

                    <div className="flex items-center gap-2">
                      <p className="text-xs text-text-muted">服务商:</p>
                      <p className="text-sm text-text-primary font-medium">
                        {providers.find(p => p.provider === config.provider)?.display_name || config.provider}
                      </p>
                    </div>
                  </>
                )}
              </motion.div>
            ))}
          </div>
        )}

        {/* Add Config - 不为有道云标签页显示 */}
        {activeTab !== 'youdao' && (
          showAddForm ? (
            <motion.div
              initial={{ opacity: 0, height: 0 }}
              animate={{ opacity: 1, height: 'auto' }}
              className="bg-bg-card rounded-xl border border-accent/30 p-5 mt-4"
            >
              <h3 className="text-sm font-semibold text-text-primary mb-4">
                新增{activeTab === 'llm' ? '基础模型' : tabs.find(t => t.key === activeTab)?.label}配置
              </h3>
              <div className="space-y-4">
                <div className="grid grid-cols-2 gap-4">
                  <Input
                    label="配置名称"
                    placeholder={activeTab === 'llm' ? '如：我的 OpenAI' : '如：我的搜索引擎'}
                    value={formData.name}
                    onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                  />
                  <div>
                    <label className="block text-sm font-medium text-text-primary mb-1.5">
                      服务商
                    </label>
                    <select
                      value={formData.provider}
                      onChange={(e) => {
                        const newProvider = e.target.value;
                        setFormData({
                          ...formData,
                          provider: newProvider,
                          api_url: DEFAULT_API_URLS[newProvider] || '',
                          api_key: '',
                          model: '',
                          extra_config: {},
                        });
                      }}
                      className="w-full h-10 px-3 rounded-lg bg-bg-tertiary border border-border-light text-sm focus:outline-none focus:border-accent"
                    >
                      <option value="">选择服务商</option>
                      {(providerOptions ?? []).map((opt) => (
                        <option key={opt.value} value={opt.value}>{opt.label}</option>
                      ))}
                    </select>
                  </div>
                </div>
                {formData.provider && providerNeedsConfig() ? (
                  <>
                    {getConfigFields().map(field => {
                      const required = isFieldRequired(field);
                      const label = getFieldLabel(field);
                      const isPassword = field.includes('key') || field.includes('secret');

                      // 获取当前值
                      const getValue = () => {
                        if (field === 'api_key') return formData.api_key;
                        if (field === 'api_url') return formData.api_url;
                        if (field === 'model') return formData.model;
                        if (field === 'dimensions') return formData.dimensions;
                        if (field === 'daily_quota') return formData.daily_quota;
                        return formData.extra_config?.[field] ?? '';
                      };

                      // 设置值
                      const setValue = (val: string) => {
                        if (field === 'api_key' || field === 'api_url' || field === 'model') {
                          setFormData({ ...formData, [field]: val });
                        } else if (field === 'dimensions' || field === 'daily_quota') {
                          setFormData({ ...formData, [field]: parseInt(val) || (field === 'dimensions' ? 2048 : 100) });
                        } else {
                          setFormData({
                            ...formData,
                            extra_config: { ...(formData.extra_config || {}), [field]: val },
                          });
                        }
                      };

                      return (
                        <Input
                          key={field}
                          label={required ? `${label} *` : `${label} (可选)`}
                          type={isPassword ? 'password' : field === 'dimensions' || field === 'daily_quota' ? 'number' : 'text'}
                          placeholder={field === 'model' ? '输入模型名称' : `输入${label}`}
                          value={getValue()}
                          onChange={(e) => setValue(e.target.value)}
                        />
                      );
                    })}
                  </>
                ) : formData.provider ? (
                  <div className="p-3 bg-bg-tertiary rounded-lg">
                    <p className="text-sm text-text-muted">
                      ✨ {getSelectedProviderInfo()?.display_name || formData.provider} 无需任何配置即可使用
                    </p>
                  </div>
                ) : null}
              </div>
              {/* 测试结果展示 */}
              {testResult && (
                <div className={cn(
                  'p-3 rounded-lg flex items-start gap-2 text-sm',
                  testResult.healthy
                    ? 'bg-success/5 border border-success/20 text-success'
                    : 'bg-error/5 border border-error/20 text-error'
                )}>
                  {testResult.healthy ? <Check size={16} className="mt-0.5 flex-shrink-0" /> : <AlertCircle size={16} className="mt-0.5 flex-shrink-0" />}
                  <div>
                    <p className="font-medium">{testResult.message}</p>
                    {testResult.latency_ms > 0 && (
                      <p className="text-xs opacity-70 mt-0.5">耗时 {testResult.latency_ms}ms</p>
                    )}
                    {testResult.detail && (
                      <p className="text-xs opacity-70 mt-0.5 break-all">{testResult.detail}</p>
                    )}
                  </div>
                </div>
              )}

              <div className="flex justify-end gap-2 mt-4">
                <Button variant="ghost" size="sm" onClick={() => { setShowAddForm(false); resetForm(); }}>
                  取消
                </Button>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={handleTest}
                  disabled={testing || !formData.provider}
                >
                  {testing ? <Loader2 size={14} className="animate-spin" /> : <Plug size={14} />}
                  {testing ? '测试中...' : '测试连接'}
                </Button>
                <Button size="sm" onClick={handleAdd} disabled={saving}>
                  {saving ? <Loader2 size={14} className="animate-spin" /> : <Check size={14} />}
                  {saving ? '验证中...' : '添加'}
                </Button>
              </div>
            </motion.div>
          ) : (
            // LLM 类型允许多个配置，其他类型只能配置一个
            (activeTab === 'llm' || configs.length === 0) && (
              <button
                onClick={() => setShowAddForm(true)}
                className="w-full flex items-center justify-center gap-2 py-3 rounded-xl border border-dashed border-border-light text-text-muted hover:text-accent hover:border-accent/40 transition-all cursor-pointer mt-4"
              >
                <Plus size={16} /> 添加{activeTab === 'llm' ? '基础模型' : tabs.find(t => t.key === activeTab)?.label}配置
              </button>
            )
          )
        )}

        {/* 删除向量配置确认弹窗 */}
        <AnimatePresence>
          {deleteConfirmId && (
            <>
              <div className="fixed inset-0 bg-black/50 z-50" onClick={() => !deleting && setDeleteConfirmId(null)} />
              <motion.div
                initial={{ opacity: 0, scale: 0.95 }}
                animate={{ opacity: 1, scale: 1 }}
                exit={{ opacity: 0, scale: 0.95 }}
                className="fixed top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 z-50 w-[420px] bg-bg-card rounded-2xl border border-border-light shadow-2xl p-6"
              >
                <div className="flex items-center gap-3 mb-4">
                  <div className="w-10 h-10 rounded-full bg-error/10 flex items-center justify-center">
                    <AlertCircle size={20} className="text-error" />
                  </div>
                  <h3 className="text-lg font-semibold text-text-primary">确认删除向量模型</h3>
                </div>
                <div className="mb-6 p-4 rounded-xl bg-warning/5 border border-warning/20">
                  <p className="text-sm text-warning font-medium mb-2">⚠️ 此操作不可逆！</p>
                  <p className="text-sm text-text-secondary">
                    更换向量模型将导致原有知识库不可用，需要重新导入。删除后，系统将自动清除该账号下的所有向量数据。
                  </p>
                </div>
                <div className="flex justify-end gap-3">
                  <Button
                    variant="ghost"
                    onClick={() => setDeleteConfirmId(null)}
                    disabled={deleting}
                  >
                    取消
                  </Button>
                  <Button
                    variant="danger"
                    onClick={handleConfirmDeleteEmbedding}
                    disabled={deleting}
                  >
                    {deleting ? (
                      <>
                        <Loader2 size={14} className="animate-spin" />
                        删除中...
                      </>
                    ) : (
                      '确认删除'
                    )}
                  </Button>
                </div>
              </motion.div>
            </>
          )}
        </AnimatePresence>
      </div>
    </div>
  );
}
