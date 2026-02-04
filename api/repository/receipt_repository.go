package repository

import (
	"buybuddy-api/models"
	"time"

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

func (r *ReceiptRepository) GetFirstReceiptDate(userID string) (*time.Time, error) {
	var receipt models.Receipt
	err := r.db.Where("user_id = ?", userID).
		Order("date ASC").
		First(&receipt).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return receipt.Date, nil
}

func (r *ReceiptRepository) QueryWithFilters(userID string, filter *models.AssistantQueryFilter, limit int) ([]models.Receipt, error) {
	query := r.db.Where("receipts.user_id = ?", userID).
		Preload("Items.Category").
		Preload("Items.Subcategory")

	if len(filter.Company) > 0 {
		orConditions := r.db.Where("1 = 0")
		for _, c := range filter.Company {
			orConditions = orConditions.Or("receipts.company ILIKE ?", "%"+c+"%")
		}
		query = query.Where(orConditions)
	}

	if filter.DateFrom != "" {
		query = query.Where("receipts.date >= ?", filter.DateFrom)
	}
	if filter.DateTo != "" {
		query = query.Where("receipts.date <= ?", filter.DateTo)
	}

	needsItemJoin := len(filter.ProductName) > 0 || len(filter.Brand) > 0 ||
		len(filter.Category) > 0 || len(filter.Subcategory) > 0 ||
		filter.MinPrice != nil || filter.MaxPrice != nil

	useCategoryFilters := len(filter.ProductName) <= 1

	if needsItemJoin {
		query = query.Joins("JOIN receipt_items ON receipt_items.receipt_id = receipts.id AND receipt_items.deleted_at IS NULL")

		if len(filter.ProductName) > 0 {
			orConditions := r.db.Where("1 = 0")
			for _, name := range filter.ProductName {
				orConditions = orConditions.Or("receipt_items.name ILIKE ?", "%"+name+"%")
				orConditions = orConditions.Or("receipt_items.raw_name ILIKE ?", "%"+name+"%")
			}
			query = query.Where(orConditions)
		}

		if len(filter.Brand) > 0 {
			orConditions := r.db.Where("1 = 0")
			for _, b := range filter.Brand {
				orConditions = orConditions.Or("receipt_items.brand ILIKE ?", "%"+b+"%")
			}
			query = query.Where(orConditions)
		}

		if useCategoryFilters && len(filter.Category) > 0 {
			query = query.Joins("JOIN categories ON categories.id = receipt_items.category_id")
			orConditions := r.db.Where("1 = 0")
			for _, cat := range filter.Category {
				orConditions = orConditions.Or("categories.name ILIKE ?", "%"+cat+"%")
			}
			query = query.Where(orConditions)
		}

		if useCategoryFilters && len(filter.Subcategory) > 0 {
			query = query.Joins("JOIN subcategories ON subcategories.id = receipt_items.subcategory_id")
			orConditions := r.db.Where("1 = 0")
			for _, sub := range filter.Subcategory {
				orConditions = orConditions.Or("subcategories.name ILIKE ?", "%"+sub+"%")
			}
			query = query.Where(orConditions)
		}

		if filter.MinPrice != nil {
			query = query.Where("receipt_items.total_price >= ?", *filter.MinPrice)
		}
		if filter.MaxPrice != nil {
			query = query.Where("receipt_items.total_price <= ?", *filter.MaxPrice)
		}

		query = query.Distinct("receipts.*")
	}

	orderBy := "receipts.date DESC"
	if filter.OrderBy != "" {
		switch filter.OrderBy {
		case "date_asc":
			orderBy = "receipts.date ASC"
		case "date_desc":
			orderBy = "receipts.date DESC"
		case "total_asc":
			orderBy = "receipts.total ASC"
		case "total_desc":
			orderBy = "receipts.total DESC"
		}
	}

	queryLimit := limit
	if filter.Limit != nil && *filter.Limit > 0 && *filter.Limit < limit {
		queryLimit = *filter.Limit
	}

	var receipts []models.Receipt
	err := query.Order(orderBy).
		Limit(queryLimit).
		Find(&receipts).Error

	return receipts, err
}
