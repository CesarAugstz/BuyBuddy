package repository

import (
	"buybuddy-api/models"

	"gorm.io/gorm"
)

type PreferencesRepository struct {
	db *gorm.DB
}

func NewPreferencesRepository(db *gorm.DB) *PreferencesRepository {
	return &PreferencesRepository{db: db}
}

func (r *PreferencesRepository) GetByUserID(userID string) (*models.UserPreferences, error) {
	var prefs models.UserPreferences
	err := r.db.Where("user_id = ?", userID).First(&prefs).Error
	if err != nil {
		return nil, err
	}
	return &prefs, nil
}

func (r *PreferencesRepository) Create(prefs *models.UserPreferences) error {
	return r.db.Create(prefs).Error
}

func (r *PreferencesRepository) Update(prefs *models.UserPreferences) error {
	return r.db.Save(prefs).Error
}

func (r *PreferencesRepository) GetOrCreate(userID string) (*models.UserPreferences, error) {
	prefs, err := r.GetByUserID(userID)
	if err == gorm.ErrRecordNotFound {
		prefs = &models.UserPreferences{
			UserID:         userID,
			ReceiptModel:   models.DefaultReceiptModel,
			AssistantModel: models.DefaultAssistantModel,
		}
		if err := r.Create(prefs); err != nil {
			return nil, err
		}
		return prefs, nil
	}
	if err != nil {
		return nil, err
	}

	changed := false
	if !models.IsSupportedGeminiModel(prefs.ReceiptModel) {
		prefs.ReceiptModel = models.DefaultReceiptModel
		changed = true
	}
	if !models.IsSupportedGeminiModel(prefs.AssistantModel) {
		prefs.AssistantModel = models.DefaultAssistantModel
		changed = true
	}
	if changed {
		if err := r.Update(prefs); err != nil {
			return nil, err
		}
	}

	return prefs, nil
}
