package app

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"YoudaoNoteLm/internal/api"
	"YoudaoNoteLm/internal/model/entity"
	"YoudaoNoteLm/internal/rag"
	"YoudaoNoteLm/internal/repository"
	"YoudaoNoteLm/internal/service"
	"YoudaoNoteLm/internal/service/external"
	"YoudaoNoteLm/pkg/cache"
	"YoudaoNoteLm/pkg/config"
	"YoudaoNoteLm/pkg/database"
	"YoudaoNoteLm/pkg/logger"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// App 应用入口。
type App struct {
	cfg          *config.Config
	mysqlDB      *gorm.DB
	redis        *redis.Client
	router       *api.Router
	server       *http.Server
	ragRetriever rag.RAGRetriever
}

func milvusInitContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 10*time.Second)
}

// NewApp 创建应用。
func NewApp() *App {
	return &App{}
}

// Initialize 初始化应用依赖。
func (a *App) Initialize() error {
	if err := a.initConfig(); err != nil {
		return err
	}
	if err := a.initLogger(); err != nil {
		return err
	}
	if err := a.initDatabase(); err != nil {
		return err
	}

	a.initDependencies()
	a.initRouter()
	a.initServer()
	return nil
}

// initConfig 加载配置
func (a *App) initConfig() error {
	cfg, err := config.Load("")
	if err != nil {
		return fmt.Errorf("load config failed: %w", err)
	}
	a.cfg = cfg
	return nil
}

// initLogger 初始化日志
func (a *App) initLogger() error {
	if err := logger.Init(&a.cfg.Log); err != nil {
		return fmt.Errorf("init logger failed: %w", err)
	}

	// 打印启动横幅
	logger.Info("=========================================")
	logger.Info(fmt.Sprintf("starting %s", a.cfg.App.Name))
	logger.Info(fmt.Sprintf("version: %s", a.cfg.App.Version))
	logger.Info(fmt.Sprintf("mode: %s", a.cfg.App.Mode))
	logger.Info("config loaded")
	logger.Info("=========================================")

	return nil
}

// initDatabase 初始化数据库
func (a *App) initDatabase() error {
	// 初始化 MySQL
	mysqlDB, err := database.InitMySQL(&a.cfg.Database.MySQL)
	if err != nil {
		return fmt.Errorf("init mysql failed: %w", err)
	}
	a.mysqlDB = mysqlDB

	// 自动迁移数据库表
	logger.Info("开始数据库迁移...")
	if err := a.mysqlDB.AutoMigrate(
		&entity.User{},
		&entity.Notebook{},
		&entity.Conversation{},
		&entity.Message{},
		&entity.Source{},
		&entity.ParentBlock{},
		&entity.UserConfig{},
		&entity.UserLLMConfig{},
		&entity.YoudaoBinding{},
		&entity.SysConfig{},
	); err != nil {
		logger.Warn("database migration failed", zap.Error(err))
	}

	// 初始化 Redis（可选）
	rs, err := database.InitRedis(&a.cfg.Database.Redis)
	if err != nil {
		logger.Warn("init redis failed, continue without redis", zap.Error(err))
	}
	a.redis = rs

	return nil
}

// initDependencies 初始化依赖注入
func (a *App) initDependencies() {
	// 创建 Repository
	userRepo := repository.NewUserRepository(a.mysqlDB)
	notebookRepo := repository.NewNotebookRepository(a.mysqlDB)
	sourceRepo := repository.NewSourceRepository(a.mysqlDB)
	userConfigRepo := repository.NewUserConfigRepository(a.mysqlDB)
	llmConfigRepo := repository.NewUserLLMConfigRepository(a.mysqlDB)
	conversationRepo := repository.NewConversationRepository(a.mysqlDB)
	messageRepo := repository.NewMessageRepository(a.mysqlDB)

	// 创建 Service
	emailSvc := service.NewEmailService()
	verifyCodeSvc := service.NewVerifyCodeService(a.redis, emailSvc)
	captchaSvc := service.NewCaptchaService(a.redis)
	tokenBlacklistSvc := service.NewTokenBlacklistService(a.redis)

	// 创建外部服务客户端
	markitdownClient := external.NewMarkitdownClient(a.cfg.External.MarkItDown.URL)
	minioStorage := external.NewMinIOStorage(
		a.cfg.External.MinIO.Endpoint,
		a.cfg.External.MinIO.AccessKey,
		a.cfg.External.MinIO.SecretKey,
		a.cfg.External.MinIO.Bucket,
	)
	bochaClient := external.NewBochaSearchClient(&http.Client{}, a.cfg.External.Bocha.Endpoint)

	userSvc := service.NewUserService(userRepo, verifyCodeSvc, minioStorage)
	authSvc := service.NewAuthService(userRepo, userSvc, verifyCodeSvc, captchaSvc, tokenBlacklistSvc)
	notebookSvc := service.NewNotebookService(notebookRepo)

	// ASR 服务（根据 provider 配置自动选择实现）
	asrSvc := external.NewASRService(a.cfg.External.ASR)
	// 注入 MinIO 存储，ASR 需要生成预签名 URL
	if setter, ok := asrSvc.(interface{ SetStorage(external.FileStorage) }); ok {
		setter.SetStorage(minioStorage)
	}

	var redisCache *cache.Cache
	if a.redis != nil {
		redisCache = cache.New(a.redis)
	}
	importTaskCache := cache.NewImportTaskCache(redisCache)
	audioPreviewCache := cache.NewAudioPreviewCache(redisCache)
	searchSvc := service.NewSearchService(bochaClient, userConfigRepo, redisCache, a.cfg.External.Bocha)

	// 创建 IngestionService
	ingestionSvc := a.initIngestionService(sourceRepo)
	if ingestionSvc == nil {
		logger.Warn("ingestion service unavailable, vector ingestion disabled")
	}

	// 创建 RAGRetriever
	var ragRetriever rag.RAGRetriever
	if ingestionSvc != nil {
		userConfigRepo := repository.NewUserConfigRepository(a.mysqlDB)
		parentBlockRepo := repository.NewParentBlockRepository(a.mysqlDB)
		embedderProvider := func(ctx context.Context, userID uint) (embedding.Embedder, error) {
			cfg, err := userConfigRepo.FindByUserAndType(userID, "embedding")
			if err != nil {
				return nil, err
			}
			if cfg == nil {
				return nil, fmt.Errorf("用户 %d 未配置 Embedding", userID)
			}
			return rag.NewEmbedder(ctx, cfg)
		}

		// 创建独立的 MilvusWriter 用于检索（Milvus 客户端轻量）
		milvusCtx, milvusCancel := milvusInitContext()
		milvusWriter, err := rag.NewMilvusWriter(milvusCtx, rag.MilvusIndexerConfig{
			Address: a.cfg.External.Milvus.Address,
		})
		milvusCancel()
		if err != nil {
			logger.Warn("Milvus Writer 初始化失败，RAGRetriever 不可用", zap.Error(err))
		} else {
			ragRetriever = rag.NewRAGRetriever(
				milvusWriter,
				parentBlockRepo,
				sourceRepo,
				embedderProvider,
				5, // defaultTopK
			)
			a.ragRetriever = ragRetriever
			logger.Info("RAGRetriever 初始化成功")
		}
	}

	// 创建 SourceService（需要 IngestionService 来删除向量数据）
	sourceSvc := service.NewSourceService(sourceRepo, ingestionSvc)

	// 创建导入服务
	importerSvc := service.NewImporterService(
		markitdownClient,
		asrSvc,
		minioStorage,
		sourceRepo,
		importTaskCache,
		audioPreviewCache,
		ingestionSvc,
	)
	generationSvc := service.NewGenerationServiceWithUserLLMConfig(a.ragRetriever, searchSvc, llmConfigRepo)

	// 创建 ChatAgentService
	var chatAgentSvc service.ChatAgentService
	if a.redis != nil && ragRetriever != nil {
		chatCache := cache.NewChatCache(a.redis)
		chatAgentSvc = service.NewChatAgentService(llmConfigRepo, ragRetriever, conversationRepo, messageRepo, chatCache)
		logger.Info("ChatAgentService 初始化成功")
	}

	a.router = api.NewRouter(
		userSvc,
		authSvc,
		notebookSvc,
		sourceSvc,
		searchSvc,
		generationSvc,
		importerSvc,
		captchaSvc,
		tokenBlacklistSvc,
		chatAgentSvc,
	)
}

// initIngestionService 初始化入库服务
// 从数据库读取用户的 Embedding 配置，创建 EmbedderProvider 和 MilvusWriter
func (a *App) initIngestionService(sourceRepo repository.SourceRepository) rag.IngestionService {
	ctx, cancel := milvusInitContext()
	defer cancel()

	// 创建 Milvus Writer
	milvusWriter, err := rag.NewMilvusWriter(ctx, rag.MilvusIndexerConfig{
		Address: a.cfg.External.Milvus.Address,
	})
	if err != nil {
		logger.Warn("init milvus writer failed", zap.Error(err))
		return nil
	}

	// 创建 EmbedderProvider：根据 userID 从数据库读取配置
	userConfigRepo := repository.NewUserConfigRepository(a.mysqlDB)
	embedderProvider := func(ctx context.Context, userID uint) (embedding.Embedder, error) {
		cfg, err := userConfigRepo.FindByUserAndType(userID, "embedding")
		if err != nil {
			return nil, err
		}
		if cfg == nil {
			return nil, fmt.Errorf("user %d missing embedding config", userID)
		}
		return rag.NewEmbedder(ctx, cfg)
	}

	parentRepo := repository.NewParentBlockRepository(a.mysqlDB)
	ingestionSvc := rag.NewIngestionService(sourceRepo, parentRepo, embedderProvider, milvusWriter)
	logger.Info("IngestionService 初始化成功")
	return ingestionSvc
}

// initRouter 初始化路由
func (a *App) initRouter() {
	// 设置 Gin 模式
	gin.SetMode(a.cfg.App.Mode)
}

// initServer 初始化 HTTP 服务器
func (a *App) initServer() {
	engine := gin.New()

	// 注册路由
	a.router.Setup(engine)

	// 创建 HTTP 服务器
	a.server = &http.Server{
		Addr:           fmt.Sprintf(":%d", a.cfg.App.Port),
		Handler:        engine,
		ReadTimeout:    60 * time.Second,
		WriteTimeout:   60 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
}

// Run 启动应用。
func (a *App) Run() {
	// 启动 HTTP 服务器
	go func() {
		logger.Info("http server started",
			zap.String("addr", a.server.Addr),
			zap.String("mode", a.cfg.App.Mode),
		)
		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("http server failed", zap.Error(err))
		}
	}()

	// 优雅关闭
	a.gracefulShutdown()
}

// gracefulShutdown 优雅关闭
func (a *App) gracefulShutdown() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 关闭 HTTP 服务器
	if err := a.server.Shutdown(ctx); err != nil {
		logger.Error("shutdown server failed", zap.Error(err))
	}

	// 关闭数据库连接
	if err := database.CloseMySQL(); err != nil {
		logger.Error("close mysql failed", zap.Error(err))
	}
	if err := database.CloseRedis(); err != nil {
		logger.Error("close redis failed", zap.Error(err))
	}

	// 同步日志
	_ = logger.Sync() // 日志同步失败无需处理，即将退出

	logger.Info("服务器已关闭")
	logger.Info("=========================================")
}
