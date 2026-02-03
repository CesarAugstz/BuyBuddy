package main

import (
	"buybuddy-api/config"
	"buybuddy-api/database"
	"buybuddy-api/middleware"
	"buybuddy-api/models"
	"buybuddy-api/repository"
	"buybuddy-api/routes"
	"log"

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

	if err := database.Migrate(&models.User{}, &models.Session{}, &models.Category{}, &models.Subcategory{}, &models.Receipt{}, &models.ReceiptItem{}, &models.ChatMessage{}, &models.UserPreferences{}); err != nil {
		log.Fatal("Failed to migrate database:", err)
	}

	categoryRepo := repository.NewCategoryRepository(database.DB)
	if err := categoryRepo.SeedDefaultCategories(); err != nil {
		log.Println("Warning: Failed to seed default categories:", err)
	}

	e := echo.New()

	e.Use(echomiddleware.Logger())
	e.Use(echomiddleware.Recover())
	e.Use(middleware.CORS(cfg.CORSOrigins))

	routes.Setup(e, cfg, database.DB)

	log.Printf("Starting server on port %s", cfg.Port)
	if err := e.Start(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}
