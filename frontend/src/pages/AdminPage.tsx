import { useState, useEffect } from 'react';
import { motion } from 'framer-motion';
import {
  Users, Settings, Shield, Search, Plus, ToggleLeft, ToggleRight,
  Check, ArrowLeft, Edit2, Save, X, Trash2, AlertCircle
} from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import { cn } from '../utils/cn';
import Button from '../components/ui/Button';
import Input from '../components/ui/Input';
import Badge from '../components/ui/Badge';
import AvatarImg from '../components/ui/AvatarImg';
import * as adminApi from '../api/admin';
import * as providersApi from '../api/providers';
import type { AdminUser, SysConfig, ConfigStatus } from '../api/admin';
import type { ProviderInfo } from '../api/providers';

export default function AdminPage() {
  const navigate = useNavigate();
  const [activeTab, setActiveTab] = useState<'users' | 'configs'>('users');

  return (
    <div className="h-full overflow-y-auto bg-bg-primary">
      <div className="max-w-6xl mx-auto px-8 py-8">
        {/* Header */}
        <div className="flex items-center gap-3 mb-8">
          <button
            onClick={() => navigate(-1)}
            className="p-2 rounded-lg text-text-muted hover:text-text-primary hover:bg-bg-hover transition-colors cursor-pointer"
          >
            <ArrowLeft size={18} />
          </button>
          <Shield size={22} className="text-accent" />
          <h1 className="text-xl font-bold text-text-primary">后台管理</h1>
        </div>

        {/* Tabs */}
        <div className="flex gap-1 mb-6 bg-bg-tertiary rounded-lg p-1 w-fit">
          <button
            onClick={() => setActiveTab('users')}
            className={cn(
              'flex items-center gap-2 px-4 py-2 rounded-md text-sm font-medium transition-all cursor-pointer',
              activeTab === 'users' ? 'bg-accent text-white' : 'text-text-muted hover:text-text-primary'
            )}
          >
            <Users size={14} /> 用户管理
          </button>
          <button
            onClick={() => setActiveTab('configs')}
            className={cn(
              'flex items-center gap-2 px-4 py-2 rounded-md text-sm font-medium transition-all cursor-pointer',
              activeTab === 'configs' ? 'bg-accent text-white' : 'text-text-muted hover:text-text-primary'
            )}
          >
            <Settings size={14} /> 系统配置
          </button>
        </div>

        {/* Content */}
        {activeTab === 'users' && <UserManagement />}
        {activeTab === 'configs' && <ConfigManagement />}
      </div>
    </div>
  );
}

// ===== User Management Component =====

function UserManagement() {
  const [users, setUsers] = useState<AdminUser[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [keyword, setKeyword] = useState('');
  const [loading, setLoading] = useState(false);

  const fetchUsers = async () => {
    setLoading(true);
    try {
      const res = await adminApi.listUsers({ keyword, page, size: 20 });
      if (res.code === 0) {
        setUsers(res.data.list);
        setTotal(res.data.total);
      }
    } catch (error) {
      console.error('Failed to fetch users:', error);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchUsers();
  }, [page, keyword]);

  const handleToggleUser = async (userId: number, enabled: boolean) => {
    try {
      const res = await adminApi.updateUserStatus(userId, enabled);
      if (res.code === 0) {
        setUsers(users.map(u => u.id === userId ? { ...u, enabled } : u));
      }
    } catch (error) {
      console.error('Failed to update user status:', error);
    }
  };

  return (
    <div className="space-y-4">
      {/* Search */}
      <div className="flex gap-4 mb-6">
        <div className="flex-1">
          <Input
            placeholder="搜索用户名或邮箱..."
            value={keyword}
            onChange={(e) => setKeyword(e.target.value)}
          />
        </div>
        <Button onClick={fetchUsers}>
          <Search size={16} /> 搜索
        </Button>
      </div>

      {/* User List */}
      {loading ? (
        <div className="text-center py-8 text-text-muted">加载中...</div>
      ) : users.length === 0 ? (
        <div className="text-center py-8 text-text-muted">暂无用户</div>
      ) : (
        <div className="space-y-3">
          {users.map((user) => (
            <motion.div
              key={user.id}
              layout
              className="bg-bg-card rounded-xl border border-border-light p-5"
            >
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-4">
                  <div className="w-10 h-10 rounded-full bg-accent/10 flex items-center justify-center">
                    {user.avatar ? (
                      <AvatarImg
                        src={user.avatar}
                        className="w-10 h-10 rounded-full"
                        fallback={
                          <span className="text-accent font-semibold">
                            {user.nickname?.[0] || user.username[0]}
                          </span>
                        }
                      />
                    ) : (
                      <span className="text-accent font-semibold">
                        {user.nickname?.[0] || user.username[0]}
                      </span>
                    )}
                  </div>
                  <div>
                    <div className="flex items-center gap-2">
                      <h3 className="text-sm font-semibold text-text-primary">
                        {user.nickname || user.username}
                      </h3>
                      <Badge variant={user.role === 'admin' ? 'warning' : 'default'}>
                        {user.role === 'admin' ? '管理员' : '用户'}
                      </Badge>
                      <Badge variant={user.enabled ? 'success' : 'error'}>
                        {user.enabled ? '正常' : '禁用'}
                      </Badge>
                    </div>
                    <p className="text-xs text-text-muted mt-1">{user.email}</p>
                  </div>
                </div>

                <div className="flex items-center gap-2">
                  <button
                    onClick={() => handleToggleUser(user.id, !user.enabled)}
                    className="cursor-pointer"
                    title={user.enabled ? '禁用用户' : '启用用户'}
                  >
                    {user.enabled ? (
                      <ToggleRight size={24} className="text-success" />
                    ) : (
                      <ToggleLeft size={24} className="text-text-muted" />
                    )}
                  </button>
                </div>
              </div>

              <div className="grid grid-cols-3 gap-4 mt-4 pt-4 border-t border-border-light">
                <div>
                  <p className="text-xs text-text-muted mb-1">用户名</p>
                  <p className="text-sm text-text-primary">{user.username}</p>
                </div>
                <div>
                  <p className="text-xs text-text-muted mb-1">注册时间</p>
                  <p className="text-sm text-text-primary">
                    {new Date(user.created_at).toLocaleDateString()}
                  </p>
                </div>
                <div>
                  <p className="text-xs text-text-muted mb-1">最后更新</p>
                  <p className="text-sm text-text-primary">
                    {new Date(user.updated_at).toLocaleDateString()}
                  </p>
                </div>
              </div>
            </motion.div>
          ))}
        </div>
      )}

      {/* Pagination */}
      {total > 20 && (
        <div className="flex justify-center gap-2 mt-6">
          <Button
            variant="ghost"
            size="sm"
            disabled={page === 1}
            onClick={() => setPage(page - 1)}
          >
            上一页
          </Button>
          <span className="flex items-center px-4 text-sm text-text-muted">
            第 {page} 页 / 共 {Math.ceil(total / 20)} 页
          </span>
          <Button
            variant="ghost"
            size="sm"
            disabled={page >= Math.ceil(total / 20)}
            onClick={() => setPage(page + 1)}
          >
            下一页
          </Button>
        </div>
      )}
    </div>
  );
}

// ===== Config Management Component =====

function ConfigManagement() {
  const [configStatus, setConfigStatus] = useState<ConfigStatus['groups']>([]);
  const [selectedGroup, setSelectedGroup] = useState<string>('search');
  const [configs, setConfigs] = useState<SysConfig[]>([]);
  const [providers, setProviders] = useState<ProviderInfo[]>([]);
  const [loading, setLoading] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [showAddForm, setShowAddForm] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  // 编辑表单状态
  const [editFormData, setEditFormData] = useState<Record<string, string>>({});

  // 新增表单状态
  const [newFormData, setNewFormData] = useState({
    provider: '',
    fields: {} as Record<string, string>,
  });

  // 固定的 2 个 group（不包含 llm、embedding 和 document）
  // document（文档转换）作为基础功能，始终从 config.yaml 读取，不需要在后台管理
  const allGroups = ['search', 'asr'] as const;
  const groupLabels: Record<string, string> = {
    search: '搜索引擎',
    asr: '语音识别',
  };

  // Fetch config status on mount
  useEffect(() => {
    const fetchStatus = async () => {
      try {
        const res = await adminApi.getConfigStatus();
        if (res.code === 0) {
          setConfigStatus(res.data.groups);
        }
      } catch (error) {
        console.error('Failed to fetch config status:', error);
      }
    };
    fetchStatus();
  }, []);

  // Fetch configs and providers when group changes
  useEffect(() => {
    // 切换服务类型时关闭添加/编辑表单
    setShowAddForm(false);
    setEditingId(null);
    setNewFormData({ provider: '', fields: {} });
    setEditFormData({});

    fetchConfigsAndProviders();
  }, [selectedGroup]);

  const fetchConfigsAndProviders = async () => {
    setLoading(true);
    try {
      const [configsRes, providersList] = await Promise.all([
        adminApi.getConfigs(selectedGroup),
        providersApi.getProvidersByType(selectedGroup),
      ]);
      if (configsRes.code === 0) {
        setConfigs(configsRes.data);
      }
      setProviders(providersList);
    } catch (error) {
      console.error('Failed to fetch configs:', error);
    } finally {
      setLoading(false);
    }
  };

  // 获取 provider 信息
  const getProviderInfo = (providerName: string): ProviderInfo | undefined => {
    return providers.find(p => p.provider === providerName);
  };

  // 获取字段标签
  const getFieldLabel = (providerName: string, fieldName: string): string => {
    const providerInfo = getProviderInfo(providerName);
    if (providerInfo?.key_labels?.[fieldName]) {
      return providerInfo.key_labels[fieldName];
    }
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

  // 解析 config_value JSON
  const parseConfigValue = (value: string): Record<string, any> => {
    try {
      return JSON.parse(value);
    } catch {
      return {};
    }
  };

  // 校验必填字段
  const validateRequiredFields = (providerName: string, fields: Record<string, string>): string | null => {
    const providerInfo = getProviderInfo(providerName);
    if (!providerInfo || !providerInfo.required_keys) return null;
    for (const key of providerInfo.required_keys) {
      const value = fields[key];
      if (!value || !value.trim()) {
        const label = providerInfo.key_labels?.[key] || key;
        return `${label}不能为空`;
      }
    }
    return null;
  };

  // 获取配置的所有字段
  const getConfigFields = (providerName: string): string[] => {
    const providerInfo = getProviderInfo(providerName);
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

  // 获取 group 的统计信息
  const getGroupStatus = (group: string) => {
    return configStatus.find(s => s.group === group);
  };

  const handleToggleConfig = async (config: SysConfig) => {
    try {
      const res = await adminApi.updateConfig(
        config.config_group,
        config.config_key,
        config.config_value,
        !config.enabled
      );
      if (res.code === 0) {
        setConfigs(configs.map(c =>
          c.id === config.id ? { ...c, enabled: !c.enabled } : c
        ));
      }
    } catch (error) {
      console.error('Failed to toggle config:', error);
    }
  };

  const handleStartEdit = (config: SysConfig) => {
    setEditingId(config.id);
    const parsed = parseConfigValue(config.config_value);
    setEditFormData(parsed);
  };

  const handleSaveEdit = async (config: SysConfig) => {
    setError(null);

    // 校验必填字段
    const validationError = validateRequiredFields(config.config_key, editFormData);
    if (validationError) {
      setError(validationError);
      return;
    }

    try {
      const configValue = JSON.stringify(editFormData);
      const res = await adminApi.updateConfig(
        config.config_group,
        config.config_key,
        configValue,
        config.enabled
      );
      if (res.code === 0) {
        setConfigs(configs.map(c =>
          c.id === config.id ? { ...c, config_value: configValue } : c
        ));
        setEditingId(null);
        setEditFormData({});
      } else if (res.message) {
        setError(res.message);
      }
    } catch (error: any) {
      console.error('Failed to update config:', error);
      const errData = error?.response?.data;
      setError(errData?.message || '更新配置失败');
    }
  };

  const handleDeleteConfig = async (config: SysConfig) => {
    const providerInfo = getProviderInfo(config.config_key);
    const displayName = providerInfo?.display_name || config.description || config.config_key;
    if (!window.confirm(`确定要删除「${displayName}」配置吗？`)) return;

    try {
      const res = await adminApi.deleteConfig(config.config_group, config.config_key);
      if (res.code === 0) {
        setConfigs(configs.filter(c => c.id !== config.id));
        // 刷新状态统计
        const statusRes = await adminApi.getConfigStatus();
        if (statusRes.code === 0) {
          setConfigStatus(statusRes.data.groups);
        }
      }
    } catch (error) {
      console.error('Failed to delete config:', error);
    }
  };

  const handleAddConfig = async () => {
    setError(null);

    if (!newFormData.provider) {
      setError('请选择服务商');
      return;
    }

    // 校验必填字段
    const validationError = validateRequiredFields(newFormData.provider, newFormData.fields);
    if (validationError) {
      setError(validationError);
      return;
    }

    setSaving(true);
    try {
      const configValue = JSON.stringify(newFormData.fields);
      const res = await adminApi.addConfig(
        selectedGroup,
        newFormData.provider,
        configValue,
        getProviderInfo(newFormData.provider)?.display_name || newFormData.provider
      );
      if (res.code === 0) {
        setShowAddForm(false);
        setNewFormData({ provider: '', fields: {} });
        fetchConfigsAndProviders();
        // 刷新状态统计
        const statusRes = await adminApi.getConfigStatus();
        if (statusRes.code === 0) {
          setConfigStatus(statusRes.data.groups);
        }
      } else if (res.message) {
        setError(res.message);
      }
    } catch (error: any) {
      console.error('Failed to add config:', error);
      const errData = error?.response?.data;
      setError(errData?.message || '添加配置失败');
    } finally {
      setSaving(false);
    }
  };

  // 获取可添加的 provider（排除已添加的）
  // 前端限定只展示博查搜索
  const getAvailableProviders = (): ProviderInfo[] => {
    const existingKeys = configs.map(c => c.config_key);
    return providers.filter(p => !existingKeys.includes(p.provider) && p.provider === 'bocha');
  };

  // 获取所有已知的字段（用于没有 provider 匹配时的兜底显示）
  const getAllKnownFields = (configValue: string): string[] => {
    const parsed = parseConfigValue(configValue);
    return Object.keys(parsed);
  };

  return (
    <div className="space-y-6">
      {/* Error message */}
      {error && (
        <motion.div
          initial={{ opacity: 0, y: -8 }}
          animate={{ opacity: 1, y: 0 }}
          className="p-4 rounded-xl bg-error/5 border border-error/20 flex items-center gap-3"
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

      {/* Group Selector - 始终显示全部 4 个 group */}
      <div className="flex gap-2 flex-wrap">
        {allGroups.map((group) => {
          const status = getGroupStatus(group);
          const count = status?.count ?? 0;
          const enabledCount = status?.enabled_count ?? 0;
          return (
            <button
              key={group}
              onClick={() => {
                setSelectedGroup(group);
                setShowAddForm(false);
                setEditingId(null);
              }}
              className={cn(
                'px-4 py-2 rounded-lg text-sm font-medium transition-all cursor-pointer',
                selectedGroup === group
                  ? 'bg-accent text-white'
                  : 'bg-bg-card border border-border-light text-text-primary hover:border-accent/40'
              )}
            >
              {groupLabels[group] || group}
              {count > 0 && (
                <span className="ml-2 text-xs opacity-70">
                  ({enabledCount}/{count})
                </span>
              )}
            </button>
          );
        })}
      </div>

      {/* 配置数量限制提示 */}
      {configs.length > 0 && (
        <div className="mb-4 p-3 rounded-xl bg-bg-tertiary">
          <p className="text-xs text-text-muted">
            💡 每种服务类型只能配置一个。如需更换，请先删除当前配置。
          </p>
        </div>
      )}

      {/* Config List */}
      {loading ? (
        <div className="text-center py-8 text-text-muted">加载中...</div>
      ) : configs.length === 0 ? (
        <div className="text-center py-8 text-text-muted">
          暂无{groupLabels[selectedGroup]}配置，点击下方按钮添加
        </div>
      ) : (
        <div className="space-y-3">
          {configs.map((config) => {
            const providerInfo = getProviderInfo(config.config_key);
            const fields = getConfigFields(config.config_key);
            const parsedValue = parseConfigValue(config.config_value);
            const isEditing = editingId === config.id;
            // 如果 provider 未注册，使用 config_value 中的 key 作为兜底
            const displayFields = fields.length > 0 ? fields : getAllKnownFields(config.config_value);

            return (
              <motion.div
                key={config.id}
                layout
                className="bg-bg-card rounded-xl border border-border-light p-5"
              >
                <div className="flex items-center justify-between mb-3">
                  <div className="flex items-center gap-3">
                    <div className={cn(
                      'w-2.5 h-2.5 rounded-full',
                      config.enabled ? 'bg-success animate-pulse' : 'bg-text-muted'
                    )} />
                    <h3 className="text-sm font-semibold text-text-primary">
                      {providerInfo?.display_name || config.description || config.config_key}
                    </h3>
                    <Badge variant={config.enabled ? 'success' : 'default'}>
                      {config.enabled ? '已启用' : '已禁用'}
                    </Badge>
                  </div>
                  <div className="flex items-center gap-2">
                    <button
                      onClick={() => isEditing ? setEditingId(null) : handleStartEdit(config)}
                      className="p-1.5 rounded-lg text-text-muted hover:text-accent hover:bg-accent/5 transition-colors cursor-pointer"
                    >
                      {isEditing ? <X size={14} /> : <Edit2 size={14} />}
                    </button>
                    <button
                      onClick={() => handleToggleConfig(config)}
                      className="cursor-pointer"
                    >
                      {config.enabled ? (
                        <ToggleRight size={24} className="text-success" />
                      ) : (
                        <ToggleLeft size={24} className="text-text-muted" />
                      )}
                    </button>
                    <button
                      onClick={() => handleDeleteConfig(config)}
                      className="p-1.5 rounded-lg text-text-muted hover:text-error hover:bg-error/5 transition-colors cursor-pointer"
                    >
                      <Trash2 size={14} />
                    </button>
                  </div>
                </div>

                {isEditing ? (
                  /* Edit Form - 结构化表单 */
                  <div className="space-y-3">
                    {fields.length > 0 ? (
                      fields.map(field => {
                        const required = providerInfo?.required_keys?.includes(field) ?? false;
                        const label = getFieldLabel(config.config_key, field);
                        const isPassword = field.includes('key') || field.includes('secret');

                        return (
                          <Input
                            key={field}
                            label={required ? `${label} *` : `${label} (可选)`}
                            type={isPassword ? 'password' : 'text'}
                            value={editFormData[field] || ''}
                            onChange={(e) => setEditFormData({ ...editFormData, [field]: e.target.value })}
                          />
                        );
                      })
                    ) : (
                      // provider 未注册时，显示原始 JSON 编辑
                      <div>
                        <label className="block text-sm font-medium text-text-primary mb-1.5">
                          配置值 (JSON)
                        </label>
                        <textarea
                          className="w-full h-24 px-3 py-2 rounded-lg bg-bg-tertiary border border-border-light text-sm font-[family-name:var(--font-mono)] focus:outline-none focus:border-accent resize-none"
                          value={JSON.stringify(editFormData, null, 2)}
                          onChange={(e) => {
                            try {
                              setEditFormData(JSON.parse(e.target.value));
                            } catch {
                              // 允许编辑无效 JSON
                            }
                          }}
                        />
                      </div>
                    )}
                    <div className="flex justify-end gap-2">
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => {
                          setEditingId(null);
                          setEditFormData({});
                          setError(null);
                        }}
                      >
                        <X size={14} /> 取消
                      </Button>
                      <Button
                        size="sm"
                        onClick={() => handleSaveEdit(config)}
                      >
                        <Save size={14} /> 保存
                      </Button>
                    </div>
                  </div>
                ) : (
                  /* Display Mode - 显示配置值 */
                  <div className="grid grid-cols-2 gap-3">
                    {displayFields.map(field => {
                      const value = parsedValue[field];
                      if (!value) return null;
                      const label = providerInfo
                        ? getFieldLabel(config.config_key, field)
                        : field;
                      const isPassword = field.includes('key') || field.includes('secret');

                      return (
                        <div key={field}>
                          <p className="text-xs text-text-muted mb-1">{label}</p>
                          <p className="text-sm text-text-primary font-[family-name:var(--font-mono)]">
                            {isPassword ? '••••••••' : value}
                          </p>
                        </div>
                      );
                    })}
                    {/* 显示描述 */}
                    {config.description && (
                      <div className="col-span-2">
                        <p className="text-xs text-text-muted mb-1">描述</p>
                        <p className="text-sm text-text-primary">{config.description}</p>
                      </div>
                    )}
                  </div>
                )}
              </motion.div>
            );
          })}
        </div>
      )}

      {/* Add Config - 只有当该服务类型没有配置时才显示 */}
      {showAddForm ? (
        <motion.div
          initial={{ opacity: 0, height: 0 }}
          animate={{ opacity: 1, height: 'auto' }}
          className="bg-bg-card rounded-xl border border-accent/30 p-5"
        >
          <h3 className="text-sm font-semibold text-text-primary mb-4">
            新增{groupLabels[selectedGroup]}配置
          </h3>
          <div className="space-y-4">
            <div>
              <label className="block text-sm font-medium text-text-primary mb-1.5">
                选择服务商
              </label>
              <select
                value={newFormData.provider}
                onChange={(e) => {
                  setNewFormData({ ...newFormData, provider: e.target.value, fields: {} });
                }}
                className="w-full h-10 px-3 rounded-lg bg-bg-tertiary border border-border-light text-sm focus:outline-none focus:border-accent"
              >
                <option value="">选择服务商</option>
                {getAvailableProviders().map((p) => (
                  <option key={p.provider} value={p.provider}>{p.display_name}</option>
                ))}
              </select>
            </div>

            {newFormData.provider && (() => {
              const fields = getConfigFields(newFormData.provider);
              return fields.map(field => {
                const providerInfo = getProviderInfo(newFormData.provider);
                const required = providerInfo?.required_keys?.includes(field) ?? false;
                const label = getFieldLabel(newFormData.provider, field);
                const isPassword = field.includes('key') || field.includes('secret');

                return (
                  <Input
                    key={field}
                    label={required ? `${label} *` : `${label} (可选)`}
                    type={isPassword ? 'password' : 'text'}
                    placeholder={`输入${label}`}
                    value={newFormData.fields[field] || ''}
                    onChange={(e) => setNewFormData({
                      ...newFormData,
                      fields: { ...newFormData.fields, [field]: e.target.value }
                    })}
                  />
                );
              });
            })()}
          </div>
          <div className="flex justify-end gap-2 mt-4">
            <Button variant="ghost" size="sm" onClick={() => {
              setShowAddForm(false);
              setNewFormData({ provider: '', fields: {} });
              setError(null);
            }}>
              取消
            </Button>
            <Button size="sm" onClick={handleAddConfig} disabled={saving || !newFormData.provider}>
              {saving ? '添加中...' : <><Check size={14} /> 添加</>}
            </Button>
          </div>
        </motion.div>
      ) : (
        // 只有当该服务类型没有配置时才显示添加按钮
        configs.length === 0 && (
          <button
            onClick={() => setShowAddForm(true)}
            className="w-full flex items-center justify-center gap-2 py-3 rounded-xl border border-dashed border-border-light text-text-muted hover:text-accent hover:border-accent/40 transition-all cursor-pointer"
          >
            <Plus size={16} /> 添加{groupLabels[selectedGroup]}配置
          </button>
        )
      )}
    </div>
  );
}
