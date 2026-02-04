package handlers

import (
	"buybuddy-api/models"
	"buybuddy-api/repository"
	"net/http"

	"github.com/labstack/echo/v4"
)

type ShoppingListHandler struct {
	listRepo *repository.ShoppingListRepository
	userRepo *repository.UserRepository
}

func NewShoppingListHandler(listRepo *repository.ShoppingListRepository, userRepo *repository.UserRepository) *ShoppingListHandler {
	return &ShoppingListHandler{
		listRepo: listRepo,
		userRepo: userRepo,
	}
}

func (h *ShoppingListHandler) GetLists(c echo.Context) error {
	userID := c.Get("userID").(string)

	lists, err := h.listRepo.GetByUserID(userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch shopping lists")
	}

	response := make([]models.ShoppingListResponse, len(lists))
	for i, list := range lists {
		checkedCount := 0
		for _, item := range list.Items {
			if item.IsChecked {
				checkedCount++
			}
		}

		response[i] = models.ShoppingListResponse{
			ShoppingList:    list,
			ItemCount:       len(list.Items),
			CheckedCount:    checkedCount,
			IsShared:        len(list.Shares) > 0,
			IsOwner:         list.OwnerID == userID,
			SharedWithCount: len(list.Shares),
		}
	}

	return c.JSON(http.StatusOK, response)
}

func (h *ShoppingListHandler) CreateList(c echo.Context) error {
	userID := c.Get("userID").(string)

	var req models.CreateShoppingListRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if req.Title == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "title is required")
	}

	list := &models.ShoppingList{
		Title:       req.Title,
		Description: req.Description,
		OwnerID:     userID,
	}

	if err := h.listRepo.Create(list); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create shopping list")
	}

	return c.JSON(http.StatusCreated, list)
}

func (h *ShoppingListHandler) GetList(c echo.Context) error {
	userID := c.Get("userID").(string)
	listID := c.Param("id")

	hasAccess, err := h.listRepo.UserHasAccess(listID, userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "shopping list not found")
	}
	if !hasAccess {
		return echo.NewHTTPError(http.StatusForbidden, "access denied")
	}

	list, err := h.listRepo.GetByID(listID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "shopping list not found")
	}

	checkedCount := 0
	for _, item := range list.Items {
		if item.IsChecked {
			checkedCount++
		}
	}

	response := models.ShoppingListResponse{
		ShoppingList:    *list,
		ItemCount:       len(list.Items),
		CheckedCount:    checkedCount,
		IsShared:        len(list.Shares) > 0,
		IsOwner:         list.OwnerID == userID,
		SharedWithCount: len(list.Shares),
	}

	return c.JSON(http.StatusOK, response)
}

func (h *ShoppingListHandler) UpdateList(c echo.Context) error {
	userID := c.Get("userID").(string)
	listID := c.Param("id")

	hasAccess, err := h.listRepo.UserHasAccess(listID, userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "shopping list not found")
	}
	if !hasAccess {
		return echo.NewHTTPError(http.StatusForbidden, "access denied")
	}

	var req models.UpdateShoppingListRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	list, err := h.listRepo.GetByID(listID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "shopping list not found")
	}

	if req.Title != "" {
		list.Title = req.Title
	}
	if req.Description != "" {
		list.Description = req.Description
	}

	if err := h.listRepo.Update(list); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to update shopping list")
	}

	return c.JSON(http.StatusOK, list)
}

func (h *ShoppingListHandler) DeleteList(c echo.Context) error {
	userID := c.Get("userID").(string)
	listID := c.Param("id")

	isOwner, err := h.listRepo.IsOwner(listID, userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "shopping list not found")
	}
	if !isOwner {
		return echo.NewHTTPError(http.StatusForbidden, "only the owner can delete this list")
	}

	if err := h.listRepo.Delete(listID, userID); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to delete shopping list")
	}

	return c.NoContent(http.StatusNoContent)
}

func (h *ShoppingListHandler) AddItem(c echo.Context) error {
	userID := c.Get("userID").(string)
	listID := c.Param("id")

	hasAccess, err := h.listRepo.UserHasAccess(listID, userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "shopping list not found")
	}
	if !hasAccess {
		return echo.NewHTTPError(http.StatusForbidden, "access denied")
	}

	var req models.CreateShoppingListItemRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if req.Name == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "item name is required")
	}

	item := &models.ShoppingListItem{
		ListID:   listID,
		Name:     req.Name,
		Quantity: req.Quantity,
		Unit:     req.Unit,
	}

	if item.Quantity == 0 {
		item.Quantity = 1
	}
	if item.Unit == "" {
		item.Unit = "un"
	}

	if err := h.listRepo.AddItem(item); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to add item")
	}

	return c.JSON(http.StatusCreated, item)
}

func (h *ShoppingListHandler) UpdateItem(c echo.Context) error {
	userID := c.Get("userID").(string)
	listID := c.Param("id")
	itemID := c.Param("itemId")

	hasAccess, err := h.listRepo.UserHasAccess(listID, userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "shopping list not found")
	}
	if !hasAccess {
		return echo.NewHTTPError(http.StatusForbidden, "access denied")
	}

	var req models.UpdateShoppingListItemRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	item, err := h.listRepo.GetItemByID(itemID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "item not found")
	}

	if item.ListID != listID {
		return echo.NewHTTPError(http.StatusBadRequest, "item does not belong to this list")
	}

	if req.Name != "" {
		item.Name = req.Name
	}
	if req.Quantity != 0 {
		item.Quantity = req.Quantity
	}
	if req.Unit != "" {
		item.Unit = req.Unit
	}
	if req.IsChecked != nil {
		item.IsChecked = *req.IsChecked
	}
	if req.SortOrder != nil {
		item.SortOrder = *req.SortOrder
	}

	if err := h.listRepo.UpdateItem(item); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to update item")
	}

	return c.JSON(http.StatusOK, item)
}

func (h *ShoppingListHandler) DeleteItem(c echo.Context) error {
	userID := c.Get("userID").(string)
	listID := c.Param("id")
	itemID := c.Param("itemId")

	hasAccess, err := h.listRepo.UserHasAccess(listID, userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "shopping list not found")
	}
	if !hasAccess {
		return echo.NewHTTPError(http.StatusForbidden, "access denied")
	}

	item, err := h.listRepo.GetItemByID(itemID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "item not found")
	}

	if item.ListID != listID {
		return echo.NewHTTPError(http.StatusBadRequest, "item does not belong to this list")
	}

	if err := h.listRepo.DeleteItem(itemID); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to delete item")
	}

	return c.NoContent(http.StatusNoContent)
}

func (h *ShoppingListHandler) ReorderItems(c echo.Context) error {
	userID := c.Get("userID").(string)
	listID := c.Param("id")

	hasAccess, err := h.listRepo.UserHasAccess(listID, userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "shopping list not found")
	}
	if !hasAccess {
		return echo.NewHTTPError(http.StatusForbidden, "access denied")
	}

	var req models.ReorderItemsRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if err := h.listRepo.ReorderItems(req.Items); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to reorder items")
	}

	return c.NoContent(http.StatusNoContent)
}

func (h *ShoppingListHandler) GetSuggestions(c echo.Context) error {
	userID := c.Get("userID").(string)
	query := c.QueryParam("q")

	suggestions, err := h.listRepo.GetItemSuggestions(userID, query, 10)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get suggestions")
	}

	return c.JSON(http.StatusOK, suggestions)
}

func (h *ShoppingListHandler) SearchUsers(c echo.Context) error {
	userID := c.Get("userID").(string)
	email := c.QueryParam("email")

	if email == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "email query parameter is required")
	}

	users, err := h.userRepo.SearchByEmail(email, userID, 10)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to search users")
	}

	return c.JSON(http.StatusOK, users)
}

func (h *ShoppingListHandler) ShareList(c echo.Context) error {
	userID := c.Get("userID").(string)
	listID := c.Param("id")

	isOwner, err := h.listRepo.IsOwner(listID, userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "shopping list not found")
	}
	if !isOwner {
		return echo.NewHTTPError(http.StatusForbidden, "only the owner can share this list")
	}

	var req models.ShareListRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	targetUser, err := h.userRepo.GetByEmail(req.Email)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "user not found")
	}

	if targetUser.ID == userID {
		return echo.NewHTTPError(http.StatusBadRequest, "cannot share with yourself")
	}

	exists, err := h.listRepo.ShareExists(listID, targetUser.ID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to check existing share")
	}
	if exists {
		return echo.NewHTTPError(http.StatusConflict, "user already has access or pending invite")
	}

	share := &models.ShoppingListShare{
		ListID:    listID,
		UserID:    targetUser.ID,
		InvitedBy: userID,
		Status:    models.ShareStatusPending,
	}

	if err := h.listRepo.CreateShare(share); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to share list")
	}

	return c.JSON(http.StatusCreated, share)
}

func (h *ShoppingListHandler) GetInvites(c echo.Context) error {
	userID := c.Get("userID").(string)

	invites, err := h.listRepo.GetPendingInvitesForUser(userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get invites")
	}

	return c.JSON(http.StatusOK, invites)
}

func (h *ShoppingListHandler) AcceptInvite(c echo.Context) error {
	userID := c.Get("userID").(string)
	inviteID := c.Param("inviteId")

	share, err := h.listRepo.GetShareByID(inviteID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "invite not found")
	}

	if share.UserID != userID {
		return echo.NewHTTPError(http.StatusForbidden, "not your invite")
	}

	if share.Status != models.ShareStatusPending {
		return echo.NewHTTPError(http.StatusBadRequest, "invite already processed")
	}

	share.Status = models.ShareStatusAccepted
	if err := h.listRepo.UpdateShare(share); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to accept invite")
	}

	return c.JSON(http.StatusOK, share)
}

func (h *ShoppingListHandler) RejectInvite(c echo.Context) error {
	userID := c.Get("userID").(string)
	inviteID := c.Param("inviteId")

	share, err := h.listRepo.GetShareByID(inviteID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "invite not found")
	}

	if share.UserID != userID {
		return echo.NewHTTPError(http.StatusForbidden, "not your invite")
	}

	if share.Status != models.ShareStatusPending {
		return echo.NewHTTPError(http.StatusBadRequest, "invite already processed")
	}

	share.Status = models.ShareStatusRejected
	if err := h.listRepo.UpdateShare(share); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to reject invite")
	}

	return c.JSON(http.StatusOK, share)
}

func (h *ShoppingListHandler) RemoveShare(c echo.Context) error {
	userID := c.Get("userID").(string)
	listID := c.Param("id")
	targetUserID := c.Param("userId")

	isOwner, err := h.listRepo.IsOwner(listID, userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "shopping list not found")
	}
	if !isOwner {
		return echo.NewHTTPError(http.StatusForbidden, "only the owner can remove shares")
	}

	if err := h.listRepo.DeleteShare(listID, targetUserID); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to remove share")
	}

	return c.NoContent(http.StatusNoContent)
}

func (h *ShoppingListHandler) GetListShares(c echo.Context) error {
	userID := c.Get("userID").(string)
	listID := c.Param("id")

	hasAccess, err := h.listRepo.UserHasAccess(listID, userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "shopping list not found")
	}
	if !hasAccess {
		return echo.NewHTTPError(http.StatusForbidden, "access denied")
	}

	list, err := h.listRepo.GetByID(listID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "shopping list not found")
	}

	return c.JSON(http.StatusOK, list.Shares)
}
