// generation_user_llm_config.go 提供支持用户自定义 LLM 配置的生成服务。
//
// userLLMConfigGenerationService 包装 GenerationService，在生成时读取用户配置的
// LLM 模型（API Key、Base URL、模型名等），构造对应的 chat model 实例。
// 提供两个构造器：
//   - NewGenerationServiceWithUserLLMConfig：基础版
//   - NewGenerationServiceWithUserLLMConfigAndMemory：带会话记忆版
package generation

import (
	"context"

	"YoudaoNoteLm/internal/llm"
	"YoudaoNoteLm/internal/model/entity"
	"YoudaoNoteLm/internal/rag"
	"YoudaoNoteLm/pkg/logger"
	"YoudaoNoteLm/pkg/utils"

	"github.com/cloudwego/eino/components/model"
	"go.uber.org/zap"
)

type userLLMConfigReader interface {
	FindDefaultByUserID(userID uint) (*entity.UserLLMConfig, error)
}

type chatModelFactory func(ctx context.Context, cfg *entity.UserLLMConfig) (model.BaseChatModel, error)

type userLLMConfigGenerationService struct {
	repo          userLLMConfigReader
	factory       chatModelFactory
	base          func(model GenerationModel) GenerationService
	encryptionKey []byte // 接口密钥加密密钥
}

func NewGenerationServiceWithUserLLMConfig(retriever rag.RAGRetriever, search SearchService, repo userLLMConfigReader, encryptionKey string) GenerationService {
	return NewGenerationServiceWithUserLLMConfigAndMemory(retriever, search, repo, nil, encryptionKey)
}

func NewGenerationServiceWithUserLLMConfigAndMemory(retriever rag.RAGRetriever, search SearchService, repo userLLMConfigReader, memory GenerationMemoryStore, encryptionKey string) GenerationService {
	return newGenerationServiceWithUserLLMChatModelFactory(retriever, search, repo, memory, func(ctx context.Context, cfg *entity.UserLLMConfig) (model.BaseChatModel, error) {
		return llm.NewChatModel(ctx, cfg)
	}, encryptionKey)
}

func newGenerationServiceWithUserLLMChatModelFactory(retriever rag.RAGRetriever, search SearchService, repo userLLMConfigReader, memory GenerationMemoryStore, factory chatModelFactory, encryptionKey string) GenerationService {
	return &userLLMConfigGenerationService{
		repo:    repo,
		factory: factory,
		base: func(model GenerationModel) GenerationService {
			return NewGenerationServiceWithMemory(retriever, search, model, memory)
		},
		encryptionKey: []byte(encryptionKey),
	}
}

func (s *userLLMConfigGenerationService) Generate(ctx context.Context, req *GenerationRequest) (*GenerationResponse, error) {
	model, err := s.resolveModel(ctx, req)
	if err != nil {
		return nil, err
	}
	return s.base(model).Generate(ctx, req)
}

func (s *userLLMConfigGenerationService) Export(ctx context.Context, req *GenerationExportRequest) (*GenerationExportResult, error) {
	return s.base(nil).Export(ctx, req)
}

func (s *userLLMConfigGenerationService) resolveModel(ctx context.Context, req *GenerationRequest) (GenerationModel, error) {
	if s.repo == nil || req == nil || req.UserID == 0 {
		return nil, nil
	}
	cfg, err := s.repo.FindDefaultByUserID(req.UserID)
	if err != nil || cfg == nil {
		return nil, err
	}

	// 解密接口密钥
	if cfg.APIKey != "" && len(s.encryptionKey) > 0 {
		decrypted, decErr := utils.Decrypt(cfg.APIKey, s.encryptionKey)
		if decErr != nil {
			logger.Debug("解密 API Key 失败（可能未加密）", zap.Error(decErr))
		} else {
			cfg.APIKey = decrypted
		}
	}

	chatModel, err := s.factory(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return NewEinoGenerationModel(chatModel), nil
}
