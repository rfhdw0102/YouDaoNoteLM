package service

import (
	"context"

	"YoudaoNoteLm/internal/llm"
	"YoudaoNoteLm/internal/model/entity"
	"YoudaoNoteLm/internal/rag"

	"github.com/cloudwego/eino/components/model"
)

type userLLMConfigReader interface {
	FindDefaultByUserID(userID uint) (*entity.UserLLMConfig, error)
}

type chatModelFactory func(ctx context.Context, cfg *entity.UserLLMConfig) (model.BaseChatModel, error)

type userLLMConfigGenerationService struct {
	repo    userLLMConfigReader
	factory chatModelFactory
	base    func(model GenerationModel) GenerationService
}

func NewGenerationServiceWithUserLLMConfig(retriever rag.RAGRetriever, search SearchService, repo userLLMConfigReader) GenerationService {
	return newGenerationServiceWithUserLLMChatModelFactory(retriever, search, repo, func(ctx context.Context, cfg *entity.UserLLMConfig) (model.BaseChatModel, error) {
		return llm.NewChatModel(ctx, cfg)
	})
}

func newGenerationServiceWithUserLLMChatModelFactory(retriever rag.RAGRetriever, search SearchService, repo userLLMConfigReader, factory chatModelFactory) GenerationService {
	return &userLLMConfigGenerationService{
		repo:    repo,
		factory: factory,
		base: func(model GenerationModel) GenerationService {
			return NewGenerationService(retriever, search, model)
		},
	}
}

func (s *userLLMConfigGenerationService) Generate(ctx context.Context, req *GenerationRequest) (*GenerationResponse, error) {
	model, err := s.resolveModel(ctx, req)
	if err != nil {
		return nil, err
	}
	return s.base(model).Generate(ctx, req)
}

func (s *userLLMConfigGenerationService) resolveModel(ctx context.Context, req *GenerationRequest) (GenerationModel, error) {
	if s.repo == nil || req == nil || req.UserID == 0 {
		return nil, nil
	}
	cfg, err := s.repo.FindDefaultByUserID(req.UserID)
	if err != nil || cfg == nil {
		return nil, err
	}
	chatModel, err := s.factory(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return NewEinoGenerationModel(chatModel), nil
}
