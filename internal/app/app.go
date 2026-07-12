package app

import (
	searchAgent "YoudaoNoteLm/internal/agent/search"
	"YoudaoNoteLm/internal/api"
	"YoudaoNoteLm/internal/model/entity"
	"YoudaoNoteLm/internal/rag"
	"YoudaoNoteLm/internal/repository"
	"YoudaoNoteLm/internal/service"
	"YoudaoNoteLm/internal/service/external"
	externalMarkitdown "YoudaoNoteLm/internal/service/external/markitdown"
	externalStorage "YoudaoNoteLm/internal/service/external/storage"
	externalYoudao "YoudaoNoteLm/internal/service/external/youdao"
	"YoudaoNoteLm/pkg/cache"
	"YoudaoNoteLm/pkg/config"
	"YoudaoNoteLm/pkg/database"
	"YoudaoNoteLm/pkg/logger"
	"YoudaoNoteLm/pkg/utils"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	// 触发 provider 注册（各子包 init() 会自动注册到全局 Registry）
	_ "YoudaoNoteLm/internal/service/external/asr"
	_ "YoudaoNoteLm/internal/service/external/embedding"
	_ "YoudaoNoteLm/internal/service/external/llm"
	_ "YoudaoNoteLm/internal/service/external/search"

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
	if err := a.verifyEncryptionKey(); err != nil {
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

// verifyEncryptionKey 校验 ENCRYPTION_KEY 能否解密数据库中已加密的 API Key
// 启动失败比静默降级更好——立即暴露密钥不匹配问题，避免运行时把密文当明文发给外部 API 导致 401
func (a *App) verifyEncryptionKey() error {
	key := a.cfg.Security.EncryptionKey
	if key == "" {
		return nil
	}

	// 优先抽样 user_llm_config 表
	var llmCfg entity.UserLLMConfig
	if err := a.mysqlDB.First(&llmCfg).Error; err == nil {
		if llmCfg.APIKey != "" {
			if _, err := utils.Decrypt(llmCfg.APIKey, []byte(key)); err != nil {
				return fmt.Errorf("ENCRYPTION_KEY 与数据库中已加密的 API Key 不匹配（user_llm_config 表），"+
					"请确保使用与加密时相同的 32 字节密钥，或在 Web UI 重新输入 API Key: %w", err)
			}
		}
		return nil
	}

	// 再抽样 user_config 表
	var userCfg entity.UserConfig
	if err := a.mysqlDB.First(&userCfg).Error; err == nil {
		if userCfg.APIKey != "" {
			if _, err := utils.Decrypt(userCfg.APIKey, []byte(key)); err != nil {
				return fmt.Errorf("ENCRYPTION_KEY 与数据库中已加密的 API Key 不匹配（user_config 表），"+
					"请确保使用与加密时相同的 32 字节密钥，或在 Web UI 重新输入 API Key: %w", err)
			}
		}
		return nil
	}

	// 无数据，跳过
	return nil
}

// initDependencies 初始化依赖注入
func (a *App) initDependencies() {
	// 创建 Repository
	userRepo := repository.NewUserRepository(a.mysqlDB)
	notebookRepo := repository.NewNotebookRepository(a.mysqlDB)
	sourceRepo := repository.NewSourceRepository(a.mysqlDB)
	sysConfigRepo := repository.NewSysConfigRepository(a.mysqlDB)
	userConfigRepo := repository.NewUserConfigRepository(a.mysqlDB)
	llmConfigRepo := repository.NewUserLLMConfigRepository(a.mysqlDB)
	conversationRepo := repository.NewConversationRepository(a.mysqlDB)
	messageRepo := repository.NewMessageRepository(a.mysqlDB)
	chatCache := cache.NewChatCache(a.redis)

	// 创建外部服务客户端
	markitdownClient := externalMarkitdown.NewClient(a.cfg.External.MarkItDown.URL)
	minioStorage, err := externalStorage.NewMinIOStorage(
		a.cfg.External.MinIO.Endpoint,
		a.cfg.External.MinIO.AccessKey,
		a.cfg.External.MinIO.SecretKey,
		a.cfg.External.MinIO.Bucket,
		a.cfg.External.MinIO.PublicEndpoint,
	)
	if err != nil {
		logger.Fatal("MinIO 初始化失败，文件上传功能将不可用", zap.Error(err))
	}
	bochaClient := external.NewBochaSearchClient(&http.Client{}, a.cfg.External.Bocha.Endpoint)

	// 创建 Service
	emailSvc := service.NewEmailService()
	verifyCodeSvc := service.NewVerifyCodeService(a.redis, emailSvc)
	captchaSvc := service.NewCaptchaService(a.redis)
	tokenBlacklistSvc := service.NewTokenBlacklistService(a.redis)
	userSvc := service.NewUserService(userRepo, verifyCodeSvc, minioStorage)
	authSvc := service.NewAuthService(userRepo, userSvc, verifyCodeSvc, captchaSvc, tokenBlacklistSvc)
	// 创建缓存
	redisCache := cache.New(a.redis)
	importTaskCache := cache.NewImportTaskCache(redisCache)
	audioPreviewCache := cache.NewAudioPreviewCache(redisCache)
	sourceSummaryCache := cache.NewSourceSummaryCache(a.redis)

	// 创建 ConfigService（配置路由降级，管理 ASR/Search/LLM/Embedding 等动态服务）
	configSvc := service.NewConfigService(sysConfigRepo, userConfigRepo, llmConfigRepo, redisCache, minioStorage, a.cfg.Security.EncryptionKey)

	// 创建 AdminService（依赖 configSvc 用于清除配置缓存，必须在 configSvc 之后创建）
	adminSvc := service.NewAdminService(userRepo, sysConfigRepo, configSvc)

	// 创建 MarkdownStructurer（LLM 结构化服务）
	structurer := service.NewMarkdownStructurer(configSvc)

	searchSvc := service.NewSearchService(bochaClient, userConfigRepo, redisCache, a.cfg.External.Bocha)
	// 创建 IngestionService
	ingestionSvc := a.initIngestionService(sourceRepo, configSvc)
	if ingestionSvc == nil {
		logger.Warn("ingestion service unavailable, vector ingestion disabled")
	}

	// 创建笔记本和资料来源服务（需要依赖 IngestionService 来删除向量数据）
	notebookSvc := service.NewNotebookService(notebookRepo, sourceRepo, conversationRepo, messageRepo, ingestionSvc, chatCache, sourceSummaryCache)
	sourceSvc := service.NewSourceService(sourceRepo, minioStorage, ingestionSvc, sourceSummaryCache)

	// 创建导入服务（ASR 通过 ConfigService 动态获取，RAG 通过 IngestionService）
	importerSvc := service.NewImporterService(
		configSvc, markitdownClient, minioStorage,
		sourceRepo, importTaskCache, audioPreviewCache,
		ingestionSvc, structurer, sourceSummaryCache,
	)

	// 创建 RAGRetriever
	parentBlockRepo := repository.NewParentBlockRepository(a.mysqlDB)
	retrieverEmbedderProvider := func(ctx context.Context, userID uint) (embedding.Embedder, error) {
		cfg, err := configSvc.GetUserConfig(userID, "embedding")
		if err != nil {
			return nil, fmt.Errorf("获取 Embedding 配置失败: %w", err)
		}
		if cfg == nil {
			return nil, fmt.Errorf("请先在设置中配置 Embedding 服务")
		}
		return rag.NewEmbedderFromConfig(ctx, cfg)
	}

	// 创建 EinoRetrieverWrapper 用于检索
	retrieverCtx, retrieverCancel := milvusInitContext()
	defer retrieverCancel()
	ragRetriever, err := rag.NewEinoRetrieverWrapper(
		retrieverCtx,
		a.cfg.Milvus.GetAddress(),
		parentBlockRepo,
		sourceRepo,
		retrieverEmbedderProvider,
		5, // defaultTopK
	)
	if err != nil {
		logger.Fatal("EinoRetrieverWrapper 初始化失败", zap.Error(err))
	}
	a.ragRetriever = ragRetriever
	logger.Info("RAGRetriever 初始化成功")

	// 创建用户配置服务
	userCfgSvc := service.NewUserConfigService(userConfigRepo, llmConfigRepo, configSvc, a.cfg.Security.EncryptionKey)

	// 创建有道云笔记服务（CLI 不可用时仅打 warning，不影响启动）
	youdaoCLI := externalYoudao.NewCLI(a.cfg.External.Youdao.CLIPath, a.cfg.External.Youdao.ConverterScriptPath)
	if err := youdaoCLI.CheckAvailable(); err != nil {
		logger.Warn("youdaonote CLI 不可用，有道云笔记导入功能将无法使用", zap.Error(err))
	}
	youdaoBindingRepo := repository.NewYoudaoBindingRepository(a.mysqlDB)
	youdaoSvc := service.NewYoudaoService(youdaoCLI, youdaoBindingRepo, sourceRepo, ingestionSvc, a.cfg.External.Youdao.CookiesPath, structurer, configSvc, sourceSummaryCache)

	// 创建搜索 Agent（依赖 youdaoSvc、youdaoCLI 和 importerSvc）
	searchAgentInst := searchAgent.NewSearchAgent(configSvc, importerSvc, youdaoSvc, youdaoCLI)
	searchAgentSvc := service.NewSearchAgentService(configSvc, importerSvc, searchAgentInst)

	// 创建生成服务（SearchService 暂为 nil，后续可接入）
	var generationMemory service.GenerationMemoryStore
	if a.redis != nil {
		generationMemory = service.NewGenerationMemoryCacheStore(cache.NewGenerationMemoryCache(a.redis))
	}
	generationSvc := service.NewGenerationServiceWithUserLLMConfigAndMemory(a.ragRetriever, searchSvc, llmConfigRepo, generationMemory, a.cfg.Security.EncryptionKey)

	// 创建 ChatAgentService 和 ConversationService
	chatAgentSvc := service.NewChatAgentService(llmConfigRepo, ragRetriever, conversationRepo, messageRepo, chatCache, sourceRepo, sourceSummaryCache, a.cfg.Security.EncryptionKey)
	convSvc := service.NewConversationService(conversationRepo, messageRepo, chatCache)
	logger.Info("ChatAgentService 初始化成功")
	logger.Info("ConversationService 初始化成功")

	a.router = api.NewRouter(
		userSvc,
		authSvc,
		notebookSvc,
		sourceSvc,
		searchSvc,
		generationSvc,
		importerSvc,
		adminSvc,
		userCfgSvc,
		searchAgentSvc,
		captchaSvc,
		tokenBlacklistSvc,
		chatAgentSvc,
		convSvc,
		configSvc,
		youdaoSvc,
		ingestionSvc,
		minioStorage,
		userRepo,
	)
}

// initIngestionService 初始化入库服务
// 从数据库读取用户的 Embedding 配置，创建 EmbedderProvider 和 EinoIndexerWrapper
func (a *App) initIngestionService(sourceRepo repository.SourceRepository, configSvc service.ConfigService) rag.IngestionService {
	ctx, cancel := milvusInitContext()
	defer cancel()

	// 创建 EinoIndexerWrapper
	einoIndexer, err := rag.NewEinoIndexerWrapper(ctx, a.cfg.Milvus.GetAddress())
	if err != nil {
		logger.Warn("init eino indexer failed", zap.Error(err))
		return nil
	}

	// 创建 EmbedderProvider：通过 ConfigService 创建
	embedderProvider := func(ctx context.Context, userID uint) (embedding.Embedder, int, error) {
		cfg, err := configSvc.GetUserConfig(userID, "embedding")
		if err != nil {
			return nil, 0, fmt.Errorf("获取 Embedding 配置失败: %w", err)
		}
		if cfg == nil {
			return nil, 0, fmt.Errorf("请先在设置中配置 Embedding 服务")
		}
		embedder, err := rag.NewEmbedderFromConfig(ctx, cfg)
		if err != nil {
			return nil, 0, err
		}
		vectorDim := 0
		if cfg.Dimensions != nil {
			vectorDim = *cfg.Dimensions
		}
		return embedder, vectorDim, nil
	}

	parentRepo := repository.NewParentBlockRepository(a.mysqlDB)
	ingestionSvc := rag.NewIngestionService(sourceRepo, parentRepo, embedderProvider, einoIndexer)
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
		WriteTimeout:   600 * time.Second,
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
