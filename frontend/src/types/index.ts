// ============ Source Types ============
export type SourceType = 'file' | 'url' | 'audio' | 'youdao' | 'search';
export type FileType = 'pdf' | 'docx' | 'txt' | 'md' | 'pptx' | 'mp3' | 'wav';

export interface Source {
  id: string;
  name: string;
  type: SourceType;
  fileType?: FileType;
  content?: string;       // parsed markdown content
  originalContent?: string;
  size?: number;
  url?: string;
  selected: boolean;
  status?: 'loading' | 'ready' | 'error';  // loading = importing in progress
  errorMessage?: string;
  previewId?: string;     // for audio: pending preview ID (not yet confirmed)
  taskId?: string;        // 后端导入任务 ID（loading 状态下用于取消）
  createdAt: string;
  updatedAt: string;
}

// ============ Chat Types ============
export type MessageRole = 'user' | 'assistant' | 'system';

export interface Reference {
  sourceId: string;
  sourceName: string;
  parentBlockId: number;
  chunkContent: string;
  score: number;
}

export interface ChatMessage {
  id: string;
  role: MessageRole;
  content: string;
  timestamp: string;
  isStreaming?: boolean;
  references?: Reference[];
  citations?: string[];   // source IDs referenced (deprecated, use references)
}

export interface Conversation {
  id: string;
  title: string;
  notebookId: string;
  messages: ChatMessage[];
  createdAt: string;
  updatedAt: string;
}

// ============ Note Types ============
export type NoteType = 'note' | 'mindmap' | 'quiz' | 'ppt';

export interface Note {
  id: string;
  title: string;
  type: NoteType;
  content: string;        // markdown content
  isSource: boolean;       // whether added as source
  notebookId: string;
  createdAt: string;
  updatedAt: string;
}

// ============ Quiz Types ============
export interface QuizQuestion {
  id: string;
  question: string;
  options: string[];
  correctIndex: number;
  explanation: string;
}

export interface Quiz {
  questions: QuizQuestion[];
  count: 5 | 10 | 15 | 20;
}

// ============ Notebook Types ============
export interface Notebook {
  id: string;
  name: string;
  description?: string;
  sources: Source[];
  conversations: Conversation[];
  notes: Note[];
  createdAt: string;
  updatedAt: string;
}

// ============ User Types ============
export interface User {
  id: string;
  email: string;
  nickname: string;
  avatar?: string;
  role?: 'user' | 'admin';
}

// ============ Settings Types ============
export interface AIServiceConfig {
  id: string;
  name: string;
  apiKey: string;
  model: string;
  url: string;
  enabled: boolean;
  contextWindow?: number;
}

export interface SearchAPIConfig {
  id: string;
  name: string;
  apiKey: string;
  dailyQuota: number;
  usedQuota: number;
  enabled: boolean;
}

// ============ UI Types ============
export type PanelTab = 'sources' | 'chat' | 'notes';
export type NoteViewMode = 'list' | 'viewer' | 'editor';

// ============ Admin Types ============
export interface AdminUser {
  id: string;
  username: string;
  email: string;
  nickname: string;
  avatar?: string;
  role: 'user' | 'admin';
  enabled: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface SysConfig {
  id: string;
  group: string;
  configKey: string;
  configValue: string;
  description?: string;
  enabled: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface ConfigGroupStatus {
  group: string;
  count: number;
  enabledCount: number;
}

// ============ User Config Types ============
export type ConfigType = 'search' | 'asr' | 'llm' | 'embedding';

export interface UserConfig {
  id: string;
  userId: string;
  configType: ConfigType;
  name: string;
  provider: string;
  apiKey: string;
  apiUrl: string;
  model?: string;
  dailyQuota?: number;
  dimensions?: number;
  extraConfig?: Record<string, any>;
  enabled: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface UserLLMConfig {
  id: string;
  userId: string;
  name: string;
  provider: string;
  apiKey: string;
  apiUrl: string;
  model: string;
  enabled: boolean;
  createdAt: string;
  updatedAt: string;
}
