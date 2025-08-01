package app

import (
	"log"
	"restapp/config"
	"restapp/internal/database"
	"restapp/internal/delivery/rest"
	"restapp/internal/middlewares"
	"restapp/internal/repositories"
	"restapp/internal/services"

	"github.com/labstack/echo-contrib/echoprometheus"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"golang.org/x/time/rate"
)

type App struct {
	cfgPath string
}

func NewApp(cfgPath string) *App {
	return &App{
		cfgPath: cfgPath,
	}
}

func (a *App) Run() {
	e := echo.New()

	e.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
		Format: "${time_rfc3339} ${method} ${uri} ${status} ${latency_human}\n",
	}))

	e.Use(middleware.Recover())
	e.Use(echoprometheus.NewMiddleware("article_hub"))
	e.Use(middleware.RateLimiter(middleware.NewRateLimiterMemoryStore(rate.Limit(10))))

	cfg := config.MustLoad(a.cfgPath)
	err := database.InitDB(cfg)
	if err != nil {
		panic(err)
	}
	db := database.GetDB()

	articleRepo := repositories.NewArticleRepository(db)
	authRepo := repositories.NewAuthRepository(db)
	commentRepo := repositories.NewCommentRepository(db)

	articleService := services.NewArticleService(articleRepo)
	authService := services.NewAuthService(authRepo, cfg)
	commentService := services.NewCommentService(commentRepo)

	articleHandler := rest.NewArticleHandler(articleService, authService, commentService)
	authHandler := rest.NewAuthHandler(authService)
	commentHandler := rest.NewCommentHandler(commentService, authService)

	authMiddleware := middlewares.NewAuthMiddleware(authService)

	e.GET("/metrics", echoprometheus.NewHandler())

	auth := e.Group("/auth")
	auth.POST("/register", authHandler.Register)
	auth.POST("/login", authHandler.Login)

	articles := e.Group("/articles")
	articles.Use(authMiddleware.AuthMiddleware)
	articles.GET("", articleHandler.GetAllArticles)
	articles.GET("/:id", articleHandler.GetById)
	articles.POST("", articleHandler.StoreArticle)
	articles.PUT("/:id", articleHandler.UpdateArticle)
	articles.DELETE("/:id", articleHandler.DeleteArticle)
	articles.GET("/:id/like", articleHandler.LikeArticle)
	articles.GET("/:id/unlike", articleHandler.UnlikeArticle)

	articles.POST("/:id/comments", commentHandler.CreateComment)

	log.Println("Server start")
	err = e.Start(":" + cfg.Server.Port)
	if err != nil {
		panic(err)
	}
}
