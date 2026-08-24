package main

import (
	"buybuddy-api/config"
	"buybuddy-api/database"
	"buybuddy-api/middleware"
	"buybuddy-api/models"
	"buybuddy-api/repository"
	"buybuddy-api/routes"
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	cfg := config.Load()

	if err := database.Connect(cfg); err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	if err := database.Migrate(&models.User{}, &models.Session{}, &models.Category{}, &models.Subcategory{}, &models.Receipt{}, &models.ReceiptItem{}, &models.ChatMessage{}, &models.UserPreferences{}, &models.ShoppingList{}, &models.ShoppingListItem{}, &models.ShoppingListShare{}, &models.KnowledgeTopic{}, &models.KnowledgeEntry{}, &models.KnowledgeEntryRevision{}); err != nil {
		log.Fatal("Failed to migrate database:", err)
	}
	if err := database.MigrateKnowledgeIndexes(); err != nil {
		log.Fatal("Failed to migrate knowledge indexes:", err)
	}

	categoryRepo := repository.NewCategoryRepository(database.DB)
	if err := categoryRepo.SeedDefaultCategories(); err != nil {
		log.Println("Warning: Failed to seed default categories:", err)
	}

	e := echo.New()

	e.Use(echomiddleware.Logger())
	e.Use(echomiddleware.Recover())
	e.Use(middleware.CORS(cfg.CORSOrigins))

	knowledgeOrganizer := routes.Setup(e, cfg, database.DB)

	log.Printf("Starting server on port %s", cfg.Port)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if cfg.KnowledgeOrganizerEnabled {
		log.Print("Knowledge organizer ticker enabled")
		go knowledgeOrganizer.RunTicker(ctx)
	}

	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- e.Start(":" + cfg.Port)
	}()

	select {
	case <-ctx.Done():
	case err := <-serverErrors:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("Server stopped unexpectedly: %v", err)
		}
		stop()
	}

	shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := e.Shutdown(shutdownContext); err != nil {
		log.Printf("Server shutdown failed: %v", err)
	}
}
