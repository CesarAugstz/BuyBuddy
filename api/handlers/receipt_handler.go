package handlers

import (
	"easybuy-api/config"
	"easybuy-api/models"
	"easybuy-api/repository"
	"easybuy-api/utils"
	"encoding/base64"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

type ReceiptHandler struct {
	cfg          *config.Config
	receiptRepo  *repository.ReceiptRepository
	categoryRepo *repository.CategoryRepository
}

func NewReceiptHandler(cfg *config.Config, receiptRepo *repository.ReceiptRepository, categoryRepo *repository.CategoryRepository) *ReceiptHandler {
	return &ReceiptHandler{
		cfg:          cfg,
		receiptRepo:  receiptRepo,
		categoryRepo: categoryRepo,
	}
}

func (h *ReceiptHandler) ProcessReceipt(c echo.Context) error {
	userID := c.Get("userID").(string)

	var req models.ProcessReceiptRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	imageData, err := base64.StdEncoding.DecodeString(req.Image)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid image data")
	}

	geminiKey := h.cfg.GeminiAPIKey
	if geminiKey == "" {
		return echo.NewHTTPError(http.StatusInternalServerError, "Gemini API key not configured")
	}

	categories, err := h.categoryRepo.GetAll()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch categories")
	}

	categoryInfos := make([]utils.CategoryInfo, len(categories))
	for i, cat := range categories {
		subcats := make([]string, len(cat.Subcategories))
		for j, subcat := range cat.Subcategories {
			subcats[j] = subcat.Name
		}
		categoryInfos[i] = utils.CategoryInfo{
			Name:          cat.Name,
			Subcategories: subcats,
		}
	}

	// Get user's previous receipt items for learning
	receipts, err := h.receiptRepo.GetByUserID(userID)
	if err != nil {
		receipts = []models.Receipt{} // Continue even if we can't get history
	}

	itemMappings := make([]utils.ItemMapping, 0)
	for _, receipt := range receipts {
		for _, item := range receipt.Items {
			itemMappings = append(itemMappings, utils.ItemMapping{
				RawName: item.RawName,
				Name:    item.Name,
			})
		}
	}

	receiptData, err := utils.ProcessReceiptWithGemini(c.Request().Context(), imageData, geminiKey, categoryInfos, itemMappings)
	if err != nil {
		fmt.Println("Gemini processing error:", err)
		return echo.NewHTTPError(http.StatusBadRequest, map[string]string{
			"message": "Could not extract information from the receipt. Please make sure the image is clear and contains a valid receipt.",
			"error":   err.Error(),
		})
	}

	fmt.Printf("User %s processed receipt: %+v\n", userID, receiptData)

	return c.JSON(http.StatusOK, receiptData)
}

func (h *ReceiptHandler) SaveReceipt(c echo.Context) error {
	userID := c.Get("userID").(string)

	var req models.SaveReceiptRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if req.AccessKey != "" {
		exists, err := h.receiptRepo.ExistsByAccessKey(req.AccessKey, userID)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to check for duplicate receipt")
		}
		if exists {
			existing, err := h.receiptRepo.GetByAccessKey(req.AccessKey, userID)
			if err == nil {
				return echo.NewHTTPError(http.StatusConflict, map[string]interface{}{
					"message": "This receipt has already been saved",
					"receipt": existing,
				})
			}
			return echo.NewHTTPError(http.StatusConflict, "This receipt has already been saved")
		}
	}

	receipt := &models.Receipt{
		UserID:    userID,
		Company:   req.Company,
		Total:     req.Total,
		AccessKey: req.AccessKey,
		Items:     []models.ReceiptItem{},
	}

	if req.Date != "" {
		if parsedDate, err := time.Parse(time.RFC3339, req.Date); err == nil {
			receipt.Date = &parsedDate
		}
	}

	for _, item := range req.Items {
		rawName := getStringFromMap(item, "rawName")

		var name string
		if customName := getStringFromMap(item, "customName"); customName != "" {
			name = customName
		} else if itemName := getStringFromMap(item, "name"); itemName != "" {
			name = itemName
		} else if nameOptions, ok := item["nameOptions"].([]interface{}); ok && len(nameOptions) > 0 {
			if firstOption, ok := nameOptions[0].(string); ok {
				name = firstOption
			}
		}

		if rawName == "" {
			rawName = name
		}
		if name == "" {
			name = rawName
		}

		receiptItem := models.ReceiptItem{
			RawName:    rawName,
			Name:       name,
			Brand:      getStringFromMap(item, "brand"),
			Quantity:   getFloatFromMap(item, "quantity", 1.0),
			Unit:       getStringFromMap(item, "unit"),
			UnitPrice:  getFloatFromMap(item, "unitPrice", 0.0),
			TotalPrice: getFloatFromMap(item, "totalPrice", 0.0),
			Barcode:    getStringFromMap(item, "barcode"),
		}

		if receiptItem.Unit == "" {
			receiptItem.Unit = "un"
		}

		// Try to get category from categoryOptions array (first option)
		categoryName := ""
		subcategoryName := ""

		if categoryOptions, ok := item["categoryOptions"].([]interface{}); ok && len(categoryOptions) > 0 {
			if firstCatOption, ok := categoryOptions[0].(map[string]interface{}); ok {
				categoryName = getStringFromMap(firstCatOption, "category")
				subcategoryName = getStringFromMap(firstCatOption, "subcategory")
			}
		}

		// Fallback to old format if categoryOptions not found
		if categoryName == "" {
			categoryName = getStringFromMap(item, "category")
			subcategoryName = getStringFromMap(item, "subcategory")
		}

		if categoryName != "" {
			category, err := h.categoryRepo.GetByName(categoryName)
			if err == nil {
				receiptItem.CategoryID = &category.ID

				if subcategoryName != "" {
					subcategory, err := h.categoryRepo.GetSubcategoryByName(category.ID, subcategoryName)
					if err == nil {
						receiptItem.SubcategoryID = &subcategory.ID
					}
				}
			}
		}

		receipt.Items = append(receipt.Items, receiptItem)
	}

	if err := h.receiptRepo.Create(receipt); err != nil {
		fmt.Println("Error saving receipt:", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to save receipt")
	}

	return c.JSON(http.StatusCreated, receipt)
}

func (h *ReceiptHandler) GetReceipts(c echo.Context) error {
	userID := c.Get("userID").(string)

	receipts, err := h.receiptRepo.GetByUserID(userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch receipts")
	}

	return c.JSON(http.StatusOK, receipts)
}

func (h *ReceiptHandler) GetReceipt(c echo.Context) error {
	userID := c.Get("userID").(string)
	receiptID := c.Param("id")

	receipt, err := h.receiptRepo.GetByID(receiptID, userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "receipt not found")
	}

	return c.JSON(http.StatusOK, receipt)
}

func (h *ReceiptHandler) DeleteReceipt(c echo.Context) error {
	userID := c.Get("userID").(string)
	receiptID := c.Param("id")

	if err := h.receiptRepo.Delete(receiptID, userID); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to delete receipt")
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "receipt deleted"})
}

func getStringFromMap(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func getFloatFromMap(m map[string]interface{}, key string, defaultVal float64) float64 {
	if val, ok := m[key]; ok {
		switch v := val.(type) {
		case float64:
			return v
		case int:
			return float64(v)
		case string:
			var f float64
			fmt.Sscanf(v, "%f", &f)
			return f
		}
	}
	return defaultVal
}
