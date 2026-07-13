package api

import (
	"YoudaoNoteLm/internal/api/v1/admin"
	"YoudaoNoteLm/internal/api/v1/auth"
	"YoudaoNoteLm/internal/api/v1/chat"
	"YoudaoNoteLm/internal/api/v1/file"
	"YoudaoNoteLm/internal/api/v1/generation"
	"YoudaoNoteLm/internal/api/v1/importn"
	"YoudaoNoteLm/internal/api/v1/notebook"
	"YoudaoNoteLm/internal/api/v1/providers"
	"YoudaoNoteLm/internal/api/v1/search"
	"YoudaoNoteLm/internal/api/v1/source"
	"YoudaoNoteLm/internal/api/v1/user"
	userconfig "YoudaoNoteLm/internal/api/v1/user_config"
	youdao "YoudaoNoteLm/internal/api/v1/youdao"
	"YoudaoNoteLm/internal/middleware"
	"YoudaoNoteLm/internal/rag"
	"YoudaoNoteLm/internal/repository"
	"YoudaoNoteLm/internal/service"
	externalStorage "YoudaoNoteLm/internal/service/external/storage"

	"github.com/gin-gonic/gin"
)

// Router HTTP 路由装配器。
type Router struct {
	userCtrl       *user.Controller
	authCtrl       *auth.Controller
	notebookCtrl   *notebook.Controller
	sourceCtrl     *source.Controller
	generationCtrl *generation.Controller
	chatCtrl       *chat.Controller
	tokenBlacklist service.TokenBlacklistService
	userRepo       repository.UserRepository
	importCtrl     *importn.Controller
	adminCtrl      *admin.Controller
	searchCtrl     *search.Controller
	providerCtrl   *providers.Controller
	youdaoCtrl     *youdao.Controller
	userConfigCtrl *userconfig.Controller
	fileCtrl       *file.Controller
}

// NewRouter 创建路由。
func NewRouter(
	userService service.UserService,
	authService service.AuthService,
	notebookService service.NotebookService,
	sourceService service.SourceService,
	searchService service.SearchService,
	generationService service.GenerationService,
	importerService service.ImporterService,
	adminService service.AdminService,
	userConfigService service.UserConfigService,
	searchAgentService service.SearchAgentService,
	captchaSvc service.CaptchaService,
	tokenBlacklist service.TokenBlacklistService,
	chatAgentService service.ChatAgentService,
	convService service.ConversationService,
	configService service.ConfigService,
	youdaoService service.YoudaoService,
	ingestionService rag.IngestionService,
	storage externalStorage.FileStorage,
	userRepo repository.UserRepository,
) *Router {
	return &Router{
		userCtrl:       user.NewController(userService, tokenBlacklist),
		authCtrl:       auth.NewController(authService, userService, captchaSvc),
		notebookCtrl:   notebook.NewController(notebookService),
		sourceCtrl:     source.NewController(sourceService, tokenBlacklist),
		generationCtrl: generation.NewController(generationService),
		chatCtrl:       chat.NewController(chatAgentService, convService),
		tokenBlacklist: tokenBlacklist,
		userRepo:       userRepo,
		importCtrl:     importn.NewController(importerService),
		searchCtrl:     search.NewController(searchAgentService, tokenBlacklist),
		adminCtrl:      admin.NewController(adminService),
		providerCtrl:   providers.NewController(configService),
		youdaoCtrl:     youdao.NewController(youdaoService),
		userConfigCtrl: userconfig.NewController(userConfigService, tokenBlacklist, ingestionService),
		fileCtrl:       file.NewController(storage),
	}
}

// Setup 注册所有路由。
func (r *Router) Setup(engine *gin.Engine) {
	engine.Use(middleware.Recovery())
	engine.Use(middleware.Logger())
	engine.Use(middleware.CORS())

	engine.Static("/uploads", "./uploads")

	engine.GET("/api/v1/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ok",
			"message": "YouDaoNoteLM API is running",
		})
	})

	v1 := engine.Group("/api/v1")
	{
		// 用户状态检查中间件（被禁用用户立即拦截，返回 1004）
		statusCheck := middleware.StatusCheck(r.userRepo)

		r.authCtrl.RegisterRoutes(v1)
		r.userCtrl.RegisterRoutes(v1, statusCheck)
		r.notebookCtrl.RegisterRoutes(v1, r.tokenBlacklist, statusCheck)
		r.sourceCtrl.RegisterRoutes(v1, statusCheck)
		r.searchCtrl.RegisterRoutes(v1, statusCheck)
		r.generationCtrl.RegisterRoutes(v1, r.tokenBlacklist, statusCheck)

		// 文件代理路由（公开，头像降级访问）
		r.fileCtrl.RegisterRoutes(v1)

		// 导入路由（需认证）
		r.importCtrl.RegisterRoutes(v1, r.tokenBlacklist, statusCheck)
		r.chatCtrl.RegisterRoutes(v1, r.tokenBlacklist, statusCheck)

		// 用户配置路由（需认证）
		r.userConfigCtrl.RegisterRoutes(v1, statusCheck)

		// 后台管理路由（需认证 + 管理员角色）
		r.adminCtrl.RegisterRoutes(v1, r.tokenBlacklist, statusCheck)

		// 有道云笔记路由（需认证）
		r.youdaoCtrl.RegisterRoutes(v1, r.tokenBlacklist, statusCheck)

		// Provider 发现路由（/active 支持可选认证）
		r.providerCtrl.RegisterRoutes(v1, middleware.OptionalAuth(r.tokenBlacklist, r.userRepo))
	}
}
