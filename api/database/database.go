package database

import (
	"buybuddy-api/config"
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

func MigrateKnowledgeIndexes() error {
	statements := []string{
		`CREATE INDEX IF NOT EXISTS idx_knowledge_entries_tags ON knowledge_entries USING GIN (tags)`,
		`CREATE INDEX IF NOT EXISTS idx_knowledge_entries_attributes ON knowledge_entries USING GIN (attributes)`,
		`CREATE INDEX IF NOT EXISTS idx_knowledge_entries_search ON knowledge_entries USING GIN (to_tsvector('simple', coalesce(title, '') || ' ' || coalesce(body, '')))`,
	}
	for _, statement := range statements {
		if err := DB.Exec(statement).Error; err != nil {
			return err
		}
	}
	return nil
}
