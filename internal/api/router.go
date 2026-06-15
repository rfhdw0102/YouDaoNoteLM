package api

import (
	"YoudaoNoteLm/internal/api/v1/admin"
	"YoudaoNoteLm/internal/api/v1/auth"
	"YoudaoNoteLm/internal/api/v1/chat"
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
	"YoudaoNoteLm/internal/service"

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
	importCtrl     *importn.Controller
	adminCtrl      *admin.Controller
	searchCtrl     *search.Controller
	providerCtrl   *providers.Controller
	youdaoCtrl     *youdao.Controller
	userConfigCtrl *userconfig.Controller
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
) *Router {
	return &Router{
		userCtrl:       user.NewController(userService, tokenBlacklist),
		authCtrl:       auth.NewController(authService, userService, captchaSvc),
		notebookCtrl:   notebook.NewController(notebookService),
		sourceCtrl:     source.NewController(sourceService, tokenBlacklist),
		generationCtrl: generation.NewController(generationService),
		chatCtrl:       chat.NewController(chatAgentService, convService),
		tokenBlacklist: tokenBlacklist,
		importCtrl:     importn.NewController(importerService),
		searchCtrl:     search.NewController(searchAgentService, tokenBlacklist),
		adminCtrl:      admin.NewController(adminService),
		providerCtrl:   providers.NewController(configService),
		youdaoCtrl:     youdao.NewController(youdaoService),
		userConfigCtrl: userconfig.NewController(userConfigService, tokenBlacklist),
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
		r.authCtrl.RegisterRoutes(v1)
		r.userCtrl.RegisterRoutes(v1)
		r.notebookCtrl.RegisterRoutes(v1, r.tokenBlacklist)
		r.sourceCtrl.RegisterRoutes(v1)
		r.searchCtrl.RegisterRoutes(v1)
		r.generationCtrl.RegisterRoutes(v1, r.tokenBlacklist)

		// 导入路由（需认证）
		r.importCtrl.RegisterRoutes(v1, r.tokenBlacklist)
		r.chatCtrl.RegisterRoutes(v1, r.tokenBlacklist)

		// 用户配置路由（需认证）
		r.userConfigCtrl.RegisterRoutes(v1)

		// 后台管理路由（需认证）
		r.adminCtrl.RegisterRoutes(v1)

		// 有道云笔记路由（需认证）
		r.youdaoCtrl.RegisterRoutes(v1, r.tokenBlacklist)

		// Provider 发现路由（/active 支持可选认证）
		r.providerCtrl.RegisterRoutes(v1, middleware.OptionalAuth(r.tokenBlacklist))
	}
}
