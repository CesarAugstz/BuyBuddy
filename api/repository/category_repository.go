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
			Name:        "Alimentos",
			Description: "Alimentos e bebidas",
			Icon:        "🍽️",
			Subcategories: []models.Subcategory{
				{Name: "Carnes", Description: "Bovina, Frango, Porco, Peixe"},
				{Name: "Laticínios", Description: "Leite, Queijo, Iogurte"},
				{Name: "Frutas e Vegetais", Description: "Produtos Frescos"},
				{Name: "Padaria", Description: "Pães, Bolos, Doces"},
				{Name: "Bebidas", Description: "Refrigerantes, Sucos, Bebidas Alcoólicas"},
				{Name: "Lanches e Doces", Description: "Salgadinhos, Balas, Biscoitos"},
			},
		},
		{
			Name:        "Casa e Limpeza",
			Description: "Itens domésticos e suprimentos",
			Icon:        "🏠",
			Subcategories: []models.Subcategory{
				{Name: "Produtos de Limpeza", Description: "Detergentes, Desinfetantes"},
				{Name: "Papel e Descartáveis", Description: "Papel Higiênico, Lenços"},
				{Name: "Utensílios de Cozinha", Description: "Utensílios, Recipientes"},
			},
		},
		{
			Name:        "Ferramentas e Construção",
			Description: "Ferramentas e materiais de construção",
			Icon:        "🔧",
			Subcategories: []models.Subcategory{
				{Name: "Ferramentas Elétricas", Description: "Furadeiras, Serras"},
				{Name: "Ferramentas Manuais", Description: "Martelos, Chaves de Fenda"},
				{Name: "Materiais de Construção", Description: "Madeira, Pregos"},
			},
		},
		{
			Name:        "Cuidados Pessoais",
			Description: "Cuidados pessoais e higiene",
			Icon:        "🧴",
			Subcategories: []models.Subcategory{
				{Name: "Higiene", Description: "Sabonete, Shampoo, Pasta de Dente"},
				{Name: "Cosméticos", Description: "Maquiagem, Cuidados com a Pele"},
			},
		},
		{
			Name:        "Outros",
			Description: "Outros itens",
			Icon:        "📦",
			Subcategories: []models.Subcategory{
				{Name: "Eletrônicos", Description: "Gadgets, Acessórios"},
				{Name: "Material de Escritório", Description: "Canetas, Papel, Pastas"},
				{Name: "Diversos", Description: "Itens não categorizados"},
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
