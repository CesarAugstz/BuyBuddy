package repository

import (
	"buybuddy-api/models"

	"gorm.io/gorm"
)

type ReceiptRepository struct {
	db *gorm.DB
}

func NewReceiptRepository(db *gorm.DB) *ReceiptRepository {
	return &ReceiptRepository{db: db}
}

func (r *ReceiptRepository) Create(receipt *models.Receipt) error {
	return r.db.Create(receipt).Error
}

func (r *ReceiptRepository) GetByUserID(userID string) ([]models.Receipt, error) {
	var receipts []models.Receipt
	err := r.db.Where("user_id = ?", userID).
		Preload("Items.Category").
		Preload("Items.Subcategory").
		Order("created_at DESC").
		Find(&receipts).Error
	return receipts, err
}

func (r *ReceiptRepository) GetByID(id string, userID string) (*models.Receipt, error) {
	var receipt models.Receipt
	err := r.db.Where("id = ? AND user_id = ?", id, userID).
		Preload("Items.Category").
		Preload("Items.Subcategory").
		First(&receipt).Error
	if err != nil {
		return nil, err
	}
	return &receipt, nil
}

func (r *ReceiptRepository) Delete(id string, userID string) error {
	return r.db.Where("id = ? AND user_id = ?", id, userID).
		Delete(&models.Receipt{}).Error
}

func (r *ReceiptRepository) ExistsByAccessKey(accessKey string, userID string) (bool, error) {
	var count int64
	err := r.db.Model(&models.Receipt{}).
		Where("access_key = ? AND user_id = ?", accessKey, userID).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *ReceiptRepository) GetByAccessKey(accessKey string, userID string) (*models.Receipt, error) {
	var receipt models.Receipt
	err := r.db.Where("access_key = ? AND user_id = ?", accessKey, userID).
		Preload("Items.Category").
		Preload("Items.Subcategory").
		First(&receipt).Error
	if err != nil {
		return nil, err
	}
	return &receipt, nil
}
