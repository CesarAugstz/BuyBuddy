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
	shoppingListRepo := repository.NewShoppingListRepository(db)

	authHandler := handlers.NewAuthHandler(cfg, userRepo)
	receiptHandler := handlers.NewReceiptHandler(cfg, receiptRepo, categoryRepo)
	assistantHandler := handlers.NewAssistantHandler(cfg, receiptRepo, chatRepo, prefsRepo, categoryRepo)
	preferencesHandler := handlers.NewPreferencesHandler(prefsRepo)
	shoppingListHandler := handlers.NewShoppingListHandler(shoppingListRepo, userRepo)

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

	shoppingLists := api.Group("/shopping-lists")
	shoppingLists.Use(middleware.AuthMiddleware(cfg.JWTSecret))
	
	// Static routes MUST be registered before parameterized routes
	shoppingLists.GET("", shoppingListHandler.GetLists)
	shoppingLists.POST("", shoppingListHandler.CreateList)
	shoppingLists.GET("/suggestions", shoppingListHandler.GetSuggestions)
	shoppingLists.GET("/invites", shoppingListHandler.GetInvites)
	shoppingLists.PUT("/invites/:inviteId/accept", shoppingListHandler.AcceptInvite)
	shoppingLists.PUT("/invites/:inviteId/reject", shoppingListHandler.RejectInvite)
	
	// Parameterized routes
	shoppingLists.GET("/:id", shoppingListHandler.GetList)
	shoppingLists.PUT("/:id", shoppingListHandler.UpdateList)
	shoppingLists.DELETE("/:id", shoppingListHandler.DeleteList)
	shoppingLists.POST("/:id/items", shoppingListHandler.AddItem)
	shoppingLists.PUT("/:id/items/:itemId", shoppingListHandler.UpdateItem)
	shoppingLists.DELETE("/:id/items/:itemId", shoppingListHandler.DeleteItem)
	shoppingLists.PUT("/:id/items/reorder", shoppingListHandler.ReorderItems)
	shoppingLists.POST("/:id/share", shoppingListHandler.ShareList)
	shoppingLists.GET("/:id/shares", shoppingListHandler.GetListShares)
	shoppingLists.DELETE("/:id/share/:userId", shoppingListHandler.RemoveShare)

	users := api.Group("/users")
	users.Use(middleware.AuthMiddleware(cfg.JWTSecret))
	users.GET("/search", shoppingListHandler.SearchUsers)
}
