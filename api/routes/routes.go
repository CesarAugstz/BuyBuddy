package routes

import (
	"buybuddy-api/config"
	"buybuddy-api/handlers"
	"buybuddy-api/middleware"
	"buybuddy-api/repository"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

func Setup(e *echo.Echo, cfg *config.Config, db *gorm.DB) {
	userRepo := repository.NewUserRepository(db)
	receiptRepo := repository.NewReceiptRepository(db)
	categoryRepo := repository.NewCategoryRepository(db)
	chatRepo := repository.NewChatRepository(db)
	prefsRepo := repository.NewPreferencesRepository(db)

	authHandler := handlers.NewAuthHandler(cfg, userRepo)
	receiptHandler := handlers.NewReceiptHandler(cfg, receiptRepo, categoryRepo)
	assistantHandler := handlers.NewAssistantHandler(cfg, receiptRepo, chatRepo, prefsRepo, categoryRepo)
	preferencesHandler := handlers.NewPreferencesHandler(prefsRepo)

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

	assistant := api.Group("/assistant")
	assistant.Use(middleware.AuthMiddleware(cfg.JWTSecret))
	assistant.POST("/ask", assistantHandler.AskQuestion)
	assistant.GET("/conversation/:conversationId", assistantHandler.GetConversationHistory)

	preferences := api.Group("/preferences")
	preferences.Use(middleware.AuthMiddleware(cfg.JWTSecret))
	preferences.GET("", preferencesHandler.GetPreferences)
	preferences.PUT("", preferencesHandler.UpdatePreferences)
	preferences.GET("/models", preferencesHandler.GetAvailableModels)
}
