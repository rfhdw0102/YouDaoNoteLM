import { create } from 'zustand';
import type { AIServiceConfig, SearchAPIConfig } from '../types';

interface SettingsState {
  aiConfigs: AIServiceConfig[];
  searchConfigs: SearchAPIConfig[];
  addAIConfig: (config: AIServiceConfig) => void;
  updateAIConfig: (id: string, data: Partial<AIServiceConfig>) => void;
  removeAIConfig: (id: string) => void;
  addSearchConfig: (config: SearchAPIConfig) => void;
  updateSearchConfig: (id: string, data: Partial<SearchAPIConfig>) => void;
  removeSearchConfig: (id: string) => void;
}

export const useSettingsStore = create<SettingsState>((set) => ({
  aiConfigs: [
    {
      id: 'ai-1',
      name: '默认配置',
      apiKey: 'sk-****************************abcd',
      model: 'gpt-4o',
      url: 'https://api.openai.com/v1',
      enabled: true,
      contextWindow: 128000,
    },
  ],
  searchConfigs: [
    {
      id: 'search-1',
      name: 'SerpAPI',
      apiKey: '****************************',
      dailyQuota: 100,
      usedQuota: 23,
      enabled: true,
    },
  ],

  addAIConfig: (config) =>
    set((state) => ({ aiConfigs: [...state.aiConfigs, config] })),

  updateAIConfig: (id, data) =>
    set((state) => ({
      aiConfigs: state.aiConfigs.map((c) => (c.id === id ? { ...c, ...data } : c)),
    })),

  removeAIConfig: (id) =>
    set((state) => ({ aiConfigs: state.aiConfigs.filter((c) => c.id !== id) })),

  addSearchConfig: (config) =>
    set((state) => ({ searchConfigs: [...state.searchConfigs, config] })),

  updateSearchConfig: (id, data) =>
    set((state) => ({
      searchConfigs: state.searchConfigs.map((c) => (c.id === id ? { ...c, ...data } : c)),
    })),

  removeSearchConfig: (id) =>
    set((state) => ({ searchConfigs: state.searchConfigs.filter((c) => c.id !== id) })),
}));
