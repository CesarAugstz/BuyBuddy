package database

import (
	"easybuy-api/config"
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func Connect(cfg *config.Config) error {
	var err error

	gormConfig := &gorm.Config{}
	if cfg.Environment == "development" {
		gormConfig.Logger = logger.Default.LogMode(logger.Info)
	} else {
		gormConfig.Logger = logger.Default.LogMode(logger.Error)
	}

	DB, err = gorm.Open(postgres.Open(cfg.Database.ConnectionString()), gormConfig)

	if err != nil {
		return err
	}

	log.Println("Database connected successfully")
	return nil
}

func Migrate(models ...interface{}) error {
	return DB.AutoMigrate(models...)
}
