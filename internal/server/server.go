package server

import (
	"database/sql"
	"embed"
	"html/template"
	"io/fs"
	"net/http"

	"github.com/gin-gonic/gin"

	"private-chat/internal/auth"
	"private-chat/internal/cleanup"
	"private-chat/internal/config"
	"private-chat/internal/db"
	"private-chat/internal/file"
	"private-chat/internal/logger"
	"private-chat/internal/repo"
	"private-chat/internal/ws"
)

//go:embed all:web/templates
var templateFS embed.FS

//go:embed all:web/static
var staticFS embed.FS

// App 聚合所有依赖，作为 HTTP/WS 处理上下文。
type App struct {
	cfg     *config.Config
	db      *sql.DB
	users   *repo.UserRepo
	session *repo.SessionRepo
	messages *repo.MessageRepo
	files   *repo.FileRepo
	tasks   *repo.CleanupTaskRepo

	authSvc  *auth.Service
	fileSvc  *file.Service
	hub      *ws.Hub
	cleaner  *cleanup.Worker
	limiter  *RateLimiter
}

// New 构建应用并初始化子服务。
func New(cfg *config.Config) (*App, error) {
	database, err := db.Open(cfg.Database.Path)
	if err != nil {
		return nil, err
	}
	users := repo.NewUserRepo(database)
	session := repo.NewSessionRepo(database)
	messages := repo.NewMessageRepo(database)
	files := repo.NewFileRepo(database)
	tasks := repo.NewCleanupTaskRepo(database)

	authSvc := auth.NewService(cfg, users, session)
	fileSvc := file.NewService(files, cfg.Storage.UploadDir, cfg.Storage.MaxImageSize, cfg.Storage.MaxFileSize)

	app := &App{
		cfg:      cfg,
		db:       database,
		users:    users,
		session:  session,
		messages: messages,
		files:    files,
		tasks:    tasks,
		authSvc:  authSvc,
		fileSvc:  fileSvc,
		limiter:  NewRateLimiter(),
	}
	app.hub = ws.NewHub(app) // app 实现 ws.MessageHandler
	app.cleaner = cleanup.New(messages, files, tasks, cfg.Retention())

	if err := app.ensureAdmin(); err != nil {
		return nil, err
	}
	return app, nil
}

// DB 返回底层数据库（供测试/健康检查使用）。
func (app *App) DB() *sql.DB { return app.db }

// Hub 返回 WebSocket Hub。
func (app *App) Hub() *ws.Hub { return app.hub }

// StartBackground 启动后台任务。
func (app *App) StartBackground() {
	go app.hub.Run()
	app.cleaner.Start()
}

// Stop 停止后台任务。
func (app *App) Stop() {
	app.cleaner.Stop()
}

// Router 构建 Gin 引擎并注册路由。
func (app *App) Router() *gin.Engine {
	if app.cfg.Log.Format == "json" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Recovery())

	// 模板与静态资源（自包含于二进制）。
	tmpl := template.Must(template.ParseFS(templateFS, "web/templates/*.html"))
	r.SetHTMLTemplate(tmpl)
	sub, _ := fs.Sub(staticFS, "web/static")
	r.StaticFS("/static", http.FS(sub))

	// 页面路由
	r.GET("/", app.handleIndex)
	r.GET("/login", app.handleIndex)
	r.GET("/chat", app.authSvc.AuthMiddleware(), app.handleChat)
	r.GET("/admin", app.authSvc.AuthMiddleware(), app.authSvc.AdminMiddleware(), app.handleAdmin)

	// 健康检查（无需认证）
	r.GET("/health", app.handleHealth)

	api := r.Group("/api")
	{
		// 认证
		api.POST("/auth/login", app.handleLogin)
		api.POST("/auth/logout", app.authSvc.AuthMiddleware(), app.handleLogout)
		api.GET("/auth/me", app.authSvc.AuthMiddleware(), app.handleMe)

		// 需要登录
		authed := api.Group("")
		authed.Use(app.authSvc.AuthMiddleware())
		{
			authed.GET("/messages", app.handleGetMessages)
			authed.POST("/messages", app.handlePostMessage)
			authed.GET("/users", app.handleListUsers)
			authed.POST("/files/upload", app.handleUpload)
			authed.GET("/files/:id", app.handleServeFile)
			authed.GET("/files/:id/download", app.handleDownloadFile)
		}

		// 管理后台
		admin := api.Group("/admin")
		admin.Use(app.authSvc.AuthMiddleware(), app.authSvc.AdminMiddleware())
		{
			admin.GET("/users", app.handleAdminListUsers)
			admin.POST("/users", app.handleAdminCreateUser)
			admin.PUT("/users/:id", app.handleAdminUpdateUser)
			admin.DELETE("/users/:id", app.handleAdminDeleteUser)
			admin.POST("/users/:id/reset-session", app.handleAdminResetSession)
			admin.GET("/stats", app.handleAdminStats)
		}
	}

	// WebSocket
	r.GET("/ws", ws.ServeWS(app.hub, app.authSvc))
	return r
}

// Run 启动 HTTP 服务（阻塞）。
func (app *App) Run() error {
	srv := &http.Server{
		Addr:    app.cfg.Addr(),
		Handler: app.Router(),
	}
	logger.Info("server starting", map[string]interface{}{"addr": app.cfg.Addr()})
	return srv.ListenAndServe()
}
