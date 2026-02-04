package repository

import (
	"buybuddy-api/models"

	"gorm.io/gorm"
)

type ShoppingListRepository struct {
	db *gorm.DB
}

func NewShoppingListRepository(db *gorm.DB) *ShoppingListRepository {
	return &ShoppingListRepository{db: db}
}

func (r *ShoppingListRepository) Create(list *models.ShoppingList) error {
	return r.db.Create(list).Error
}

func (r *ShoppingListRepository) GetByUserID(userID string) ([]models.ShoppingList, error) {
	var lists []models.ShoppingList

	err := r.db.
		Joins("LEFT JOIN shopping_list_shares ON shopping_list_shares.list_id = shopping_lists.id AND shopping_list_shares.user_id = ? AND shopping_list_shares.status = ?", userID, models.ShareStatusAccepted).
		Where("shopping_lists.owner_id = ? OR shopping_list_shares.user_id = ?", userID, userID).
		Preload("Items").
		Preload("Shares", "status = ?", models.ShareStatusAccepted).
		Preload("Shares.User").
		Preload("Owner").
		Order("shopping_lists.updated_at DESC").
		Find(&lists).Error

	return lists, err
}

func (r *ShoppingListRepository) GetByID(id string) (*models.ShoppingList, error) {
	var list models.ShoppingList
	err := r.db.Where("id = ?", id).
		Preload("Items", func(db *gorm.DB) *gorm.DB {
			return db.Order("sort_order ASC, created_at ASC")
		}).
		Preload("Shares").
		Preload("Shares.User").
		Preload("Owner").
		First(&list).Error
	if err != nil {
		return nil, err
	}
	return &list, nil
}

func (r *ShoppingListRepository) UserHasAccess(listID, userID string) (bool, error) {
	var list models.ShoppingList
	err := r.db.Where("id = ?", listID).First(&list).Error
	if err != nil {
		return false, err
	}

	if list.OwnerID == userID {
		return true, nil
	}

	var count int64
	err = r.db.Model(&models.ShoppingListShare{}).
		Where("list_id = ? AND user_id = ? AND status = ?", listID, userID, models.ShareStatusAccepted).
		Count(&count).Error
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

func (r *ShoppingListRepository) IsOwner(listID, userID string) (bool, error) {
	var count int64
	err := r.db.Model(&models.ShoppingList{}).
		Where("id = ? AND owner_id = ?", listID, userID).
		Count(&count).Error
	return count > 0, err
}

func (r *ShoppingListRepository) Update(list *models.ShoppingList) error {
	return r.db.Save(list).Error
}

func (r *ShoppingListRepository) Delete(id string, userID string) error {
	return r.db.Where("id = ? AND owner_id = ?", id, userID).
		Delete(&models.ShoppingList{}).Error
}

func (r *ShoppingListRepository) AddItem(item *models.ShoppingListItem) error {
	var maxOrder int
	r.db.Model(&models.ShoppingListItem{}).
		Where("list_id = ?", item.ListID).
		Select("COALESCE(MAX(sort_order), 0)").
		Scan(&maxOrder)

	item.SortOrder = maxOrder + 1
	return r.db.Create(item).Error
}

func (r *ShoppingListRepository) GetItemByID(itemID string) (*models.ShoppingListItem, error) {
	var item models.ShoppingListItem
	err := r.db.Where("id = ?", itemID).First(&item).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *ShoppingListRepository) UpdateItem(item *models.ShoppingListItem) error {
	return r.db.Save(item).Error
}

func (r *ShoppingListRepository) DeleteItem(itemID string) error {
	return r.db.Where("id = ?", itemID).Delete(&models.ShoppingListItem{}).Error
}

func (r *ShoppingListRepository) ReorderItems(items []struct {
	ID        string `json:"id"`
	SortOrder int    `json:"sortOrder"`
}) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		for _, item := range items {
			if err := tx.Model(&models.ShoppingListItem{}).
				Where("id = ?", item.ID).
				Update("sort_order", item.SortOrder).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *ShoppingListRepository) CreateShare(share *models.ShoppingListShare) error {
	return r.db.Create(share).Error
}

func (r *ShoppingListRepository) GetShareByID(shareID string) (*models.ShoppingListShare, error) {
	var share models.ShoppingListShare
	err := r.db.Where("id = ?", shareID).
		Preload("List").
		Preload("User").
		Preload("Inviter").
		First(&share).Error
	if err != nil {
		return nil, err
	}
	return &share, nil
}

func (r *ShoppingListRepository) GetPendingInvitesForUser(userID string) ([]models.ShoppingListShare, error) {
	var shares []models.ShoppingListShare
	err := r.db.Where("user_id = ? AND status = ?", userID, models.ShareStatusPending).
		Preload("List").
		Preload("Inviter").
		Order("created_at DESC").
		Find(&shares).Error
	return shares, err
}

func (r *ShoppingListRepository) UpdateShare(share *models.ShoppingListShare) error {
	return r.db.Save(share).Error
}

func (r *ShoppingListRepository) DeleteShare(listID, userID string) error {
	return r.db.Where("list_id = ? AND user_id = ?", listID, userID).
		Delete(&models.ShoppingListShare{}).Error
}

func (r *ShoppingListRepository) ShareExists(listID, userID string) (bool, error) {
	var count int64
	err := r.db.Model(&models.ShoppingListShare{}).
		Where("list_id = ? AND user_id = ?", listID, userID).
		Count(&count).Error
	return count > 0, err
}

func (r *ShoppingListRepository) GetItemSuggestions(userID string, query string, limit int) ([]string, error) {
	var names []string

	err := r.db.Model(&models.ReceiptItem{}).
		Joins("JOIN receipts ON receipts.id = receipt_items.receipt_id").
		Where("receipts.user_id = ? AND receipt_items.name ILIKE ?", userID, "%"+query+"%").
		Select("DISTINCT receipt_items.name").
		Limit(limit).
		Pluck("name", &names).Error

	return names, err
}
