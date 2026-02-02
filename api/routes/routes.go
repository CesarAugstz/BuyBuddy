package routes

import (
	"easybuy-api/config"
	"easybuy-api/handlers"
	"easybuy-api/middleware"
	"easybuy-api/repository"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

func Setup(e *echo.Echo, cfg *config.Config, db *gorm.DB) {
	userRepo := repository.NewUserRepository(db)
	receiptRepo := repository.NewReceiptRepository(db)
	categoryRepo := repository.NewCategoryRepository(db)

	authHandler := handlers.NewAuthHandler(cfg, userRepo)
	receiptHandler := handlers.NewReceiptHandler(cfg, receiptRepo, categoryRepo)

	e.GET("/health", handlers.Health)

	api := e.Group("/api")

	auth := api.Group("/auth")
	auth.POST("/login", authHandler.Login)
	auth.POST("/logout", authHandler.Logout)
	auth.GET("/verify", authHandler.Verify, middleware.AuthMiddleware(cfg.JWTSecret))

	receipts := api.Group("/receipts")
	receipts.Use(middleware.AuthMiddleware(cfg.JWTSecret))
	receipts.POST("/process", receiptHandler.ProcessReceipt)
	receipts.POST("", receiptHandler.SaveReceipt)
	receipts.GET("", receiptHandler.GetReceipts)
	receipts.GET("/:id", receiptHandler.GetReceipt)
	receipts.DELETE("/:id", receiptHandler.DeleteReceipt)
}
