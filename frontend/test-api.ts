// 测试前端 API 调用
// 使用方法: npx tsx test-api.ts

import * as providersApi from './src/api/providers';
import * as userConfigApi from './src/api/userConfig';

const BASE_URL = 'http://localhost:8080/api/v1';

async function testProviders() {
  console.log('=== 测试 Provider Registry API ===\n');

  try {
    // 测试获取所有 providers
    console.log('1. 获取所有 providers:');
    const allProviders = await providersApi.listProviders();
    console.log(JSON.stringify(allProviders, null, 2));

    // 测试按类型过滤
    console.log('\n2. 获取 search 类型的 providers:');
    const searchProviders = await providersApi.getProvidersByType('search');
    console.log(JSON.stringify(searchProviders, null, 2));

    console.log('\n3. 获取 llm 类型的 providers:');
    const llmProviders = await providersApi.getProvidersByType('llm');
    console.log(JSON.stringify(llmProviders, null, 2));

    console.log('\n4. 获取 embedding 类型的 providers:');
    const embeddingProviders = await providersApi.getProvidersByType('embedding');
    console.log(JSON.stringify(embeddingProviders, null, 2));

    console.log('\n5. 获取 asr 类型的 providers:');
    const asrProviders = await providersApi.getProvidersByType('asr');
    console.log(JSON.stringify(asrProviders, null, 2));
  } catch (error) {
    console.error('Provider API 测试失败:', error);
  }
}

async function testUserConfig() {
  console.log('\n=== 测试用户配置 API ===\n');
  console.log('注意：用户配置 API 需要认证 token，这里仅展示 API 结构\n');

  // 展示 API 函数签名
  console.log('可用的用户配置 API:');
  console.log('- listLLMConfigs()');
  console.log('- createLLMConfig(data: UserConfigRequest)');
  console.log('- updateLLMConfig(id: number, data: UserConfigRequest)');
  console.log('- deleteLLMConfig(id: number)');
  console.log('- listSearchConfigs()');
  console.log('- createSearchConfig(data: UserConfigRequest)');
  console.log('- updateSearchConfig(id: number, data: UserConfigRequest)');
  console.log('- deleteSearchConfig(id: number)');
  console.log('- listASRConfigs()');
  console.log('- createASRConfig(data: UserConfigRequest)');
  console.log('- updateASRConfig(id: number, data: UserConfigRequest)');
  console.log('- deleteASRConfig(id: number)');
  console.log('- listEmbeddingConfigs()');
  console.log('- createEmbeddingConfig(data: UserConfigRequest)');
  console.log('- updateEmbeddingConfig(id: number, data: UserConfigRequest)');
  console.log('- deleteEmbeddingConfig(id: number)');
}

async function main() {
  console.log('YoudaoNoteLM 前端 API 测试\n');
  console.log(`Base URL: ${BASE_URL}\n`);

  await testProviders();
  await testUserConfig();

  console.log('\n=== 测试完成 ===');
}

main().catch(console.error);
