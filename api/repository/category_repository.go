package repository

import (
	"easybuy-api/models"

	"gorm.io/gorm"
)

type CategoryRepository struct {
	db *gorm.DB
}

func NewCategoryRepository(db *gorm.DB) *CategoryRepository {
	return &CategoryRepository{db: db}
}

func (r *CategoryRepository) GetAll() ([]models.Category, error) {
	var categories []models.Category
	err := r.db.Preload("Subcategories").Find(&categories).Error
	return categories, err
}

func (r *CategoryRepository) GetByID(id uint) (*models.Category, error) {
	var category models.Category
	err := r.db.Preload("Subcategories").First(&category, id).Error
	if err != nil {
		return nil, err
	}
	return &category, nil
}

func (r *CategoryRepository) GetByName(name string) (*models.Category, error) {
	var category models.Category
	err := r.db.Where("name = ?", name).First(&category).Error
	if err != nil {
		return nil, err
	}
	return &category, nil
}

func (r *CategoryRepository) Create(category *models.Category) error {
	return r.db.Create(category).Error
}

func (r *CategoryRepository) GetSubcategoryByName(categoryID uint, name string) (*models.Subcategory, error) {
	var subcategory models.Subcategory
	err := r.db.Where("category_id = ? AND name = ?", categoryID, name).First(&subcategory).Error
	if err != nil {
		return nil, err
	}
	return &subcategory, nil
}

func (r *CategoryRepository) CreateSubcategory(subcategory *models.Subcategory) error {
	return r.db.Create(subcategory).Error
}

func (r *CategoryRepository) SeedDefaultCategories() error {
	categories := []models.Category{
		{
			Name:        "Food",
			Description: "Food and beverages",
			Icon:        "🍽️",
			Subcategories: []models.Subcategory{
				{Name: "Meat", Description: "Beef, Chicken, Pork, Fish"},
				{Name: "Dairy", Description: "Milk, Cheese, Yogurt"},
				{Name: "Fruits & Vegetables", Description: "Fresh Produce"},
				{Name: "Bakery", Description: "Bread, Cakes, Pastries"},
				{Name: "Beverages", Description: "Soft Drinks, Juices, Alcohol"},
				{Name: "Snacks & Sweets", Description: "Chips, Candy, Cookies"},
			},
		},
		{
			Name:        "Household",
			Description: "Household items and supplies",
			Icon:        "🏠",
			Subcategories: []models.Subcategory{
				{Name: "Cleaning Products", Description: "Detergents, Disinfectants"},
				{Name: "Paper Products", Description: "Toilet Paper, Tissues"},
				{Name: "Kitchen Supplies", Description: "Utensils, Containers"},
			},
		},
		{
			Name:        "Tools & Hardware",
			Description: "Tools and hardware items",
			Icon:        "🔧",
			Subcategories: []models.Subcategory{
				{Name: "Power Tools", Description: "Drills, Saws"},
				{Name: "Hand Tools", Description: "Hammers, Screwdrivers"},
				{Name: "Building Materials", Description: "Lumber, Nails"},
			},
		},
		{
			Name:        "Personal Care",
			Description: "Personal care and hygiene",
			Icon:        "🧴",
			Subcategories: []models.Subcategory{
				{Name: "Hygiene", Description: "Soap, Shampoo, Toothpaste"},
				{Name: "Cosmetics", Description: "Makeup, Skincare"},
			},
		},
		{
			Name:        "Other",
			Description: "Other items",
			Icon:        "📦",
			Subcategories: []models.Subcategory{
				{Name: "Electronics", Description: "Gadgets, Accessories"},
				{Name: "Office Supplies", Description: "Pens, Paper, Folders"},
				{Name: "Miscellaneous", Description: "Uncategorized items"},
			},
		},
	}

	for _, category := range categories {
		var existing models.Category
		err := r.db.Where("name = ?", category.Name).First(&existing).Error
		if err == gorm.ErrRecordNotFound {
			if err := r.db.Create(&category).Error; err != nil {
				return err
			}
		}
	}

	return nil
}
