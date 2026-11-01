package internal

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"time"

	"go-admin/internal/apps"
	appcaptcha "go-admin/internal/captcha"
	"go-admin/internal/global"
	"go-admin/internal/handler"
	"go-admin/internal/middleware"
	appoauth "go-admin/internal/oauth"
	adminplugin "go-admin/internal/plugin"
	adminqueue "go-admin/internal/queue"
	"go-admin/internal/service"
	"go-admin/internal/utils"
	"go-admin/internal/ws"
	"go-admin/pluginapi"

	"github.com/gofiber/fiber/v3"
	"github.com/wuwuseo/cmf/cache"
	"github.com/wuwuseo/cmf/config"
	"github.com/wuwuseo/cmf/jwt"
	"github.com/wuwuseo/cmf/log"
	cmfplugin "github.com/wuwuseo/cmf/plugin"
	cmfqueue "github.com/wuwuseo/cmf/queue"
	"github.com/wuwuseo/cmf/sse"
	"go.uber.org/zap"
)

var activePluginManager *adminplugin.DefaultManager

func RegisterRoute(app *fiber.App, config *config.Config) {
	// 添加请求日志中间件
	if appInstance != nil && appInstance.Logger != nil {
		app.Use(log.RequestLoggerMiddleware(appInstance.Logger))
	}

	api := app.Group("/api")                         // API 路由组
	v1 := api.Group("/v1", func(c fiber.Ctx) error { // v1 版本路由中间件
		c.Set("Version", "1")
		c.Locals("utils.Response", utils.NewResponse())
		return c.Next()
	})

	admin := v1.Group("/admin")
	public := v1.Group("/public", func(c fiber.Ctx) error {
		c.Locals("service.UserService", service.NewUserService())
		return c.Next()
	})

	captchaRuntime, err := appcaptcha.NewRuntime(
		context.Background(),
		global.ORM,
		config,
		os.Getenv,
	)
	if err != nil {
		panic("initialize captcha runtime: " + err.Error())
	}
	captchaAdapter := appcaptcha.NewFiberAdapter(captchaRuntime.Service)

	// 初始化带缓存的服务
	var permissionService service.PermissionServiceInterface
	departmentService := service.NewDepartmentService()
	var roleService service.RoleServiceInterface
	var systemPermissionService service.SystemPermissionServiceInterface
	if appInstance != nil && appInstance.Cache != nil {
		permissionService = service.NewPermissionService(appInstance.Cache)
		roleService = service.NewRoleServiceWithDepartmentService(appInstance.Cache, departmentService)
	} else {
		permissionService = service.NewPermissionService(nil)
		roleService = service.NewRoleServiceWithDepartmentService(nil, departmentService)
	}
	systemPermissionService = service.NewSystemPermissionService()
	// 数据权限服务注入共享缓存；角色分配/启停、管理员部门变更等权限变更点会同步失效数据范围缓存
	var dataPermRawCache *cache.Cache[[]byte]
	if appInstance != nil {
		dataPermRawCache = appInstance.Cache
	}
	dataPermissionService := service.NewDataPermissionService(departmentService, dataPermRawCache)

	// 初始化操作日志服务与登录日志服务
	opLogService := service.NewOperationLogService()
	loginLogService := service.NewLoginLogService()

	// 初始化用户管理相关服务
	var userListService service.UserListServiceInterface
	var userActivationService service.UserActivationServiceInterface
	var blacklistService service.BlacklistServiceInterface
	var thirdPartyUserService service.ThirdPartyUserServiceInterface
	var roleAssignmentService service.RoleAssignmentServiceInterface
	var userService service.UserServiceInterface

	if appInstance != nil && appInstance.Cache != nil {
		blacklistService = service.NewBlacklistService(appInstance.Cache)
		roleAssignmentService = service.NewRoleAssignmentService(appInstance.Cache, departmentService)
		userService = service.NewUserServiceWithCache(appInstance.Cache)
	} else {
		blacklistService = service.NewBlacklistService(nil)
		roleAssignmentService = service.NewRoleAssignmentService(nil, departmentService)
		userService = service.NewUserService()
	}
	userListService = service.NewUserListService()
	userActivationService = service.NewUserActivationService()
	thirdPartyUserService = service.NewThirdPartyUserService()

	oauthCipher, _ := appoauth.NewSecretCipherFromEnv()
	oauthRegistry := appoauth.NewRegistry(appoauth.NewGitHubProvider())
	oauthConfigService := service.NewOAuthProviderConfigService(global.ORM, oauthCipher, oauthRegistry)
	oauthSessionService := service.NewOAuthSessionService(global.ORM, oauthCipher, time.Now)
	oauthAuthService := service.NewOAuthAuthService(global.ORM, oauthRegistry, oauthConfigService, oauthSessionService, time.Now)

	// 初始化站内消息服务（多渠道：站内信 SSE / 短信 / 邮件 / 极光推送）
	var hub *sse.Hub
	var ticketStore *sse.TicketStore
	var notifierRegistry *service.NotifierRegistry
	if appInstance != nil {
		hub = appInstance.Hub
		ticketStore = appInstance.TicketStore
		notifierRegistry = appInstance.Notifiers
	}
	if notifierRegistry == nil {
		// 兜底：App 未初始化注册表时至少保证站内信渠道可用
		notifierRegistry = service.BuildNotifiers(hub)
	}
	prefSvc := service.NewMessagePreferenceService()
	messageService := service.NewMessageServiceWithNotifiers(notifierRegistry, prefSvc)
	broadcastService := service.NewMessageBroadcastService(messageService)

	// 注册外部渠道异步投递处理器：短信/邮件/极光推送经任务队列 worker 消费。
	// 队列运行时未初始化（Setup 失败或测试环境）时跳过，投递端会自动降级为同步。
	if rt := global.QueueRuntime.Load(); rt != nil {
		rt.Handle(service.TaskNotifyExternal, service.NewExternalDispatchHandler(notifierRegistry))
	}

	// 初始化插件管理服务
	pluginRegistry := cmfplugin.NewMemoryExtensionRegistry()
	requirePluginSignature := config.GetBool("plugin.require_signature")
	if value, found := os.LookupEnv("CMF_PLUGIN_REQUIRE_SIGNATURE"); found {
		requirePluginSignature, err = strconv.ParseBool(value)
		if err != nil {
			panic("initialize plugin signature policy: " + err.Error())
		}
	}
	trustedPluginKeyConfig := config.GetString("plugin.trusted_public_keys")
	if value, found := os.LookupEnv("CMF_PLUGIN_TRUSTED_PUBLIC_KEYS"); found {
		trustedPluginKeyConfig = value
	}
	trustedPluginKeys, err := adminplugin.ParseTrustedPublicKeys(trustedPluginKeyConfig)
	if err != nil {
		panic("initialize plugin trust store: " + err.Error())
	}
	if err := validatePluginTrustPolicy(os.Getenv("CMF_APP_ENV"), requirePluginSignature, len(trustedPluginKeys)); err != nil {
		panic("initialize plugin trust store: " + err.Error())
	}
	pluginManager := adminplugin.NewManager(global.ORM, adminplugin.Options{
		PluginRoot:        "./data/plugins",
		PackageRoot:       "./data/plugin-packages",
		TempRoot:          "./data/plugin-tmp",
		RequireSignature:  requirePluginSignature,
		TrustedPublicKeys: trustedPluginKeys,
		Registry:          pluginRegistry,
	})
	activePluginManager = pluginManager
	if err := pluginManager.RestoreEnabled(context.Background()); err != nil {
		log.Error("恢复插件失败", zap.Error(err))
	}
	pluginService := service.NewPluginService(pluginManager)
	// 登录限流器：每分钟最多 10 次请求，防止暴力破解
	loginLimiter := middleware.NewRateLimiter(middleware.RateLimiterConfig{
		Window:   1 * time.Minute,
		MaxCount: 10,
	})

	// 公开接口
	publicCaptcha := public.Group("/captcha", func(c fiber.Ctx) error {
		c.Locals("captcha.FiberAdapter", captchaAdapter)
		return c.Next()
	})
	publicCaptcha.Post("/challenges", handler.CreateCaptchaChallenge)
	publicCaptcha.Post("/verifications", handler.VerifyCaptchaChallenge)
	public.Post("/login", captchaAdapter.Require("user_login"), loginLimiter.Handler(), handler.Login)
	public.Post("/refresh-token", handler.RefreshToken)
	public.Get("/public-key", handler.GetPublicKey) // 获取RSA公钥用于加密登录
	publicOAuth := public.Group("/oauth", func(c fiber.Ctx) error {
		c.Locals("service.OAuthProviderConfigService", oauthConfigService)
		c.Locals("service.OAuthAuthService", oauthAuthService)
		return c.Next()
	})
	publicOAuth.Get("/providers", handler.ListOAuthProviders)
	publicOAuth.Get("/user/:provider/authorize", func(c fiber.Ctx) error {
		c.Locals("oauth.subject_type", "user")
		return handler.BeginOAuthLogin(c)
	})
	publicOAuth.Get("/admin/:provider/authorize", func(c fiber.Ctx) error {
		c.Locals("oauth.subject_type", "admin")
		return handler.BeginOAuthLogin(c)
	})
	publicOAuth.Get("/:provider/callback", handler.OAuthCallback)
	publicOAuth.Post("/exchange", handler.ExchangeOAuthLoginCode)

	userAuthBase := v1.Group("/user/auth", jwt.NewJWTMiddleware(config.App.Secret), middleware.DomainMiddleware(), func(c fiber.Ctx) error {
		c.Locals("service.OAuthAuthService", oauthAuthService)
		return c.Next()
	}, middleware.BlacklistCheck(blacklistService, "user"))
	userOAuth := userAuthBase.Group("/oauth")
	userOAuth.Get("/identities", handler.ListUserOAuthIdentities)
	userOAuth.Get("/:provider/authorize", handler.BeginOAuthBind("user"))
	userOAuth.Delete("/:provider", handler.UnbindUserOAuthIdentity)

	// 管理员公开接口（无需认证）
	adminPublicLogin := admin.Group("/public", func(c fiber.Ctx) error {
		c.Locals("service.AdminService", service.NewAdminServiceWithBlacklist(blacklistService))
		c.Locals("utils.Response", utils.NewResponse())
		return c.Next()
	})
	adminPublicLogin.Post("/login", captchaAdapter.Require("admin_login"), loginLimiter.Handler(), handler.AdminLogin)
	adminPublicLogin.Post("/refresh", handler.AdminRefreshToken)

	pluginProxy := v1.Group("/plugin", func(c fiber.Ctx) error {
		c.Locals("service.PluginService", pluginService)
		return c.Next()
	})
	pluginPublic := func(c fiber.Ctx) error {
		c.Locals("plugin.proxy_path_prefix", "/public")
		c.Locals("plugin.proxy_point", pluginapi.ExtensionPublicRoutes)
		return handler.ProxyPlugin(c)
	}
	pluginProxy.All("/:plugin_id/public", pluginPublic)
	pluginProxy.All("/:plugin_id/public/*", pluginPublic)
	pluginUserAuth := []any{jwt.NewJWTMiddlewareWithConfig(jwt.JWTConfig{
		Secret: config.App.Secret,
		ErrorHandler: func(c fiber.Ctx, _ error) error {
			return c.SendStatus(fiber.StatusUnauthorized)
		},
	}), middleware.DomainMiddleware(), func(c fiber.Ctx) error {
		claims := jwt.GetJWTUserData(c)
		userID, ok := claims["id"].(string)
		expires, validExpiry := claims["expires"].(float64)
		if !ok || userID == "" || claims["token_type"] != "access" ||
			!validExpiry || expires <= float64(time.Now().Unix()) {
			return c.SendStatus(fiber.StatusUnauthorized)
		}
		c.Locals("plugin.user_id", userID)
		c.Locals("plugin.proxy_path_prefix", "/user/auth")
		c.Locals("plugin.proxy_point", pluginapi.ExtensionUserRoutes)
		return c.Next()
	}, middleware.BlacklistCheck(blacklistService, "user"), handler.ProxyPlugin}
	pluginProxy.All("/:plugin_id/user/auth", pluginUserAuth[0], pluginUserAuth[1:]...)
	pluginProxy.All("/:plugin_id/user/auth/*", pluginUserAuth[0], pluginUserAuth[1:]...)
	pluginProxyAuth := []any{jwt.NewJWTMiddleware(config.App.AdminSecret), middleware.DomainMiddleware(), func(c fiber.Ctx) error {
		c.Locals("service.PermissionService", permissionService)
		c.Locals("plugin.proxy_path_prefix", "/admin/auth")
		c.Locals("plugin.proxy_point", cmfplugin.ExtensionAdminRoutes)
		return c.Next()
	}, middleware.BlacklistCheck(blacklistService, "admin"), middleware.PermissionMiddleware(), handler.ProxyPlugin}
	pluginProxy.All("/:plugin_id/admin/auth", pluginProxyAuth[0], pluginProxyAuth[1:]...)
	pluginProxy.All("/:plugin_id/admin/auth/*", pluginProxyAuth[0], pluginProxyAuth[1:]...)
	// 安全基线：通配代理路由与 /admin/auth 子路由使用同一套鉴权链，
	// 缺少 PermissionMiddleware 会让任意已认证低权限管理员绕过 RBAC 访问插件接口。
	// 注意：此路由不设置 plugin.proxy_path_prefix，避免给任意子路径错误拼上 /admin/auth 前缀。
	pluginProxy.All("/:plugin_id/*", jwt.NewJWTMiddleware(config.App.AdminSecret), middleware.DomainMiddleware(), func(c fiber.Ctx) error {
		c.Locals("service.PermissionService", permissionService)
		c.Locals("plugin.proxy_point", cmfplugin.ExtensionAdminRoutes)
		return c.Next()
	}, middleware.BlacklistCheck(blacklistService, "admin"), middleware.PermissionMiddleware(), handler.ProxyPlugin)

	// 需要认证的接口
	adminAuthBase := admin.Group("/auth", jwt.NewJWTMiddleware(config.App.AdminSecret), middleware.DomainMiddleware(), func(c fiber.Ctx) error {
		c.Locals("audit_started_at", time.Now())
		c.Locals("service.SuperAdminService", service.NewSuperAdminService(dataPermissionService, departmentService))
		c.Locals("service.AdminService", service.NewAdminServiceWithBlacklist(blacklistService))
		c.Locals("service.UserService", userService)
		c.Locals("service.RoleService", roleService)
		c.Locals("service.PermissionService", permissionService)
		c.Locals("service.SystemPermissionService", systemPermissionService)
		c.Locals("service.OperationLogService", opLogService)
		c.Locals("service.LoginLogService", loginLogService)
		c.Locals("service.UserListService", userListService)
		c.Locals("service.UserActivationService", userActivationService)
		c.Locals("service.BlacklistService", blacklistService)
		c.Locals("service.ThirdPartyUserService", thirdPartyUserService)
		c.Locals("service.RoleAssignmentService", roleAssignmentService)
		c.Locals("service.MessageService", messageService)
		c.Locals("service.MessagePreferenceService", prefSvc)
		c.Locals("service.MessageBroadcastService", broadcastService)
		c.Locals("service.PluginService", pluginService)
		c.Locals("service.DepartmentService", departmentService)
		c.Locals("service.DataPermissionService", dataPermissionService)
		c.Locals("service.OAuthProviderConfigService", oauthConfigService)
		c.Locals("service.OAuthAuthService", oauthAuthService)
		c.Locals("sse.TicketStore", ticketStore)
		return c.Next()
	},
		// 关键管理操作的自动审计：真实状态码与耗时，拒绝请求同样留痕
		middleware.Audit(middleware.AdminAuditRules(), opLogService),
		middleware.BlacklistCheck(blacklistService, "admin"))

	adminAuthBase.Get("/user", handler.GetAdmin)
	adminAuthBase.Put("/user", handler.UpdateAdmin)
	adminAuthBase.Put("/password", handler.ChangeAdminPassword)
	adminAuthBase.Post("/logout", handler.AdminLogout)
	adminOAuth := adminAuthBase.Group("/oauth")
	adminOAuth.Get("/identities", handler.ListAdminOAuthIdentities)
	adminOAuth.Get("/:provider/authorize", handler.BeginOAuthBind("admin"))
	adminOAuth.Delete("/:provider", handler.UnbindAdminOAuthIdentity)

	// 需要认证+权限校验的接口
	adminAuth := adminAuthBase.Group("", middleware.PermissionMiddleware())
	adminAuth.Get("/oauth/providers", handler.ListOAuthProviderConfigs)
	adminAuth.Get("/oauth/providers/:provider", handler.GetOAuthProviderConfig)
	adminAuth.Put("/oauth/providers/:provider", handler.UpsertOAuthProviderConfig)
	adminAuth.Delete("/oauth/providers/:provider", handler.DeleteOAuthProviderConfig)

	// 超级管理员管理路由（管理其他管理员账号）
	captchaSettings := adminAuth.Group("/captcha", func(c fiber.Ctx) error {
		c.Locals("captcha.SettingsService", captchaRuntime.Settings)
		return c.Next()
	})
	captchaSettings.Get("/settings", handler.GetCaptchaSettings)
	captchaSettings.Put("/scenes/:scene", handler.UpdateCaptchaScene)
	captchaSettings.Put("/providers/:provider", handler.UpdateCaptchaProvider)
	captchaSettings.Post("/providers/:provider/test", handler.ValidateCaptchaProvider)

	adminAuth.Post("/admins", handler.CreateAdmin)
	adminAuth.Get("/admins", handler.ListAdmins)
	adminAuth.Get("/admins/:id", handler.GetAdminDetail)
	adminAuth.Put("/admins/:id", handler.UpdateAdminByID)
	adminAuth.Get("/admins/:id/roles", handler.GetAdminRoles)
	adminAuth.Put("/admins/:id/roles", handler.AssignAdminRoles)
	adminAuth.Put("/admins/:id/activate", handler.ActivateAdmin)
	adminAuth.Put("/admins/:id/deactivate", handler.DeactivateAdmin)
	// 角色管理路由
	roles := adminAuth.Group("/roles")
	roles.Post("", handler.CreateRole) // 创建角色

	// 角色权限管理路由（必须在 /:id 之前注册，避免被参数化路由捕获）
	roles.Get("/permissions", handler.GetRolePermissions)                // 获取角色权限
	roles.Post("/permissions/assign", handler.AssignPermissionsToRole)   // 为角色分配权限
	roles.Post("/permissions/remove", handler.RemovePermissionsFromRole) // 从角色移除权限

	roles.Get("/all", handler.ListAllRoles)                           // 获取所有角色（不带分页）
	roles.Get("/:id/permissions", handler.GetRolePermissionsByRoleID) // 按ID获取角色权限
	roles.Post("/:id/permissions", handler.SaveRolePermissions)       // 覆盖式保存角色权限
	roles.Get("/:id", handler.GetRole)                                // 获取角色详情
	roles.Get("", handler.ListRoles)                                  // 获取角色列表
	roles.Put("/:id", handler.UpdateRole)                             // 更新角色
	roles.Delete("/:id", handler.DeleteRole)                          // 删除角色
	roles.Put("/:id/enable", handler.EnableRole)                      // 启用角色
	roles.Put("/:id/disable", handler.DisableRole)                    // 禁用角色

	// 权限管理路由
	permissions := adminAuth.Group("/permissions")
	permissions.Post("", handler.CreatePermission)      // 创建权限
	permissions.Put("", handler.UpdatePermission)       // 更新权限
	permissions.Delete("", handler.DeletePermission)    // 删除权限
	permissions.Get("/detail", handler.GetPermission)   // 获取权限详情
	permissions.Get("/all", handler.ListAllPermissions) // 获取所有权限（不带分页）
	permissions.Get("", handler.ListPermissions)        // 获取权限列表

	// 权限定义管理路由
	systemPermissions := adminAuth.Group("/system-permissions")
	systemPermissions.Post("", handler.CreateSystemPermission)       // 创建权限定义
	systemPermissions.Get("", handler.ListSystemPermissions)         // 获取权限定义列表
	systemPermissions.Get("/", handler.ListSystemPermissions)        // 获取权限定义列表（兼容尾斜杠）
	systemPermissions.Get("/tree", handler.GetSystemPermissionTree)  // 获取权限树
	systemPermissions.Get("/:id", handler.GetSystemPermission)       // 获取权限定义详情
	systemPermissions.Put("/:id", handler.UpdateSystemPermission)    // 更新权限定义
	systemPermissions.Delete("/:id", handler.DeleteSystemPermission) // 删除权限定义

	// 部门管理路由
	departments := adminAuth.Group("/departments")
	departments.Post("", handler.CreateDepartment)
	departments.Get("/tree", handler.ListDepartmentTree)
	departments.Get("/:id", handler.GetDepartment)
	departments.Put("/:id", handler.UpdateDepartment)
	departments.Delete("/:id", handler.DeleteDepartment)

	// 用户权限路由
	userPermissions := adminAuth.Group("/user")
	userPermissions.Get("/menu-permissions", handler.GetUserMenuPermissions)     // 获取用户菜单权限
	userPermissions.Get("/button-permissions", handler.GetUserButtonPermissions) // 获取用户按钮权限
	userPermissions.Get("/api-permissions", handler.GetUserAPIPermissions)       // 获取用户API权限
	userPermissions.Get("/permissions", handler.GetUserPermissions)              // 获取用户所有权限（菜单和按钮）

	// 定时任务管理路由
	cronJobService := service.NewCronJobService(global.CronManager)
	cronJobs := adminAuth.Group("/cron-jobs", func(c fiber.Ctx) error {
		c.Locals("service.CronJobService", cronJobService)
		return c.Next()
	})
	cronJobs.Post("", handler.CreateCronJob)                          // 创建定时任务
	cronJobs.Get("", handler.ListCronJobs)                            // 获取定时任务列表
	cronJobs.Get("/:id", handler.GetCronJob)                          // 获取定时任务详情
	cronJobs.Put("/:id", handler.UpdateCronJob)                       // 更新定时任务
	cronJobs.Delete("/:id", handler.DeleteCronJob)                    // 删除定时任务
	cronJobs.Post("/:id/enable", handler.EnableCronJob)               // 启用定时任务
	cronJobs.Post("/:id/disable", handler.DisableCronJob)             // 禁用定时任务
	cronJobs.Post("/:id/run", handler.RunCronJob)                     // 立即执行
	cronJobs.Get("/:id/logs", handler.GetCronJobLogs)                 // 获取日志列表
	cronJobs.Get("/:id/logs/tail", handler.TailCronJobLog)            // 读取日志尾部
	cronJobs.Get("/:id/logs/:filename", handler.GetCronJobLogContent) // 获取日志内容

	// 任务队列管理路由（静态路径先于参数化路径注册）
	mountQueueJobs(adminAuth)

	// 任务队列监控页（asynqmon 只读 UI，默认关闭；开启时挂载在管理端认证之后）
	if config.Queue.EnableMonitor {
		if qcfg, ok := adminqueue.EngineConfig(); ok {
			monitorPath := config.Queue.MonitorPath
			if err := mountQueueMonitor(adminAuth, monitorPath, cmfqueue.MonitorHandler(qcfg, monitorPath)); err != nil {
				panic(err)
			}
		} else {
			log.Warn("任务队列未初始化，监控页未挂载")
		}
	}

	// 插件管理路由
	// 注意：静态路径 /install 必须在参数化路由 /:plugin_id 之前注册
	plugins := adminAuth.Group("/plugins")
	plugins.Post("/install", handler.InstallPlugin)
	plugins.Get("", handler.ListPlugins)
	plugins.Get("/:plugin_id", handler.GetPlugin)
	plugins.Post("/:plugin_id/enable", handler.EnablePlugin)
	plugins.Post("/:plugin_id/disable", handler.DisablePlugin)
	plugins.Delete("/:plugin_id", handler.UninstallPlugin)

	// 附件分组管理路由
	attachmentGroupService := service.NewAttachmentGroupService()
	var attachmentService service.AttachmentServiceInterface
	if appInstance != nil && appInstance.Filesystem != nil {
		attachmentService = service.NewAttachmentService(appInstance.Filesystem)
	} else {
		attachmentService = service.NewAttachmentService(nil)
	}

	groups := adminAuth.Group("/attachment-groups", func(c fiber.Ctx) error {
		c.Locals("service.AttachmentGroupService", attachmentGroupService)
		return c.Next()
	})
	groups.Post("/", handler.CreateAttachmentGroup)
	groups.Get("/", handler.ListAttachmentGroups)
	groups.Get("/:id", handler.GetAttachmentGroup)
	groups.Put("/:id", handler.UpdateAttachmentGroup)
	groups.Delete("/:id", handler.DeleteAttachmentGroup)

	// 附件管理路由
	attachments := adminAuth.Group("/attachments", func(c fiber.Ctx) error {
		c.Locals("service.AttachmentService", attachmentService)
		c.Locals("attachment.access_secret", config.App.AdminSecret)
		c.Locals("attachment.content_base_path", "/api/v1/attachments")
		c.Locals("attachment.access_ttl", config.Attachment.AccessURLTTL)
		c.Locals("attachment.x_accel_enabled", config.Attachment.XAccelEnabled)
		return c.Next()
	})
	attachments.Post("/upload", handler.UploadAttachment)
	// 分片上传查询与取消路由（上传已统一到 /upload）
	attachments.Get("/chunk/status/:upload_id", handler.GetChunkUploadStatus)
	attachments.Delete("/chunk/:upload_id", handler.CancelChunkUpload)
	attachments.Delete("/batch", handler.BatchDeleteAttachments)
	attachments.Get("/", handler.ListAttachments)
	attachments.Get("/:id", handler.GetAttachment)
	attachments.Get("/:id/access-url", handler.GetAttachmentAccessURL)
	attachments.Get("/:id/preview", handler.PreviewAttachment)
	attachments.Delete("/:id", handler.DeleteAttachment)
	attachments.Put("/:id/move", handler.MoveAttachment)

	attachmentContent := v1.Group("/attachments", func(c fiber.Ctx) error {
		c.Locals("service.AttachmentService", attachmentService)
		c.Locals("attachment.access_secret", config.App.AdminSecret)
		c.Locals("attachment.access_ttl", config.Attachment.AccessURLTTL)
		c.Locals("attachment.x_accel_enabled", config.Attachment.XAccelEnabled)
		return c.Next()
	})
	attachmentContent.Get("/:id/content", handler.ServeAttachmentContent)

	// 前台统一上传路由（小程序/App/Web）：复用管理端附件上传服务与全部校验逻辑，
	// 身份来自用户JWT（config.App.Secret）+ UserIdentity，upload_subject 区分归属域，
	// 分片会话属主校验防止跨用户/跨域续传、取消与合并
	uploadLimiter := middleware.NewRateLimiter(middleware.RateLimiterConfig{
		Window:   1 * time.Minute,
		MaxCount: 120, // 兼容分片上传的逐片请求，同时约束滥用
		KeyFunc: func(c fiber.Ctx) string {
			if uid, ok := c.Locals("uid").(string); ok && uid != "" {
				return "upload:" + uid
			}
			return "upload:" + c.IP()
		},
	})
	frontAuth := v1.Group("/auth",
		jwt.NewJWTMiddleware(config.App.Secret),
		middleware.DomainMiddleware(),
		middleware.UserIdentity(),
		middleware.BlacklistCheck(blacklistService, "user"),
		uploadLimiter.Handler(),
		func(c fiber.Ctx) error {
			c.Locals("service.AttachmentService", attachmentService)
			// access_secret 与 /api/v1/attachments/:id/content 端点保持一致，签名访问地址互通
			c.Locals("attachment.access_secret", config.App.AdminSecret)
			c.Locals("attachment.content_base_path", "/api/v1/attachments")
			c.Locals("attachment.access_ttl", config.Attachment.AccessURLTTL)
			c.Locals("attachment.x_accel_enabled", config.Attachment.XAccelEnabled)
			c.Locals("attachment.upload_subject", "user")
			c.Locals("service.OperationLogService", opLogService)
			return c.Next()
		})
	frontAttachments := frontAuth.Group("/attachments")
	frontAttachments.Post("/upload", handler.UploadAttachment)
	frontAttachments.Get("/chunk/status/:upload_id", handler.GetChunkUploadStatus)
	frontAttachments.Delete("/chunk/:upload_id", handler.CancelChunkUpload)

	// 实时通知凭证（用户端）：颁发自己 uid 的 ticket，供 /api/v1/realtime 建连
	frontAuth.Post("/realtime/ticket", ws.IssueTicket)

	// 操作日志管理路由
	opLogs := adminAuth.Group("/operation-logs")
	opLogs.Get("/", handler.ListOperationLogs)
	opLogs.Get("/:id", handler.GetOperationLog)

	// 登录日志管理路由
	loginLogs := adminAuth.Group("/login-logs")
	loginLogs.Get("", handler.ListLoginLogs)
	loginLogs.Get("/", handler.ListLoginLogs)
	loginLogs.Get("/:id", handler.GetLoginLog)

	// 数据字典路由
	dictDataService := service.NewDictDataService()
	dictData := adminAuth.Group("/dict-data", func(c fiber.Ctx) error {
		c.Locals("service.DictDataService", dictDataService)
		return c.Next()
	})
	dictData.Get("/type/:type", handler.GetDictDataByType)
	dictData.Post("", handler.CreateDictData)
	dictData.Get("", handler.ListDictData)
	dictData.Get("/:id", handler.GetDictData)
	dictData.Put("/:id", handler.UpdateDictData)
	dictData.Delete("/:id", handler.DeleteDictData)

	// 幻灯片管理路由
	slideService := service.NewSlideService()
	slides := adminAuth.Group("/slides", func(c fiber.Ctx) error {
		c.Locals("service.SlideService", slideService)
		return c.Next()
	})
	slides.Post("", handler.CreateSlide)
	slides.Get("", handler.ListSlides)
	slides.Get("/:id", handler.GetSlide)
	slides.Put("/:id", handler.UpdateSlide)
	slides.Delete("/:id", handler.DeleteSlide)

	// 幻灯片分组管理路由(分组数据复用 slide_group 字典)
	// 使用独立前缀避免与 /slides/:id 的动态参数冲突
	slideGroupService := service.NewSlideGroupService()
	slideGroups := adminAuth.Group("/slide-groups", func(c fiber.Ctx) error {
		c.Locals("service.SlideGroupService", slideGroupService)
		return c.Next()
	})
	slideGroups.Get("", handler.ListSlideGroups)
	slideGroups.Post("", handler.CreateSlideGroup)
	slideGroups.Put("/:id", handler.UpdateSlideGroup)
	slideGroups.Delete("/:id", handler.DeleteSlideGroup)

	// 公开幻灯片接口(免登录,前台按分组拉取)
	public.Get("/slides", func(c fiber.Ctx) error {
		c.Locals("service.SlideService", slideService)
		return c.Next()
	}, handler.ListPublicSlides)

	// 动态配置路由
	var configService *service.ConfigService
	if appInstance != nil && appInstance.Cache != nil {
		configService = service.NewConfigService(appInstance.Cache)
	} else {
		configService = service.NewConfigService(nil)
	}
	configs := adminAuth.Group("/configs", func(c fiber.Ctx) error {
		c.Locals("service.ConfigService", configService)
		return c.Next()
	})
	configs.Get("/groups", handler.GetConfigGroups)
	configs.Get("/by-key/:key", handler.GetConfigByKey)
	configs.Get("/group/:group", handler.GetConfigsByGroup)
	configs.Post("", handler.CreateConfig)
	configs.Get("", handler.ListConfigs)
	configs.Get("/:id", handler.GetConfig)
	configs.Put("/:id", handler.UpdateConfig)
	configs.Delete("/:id", handler.DeleteConfig)

	// 用户管理路由
	users := adminAuth.Group("/users")
	users.Post("", handler.CreateUser)
	users.Get("", handler.ListUsers)
	users.Post("/export", handler.ExportUsers)
	users.Get("/:id", handler.GetUserDetail)
	users.Put("/:id", handler.UpdateManagedUser)
	users.Delete("/:id", handler.DeleteManagedUser)
	users.Put("/:id/activate", handler.ActivateUser)
	users.Put("/:id/deactivate", handler.DeactivateUser)
	users.Post("/:id/blacklist", handler.AddToBlacklist)
	users.Delete("/:id/blacklist", handler.RemoveFromBlacklist)
	users.Get("/:id/roles", handler.GetUserRoles)
	users.Put("/:id/roles", handler.AssignUserRoles)
	users.Get("/:id/third-party", handler.ListUserThirdParty)

	// 黑名单路由
	blacklists := adminAuth.Group("/blacklist")
	blacklists.Get("", handler.ListBlacklist)

	// 第三方用户管理路由
	thirdParty := adminAuth.Group("/third-party-users")
	thirdParty.Get("", handler.ListThirdPartyUsers)
	thirdParty.Delete("/:id", handler.UnbindThirdPartyUser)
	adminThirdParty := adminAuth.Group("/third-party-admins")
	adminThirdParty.Get("", handler.ListAdminThirdPartyIdentities)
	adminThirdParty.Delete("/:id", handler.DeleteAdminThirdPartyIdentity)

	// 站内消息路由（鉴权后访问）
	// 注意：所有静态路径（/preference、/unread-count 等）必须在 /:id 之前注册，
	// 避免被参数化路由捕获导致 strconv.Atoi 失败
	messages := adminAuth.Group("/messages")
	messages.Get("", handler.ListMessages)
	// 静态路径在前
	messages.Post("/read", handler.MarkRead)
	messages.Post("/read-all", handler.MarkAllRead)
	messages.Post("/unread", handler.MarkUnread)
	messages.Get("/unread-count", handler.UnreadCount)
	messages.Post("/delete-batch", handler.DeleteBatchMessages)
	messages.Post("/clear", handler.ClearMessages)
	messages.Get("/preference", handler.GetMessagePreference)
	messages.Put("/preference", handler.UpdateMessagePreference)
	messages.Post("/broadcast", handler.BroadcastMessage)
	messages.Post("/test-push", handler.TestPushMessage)
	// 参数化路由放最后
	messages.Get("/:id", handler.GetMessage)
	messages.Delete("/:id", handler.DeleteMessage)

	// SSE 凭证接口（鉴权后访问）
	adminAuth.Post("/sse/ticket", handler.IssueSSETicket)

	// 实时通知凭证接口：只颁发自己的 uid，无需额外 RBAC 授权
	adminAuthBase.Post("/realtime/ticket", ws.IssueTicket)

	// SSE 连接接口（不走 BlacklistCheck/Permission，ticket 鉴权独立）
	sseGroup := admin.Group("/sse", func(c fiber.Ctx) error {
		c.Locals("utils.Response", utils.NewResponse())
		c.Locals("service.MessageService", messageService)
		c.Locals("sse.Hub", hub)
		c.Locals("sse.TicketStore", ticketStore)
		return c.Next()
	})
	sseGroup.Get("/connect", handler.SSEConnect)

	// WS 连接端点：ticket 在升级前校验，失败返回 401，不升级
	v1.Get("/realtime", ws.Connect)
	if err := apps.Mount(v1, config, blacklistService); err != nil {
		panic("mount source feature sets: " + err.Error())
	}
}

func validatePluginTrustPolicy(environment string, requireSignature bool, trustedKeyCount int) error {
	if strings.EqualFold(strings.TrimSpace(environment), "production") && !requireSignature {
		return errors.New("plugin signatures must be enabled in production")
	}
	if requireSignature && trustedKeyCount == 0 {
		return errors.New("signatures are required but no trusted public key is configured")
	}
	return nil
}
