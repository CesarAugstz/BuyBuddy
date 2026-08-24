package handlers

import (
	"buybuddy-api/models"
	"buybuddy-api/repository"
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type KnowledgeHandler struct {
	repo      *repository.KnowledgeRepository
	organizer KnowledgeOrganizer
}

type KnowledgeOrganizer interface {
	OrganizeTopic(context.Context, string, string) (*models.KnowledgeOrganizationResponse, error)
}

func NewKnowledgeHandler(repo *repository.KnowledgeRepository, organizers ...KnowledgeOrganizer) *KnowledgeHandler {
	handler := &KnowledgeHandler{repo: repo}
	if len(organizers) > 0 {
		handler.organizer = organizers[0]
	}
	return handler
}

func (h *KnowledgeHandler) ensureKnowledge(userID string) error {
	_, err := h.repo.EnsureInbox(userID)
	return err
}

func knowledgeHTTPError(err error, fallback string) *echo.HTTPError {
	switch {
	case errors.Is(err, repository.ErrKnowledgeNotFound):
		return echo.NewHTTPError(http.StatusNotFound, "knowledge item not found")
	case errors.Is(err, repository.ErrKnowledgeConflict):
		return echo.NewHTTPError(http.StatusConflict, err.Error())
	case errors.Is(err, repository.ErrKnowledgeInvalid):
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	default:
		return echo.NewHTTPError(http.StatusInternalServerError, fallback)
	}
}

func parseKnowledgeUUID(raw, field string) (string, *echo.HTTPError) {
	value := strings.TrimSpace(raw)
	parsed, err := uuid.Parse(value)
	if err != nil {
		return "", echo.NewHTTPError(http.StatusBadRequest, field+" must be a valid UUID")
	}
	return parsed.String(), nil
}

func (h *KnowledgeHandler) ListTopics(c echo.Context) error {
	userID := c.Get("userID").(string)
	topics, err := h.repo.ListTopics(userID)
	if err != nil {
		return knowledgeHTTPError(err, "failed to fetch knowledge topics")
	}
	return c.JSON(http.StatusOK, topics)
}

func (h *KnowledgeHandler) GetTopicTree(c echo.Context) error {
	userID := c.Get("userID").(string)
	tree, err := h.repo.ListTopicTree(userID)
	if err != nil {
		return knowledgeHTTPError(err, "failed to fetch knowledge topic tree")
	}
	return c.JSON(http.StatusOK, tree)
}

func (h *KnowledgeHandler) GetTopic(c echo.Context) error {
	userID := c.Get("userID").(string)
	topicID, httpErr := parseKnowledgeUUID(c.Param("id"), "id")
	if httpErr != nil {
		return httpErr
	}
	if err := h.ensureKnowledge(userID); err != nil {
		return knowledgeHTTPError(err, "failed to initialize knowledge")
	}
	topic, err := h.repo.GetTopicDetail(userID, topicID)
	if err != nil {
		return knowledgeHTTPError(err, "failed to fetch knowledge topic")
	}
	return c.JSON(http.StatusOK, topic)
}

func (h *KnowledgeHandler) CreateTopic(c echo.Context) error {
	userID := c.Get("userID").(string)
	var request models.CreateKnowledgeTopicRequest
	if err := c.Bind(&request); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	request.Name = strings.TrimSpace(request.Name)
	request.Description = strings.TrimSpace(request.Description)
	if err := validateCreateKnowledgeTopicRequest(request); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if request.ParentID != nil {
		parentID, httpErr := parseKnowledgeUUID(*request.ParentID, "parentId")
		if httpErr != nil {
			return httpErr
		}
		request.ParentID = &parentID
	}

	topic := &models.KnowledgeTopic{
		UserID:      userID,
		ParentID:    request.ParentID,
		Name:        request.Name,
		Description: request.Description,
	}
	if err := h.repo.CreateTopic(userID, topic); err != nil {
		return knowledgeHTTPError(err, "failed to create knowledge topic")
	}
	return c.JSON(http.StatusCreated, topic)
}

func validateCreateKnowledgeTopicRequest(request models.CreateKnowledgeTopicRequest) error {
	if strings.TrimSpace(request.Name) == "" {
		return errors.New("name is required")
	}
	if utf8.RuneCountInString(strings.TrimSpace(request.Name)) > 120 {
		return errors.New("name must be 120 characters or fewer")
	}
	if request.ParentID != nil && strings.TrimSpace(*request.ParentID) == "" {
		return errors.New("parentId cannot be empty")
	}
	return nil
}

func (h *KnowledgeHandler) UpdateTopic(c echo.Context) error {
	userID := c.Get("userID").(string)
	topicID, httpErr := parseKnowledgeUUID(c.Param("id"), "id")
	if httpErr != nil {
		return httpErr
	}
	var request models.UpdateKnowledgeTopicRequest
	if err := c.Bind(&request); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if request.Name == nil && request.Description == nil && request.ParentID == nil && !request.MoveToRoot {
		return echo.NewHTTPError(http.StatusBadRequest, "at least one field is required")
	}
	if request.MoveToRoot && request.ParentID != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "parentId and moveToRoot cannot be used together")
	}
	if request.ParentID != nil && strings.TrimSpace(*request.ParentID) == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "parentId cannot be empty")
	}
	if request.ParentID != nil {
		parentID, parentHTTPErr := parseKnowledgeUUID(*request.ParentID, "parentId")
		if parentHTTPErr != nil {
			return parentHTTPErr
		}
		request.ParentID = &parentID
	}
	if err := h.ensureKnowledge(userID); err != nil {
		return knowledgeHTTPError(err, "failed to initialize knowledge")
	}
	topic, err := h.repo.UpdateTopic(userID, topicID, request)
	if err != nil {
		return knowledgeHTTPError(err, "failed to update knowledge topic")
	}
	return c.JSON(http.StatusOK, topic)
}

func (h *KnowledgeHandler) DeleteTopic(c echo.Context) error {
	userID := c.Get("userID").(string)
	topicID, httpErr := parseKnowledgeUUID(c.Param("id"), "id")
	if httpErr != nil {
		return httpErr
	}
	if err := h.ensureKnowledge(userID); err != nil {
		return knowledgeHTTPError(err, "failed to initialize knowledge")
	}
	if err := h.repo.DeleteTopic(userID, topicID); err != nil {
		return knowledgeHTTPError(err, "failed to delete knowledge topic")
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *KnowledgeHandler) ListTopicEntries(c echo.Context) error {
	userID := c.Get("userID").(string)
	topicID, httpErr := parseKnowledgeUUID(c.Param("id"), "id")
	if httpErr != nil {
		return httpErr
	}
	if err := h.ensureKnowledge(userID); err != nil {
		return knowledgeHTTPError(err, "failed to initialize knowledge")
	}
	limit, err := parseBoundedInteger(c.QueryParam("limit"), 50, 1, 100)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "limit must be between 1 and 100")
	}
	offset, err := parseBoundedInteger(c.QueryParam("offset"), 0, 0, 100000)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "offset must be a non-negative integer")
	}
	entries, err := h.repo.ListEntries(userID, topicID, limit, offset)
	if err != nil {
		return knowledgeHTTPError(err, "failed to fetch knowledge entries")
	}
	return c.JSON(http.StatusOK, entries)
}

func (h *KnowledgeHandler) GetEntry(c echo.Context) error {
	userID := c.Get("userID").(string)
	entryID, httpErr := parseKnowledgeUUID(c.Param("id"), "id")
	if httpErr != nil {
		return httpErr
	}
	if err := h.ensureKnowledge(userID); err != nil {
		return knowledgeHTTPError(err, "failed to initialize knowledge")
	}
	entry, err := h.repo.GetEntry(userID, entryID)
	if err != nil {
		return knowledgeHTTPError(err, "failed to fetch knowledge entry")
	}
	return c.JSON(http.StatusOK, entry)
}

func (h *KnowledgeHandler) CreateEntry(c echo.Context) error {
	userID := c.Get("userID").(string)
	var request models.CreateKnowledgeEntryRequest
	if err := c.Bind(&request); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	request.TopicID = strings.TrimSpace(request.TopicID)
	request.Kind = strings.TrimSpace(request.Kind)
	request.Title = strings.TrimSpace(request.Title)
	if request.TopicID == "" || request.Title == "" || strings.TrimSpace(request.Body) == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "topicId, title, and body are required")
	}
	topicID, httpErr := parseKnowledgeUUID(request.TopicID, "topicId")
	if httpErr != nil {
		return httpErr
	}
	request.TopicID = topicID
	if request.Kind == "" {
		request.Kind = "note"
	}
	entry := &models.KnowledgeEntry{
		UserID:     userID,
		TopicID:    request.TopicID,
		Kind:       request.Kind,
		Title:      request.Title,
		Body:       request.Body,
		Attributes: request.Attributes,
		Tags:       request.Tags,
		OccurredAt: request.OccurredAt,
		Source:     models.KnowledgeSourceManual,
	}
	if err := h.repo.CreateEntry(userID, entry); err != nil {
		return knowledgeHTTPError(err, "failed to create knowledge entry")
	}
	return c.JSON(http.StatusCreated, entry)
}

func (h *KnowledgeHandler) UpdateEntry(c echo.Context) error {
	userID := c.Get("userID").(string)
	entryID, httpErr := parseKnowledgeUUID(c.Param("id"), "id")
	if httpErr != nil {
		return httpErr
	}
	var request models.UpdateKnowledgeEntryRequest
	if err := c.Bind(&request); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if err := validateUpdateKnowledgeEntryRequest(request); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if request.TopicID != nil {
		topicID, topicHTTPErr := parseKnowledgeUUID(*request.TopicID, "topicId")
		if topicHTTPErr != nil {
			return topicHTTPErr
		}
		request.TopicID = &topicID
	}
	if err := h.ensureKnowledge(userID); err != nil {
		return knowledgeHTTPError(err, "failed to initialize knowledge")
	}
	mutation := models.KnowledgeEntryMutation{
		TopicID:           request.TopicID,
		Kind:              request.Kind,
		Title:             request.Title,
		Body:              request.Body,
		Attributes:        request.Attributes,
		ReplaceAttributes: request.ReplaceAttributes,
		Tags:              request.Tags,
		OccurredAt:        request.OccurredAt,
		ClearOccurredAt:   request.ClearOccurredAt,
	}
	entry, err := h.repo.UpdateEntry(userID, entryID, request.ExpectedVersion, mutation, models.KnowledgeSourceManual)
	if err != nil {
		return knowledgeHTTPError(err, "failed to update knowledge entry")
	}
	return c.JSON(http.StatusOK, entry)
}

func validateUpdateKnowledgeEntryRequest(request models.UpdateKnowledgeEntryRequest) error {
	if request.ExpectedVersion <= 0 {
		return errors.New("expectedVersion is required")
	}
	if request.ReplaceAttributes && request.Attributes == nil {
		return errors.New("attributes are required when replaceAttributes is true")
	}
	if request.TopicID == nil && request.Kind == nil && request.Title == nil && request.Body == nil &&
		request.Attributes == nil && request.Tags == nil && request.OccurredAt == nil && !request.ClearOccurredAt {
		return errors.New("at least one field is required")
	}
	if request.ClearOccurredAt && request.OccurredAt != nil {
		return errors.New("occurredAt and clearOccurredAt cannot be used together")
	}
	return nil
}

func (h *KnowledgeHandler) DeleteEntry(c echo.Context) error {
	userID := c.Get("userID").(string)
	entryID, httpErr := parseKnowledgeUUID(c.Param("id"), "id")
	if httpErr != nil {
		return httpErr
	}
	if err := h.ensureKnowledge(userID); err != nil {
		return knowledgeHTTPError(err, "failed to initialize knowledge")
	}
	rawVersion := c.QueryParam("expectedVersion")
	value, err := strconv.Atoi(rawVersion)
	if rawVersion == "" || err != nil || value <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "expectedVersion is required and must be a positive integer")
	}
	if err := h.repo.DeleteEntry(userID, entryID, value, models.KnowledgeSourceManual); err != nil {
		return knowledgeHTTPError(err, "failed to delete knowledge entry")
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *KnowledgeHandler) Search(c echo.Context) error {
	userID := c.Get("userID").(string)
	limit, err := parseBoundedInteger(c.QueryParam("limit"), 30, 1, 50)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "limit must be between 1 and 50")
	}
	filter := models.KnowledgeSearchFilter{
		Query:           strings.TrimSpace(c.QueryParam("q")),
		TopicID:         strings.TrimSpace(c.QueryParam("topicId")),
		IncludeChildren: c.QueryParam("includeChildren") == "true",
		Kind:            strings.TrimSpace(c.QueryParam("kind")),
		Tag:             strings.TrimSpace(c.QueryParam("tag")),
		Limit:           limit,
	}
	if filter.TopicID != "" {
		topicID, httpErr := parseKnowledgeUUID(filter.TopicID, "topicId")
		if httpErr != nil {
			return httpErr
		}
		filter.TopicID = topicID
	}
	if utf8.RuneCountInString(filter.Query) > 500 {
		return echo.NewHTTPError(http.StatusBadRequest, "q must be 500 characters or fewer")
	}
	if raw := c.QueryParam("occurredFrom"); raw != "" {
		value, parseErr := parseKnowledgeTime(raw, false)
		if parseErr != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "occurredFrom must be an RFC3339 timestamp or YYYY-MM-DD")
		}
		filter.OccurredFrom = &value
	}
	if raw := c.QueryParam("occurredTo"); raw != "" {
		value, parseErr := parseKnowledgeTime(raw, true)
		if parseErr != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "occurredTo must be an RFC3339 timestamp or YYYY-MM-DD")
		}
		filter.OccurredTo = &value
	}
	if err := h.ensureKnowledge(userID); err != nil {
		return knowledgeHTTPError(err, "failed to initialize knowledge")
	}
	results, err := h.repo.Search(userID, filter)
	if err != nil {
		return knowledgeHTTPError(err, "failed to search knowledge")
	}
	return c.JSON(http.StatusOK, results)
}

func (h *KnowledgeHandler) UndoEntry(c echo.Context) error {
	userID := c.Get("userID").(string)
	entryID, httpErr := parseKnowledgeUUID(c.Param("id"), "id")
	if httpErr != nil {
		return httpErr
	}
	var request models.UndoKnowledgeEntryRequest
	if err := c.Bind(&request); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if request.ExpectedVersion <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "expectedVersion is required")
	}
	if err := h.ensureKnowledge(userID); err != nil {
		return knowledgeHTTPError(err, "failed to initialize knowledge")
	}
	entry, err := h.repo.UndoEntry(userID, entryID, request.ExpectedVersion)
	if err != nil {
		return knowledgeHTTPError(err, "failed to undo knowledge change")
	}
	return c.JSON(http.StatusOK, entry)
}

func (h *KnowledgeHandler) OrganizeTopic(c echo.Context) error {
	userID := c.Get("userID").(string)
	topicID, httpErr := parseKnowledgeUUID(c.Param("id"), "id")
	if httpErr != nil {
		return httpErr
	}
	if h.organizer == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "knowledge organizer is unavailable")
	}
	response, err := h.organizer.OrganizeTopic(c.Request().Context(), userID, topicID)
	if err != nil {
		if errors.Is(err, repository.ErrKnowledgeNotFound) ||
			errors.Is(err, repository.ErrKnowledgeConflict) ||
			errors.Is(err, repository.ErrKnowledgeInvalid) {
			return knowledgeHTTPError(err, "failed to organize knowledge topic")
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return echo.NewHTTPError(http.StatusRequestTimeout, "topic organization was interrupted; please try again")
		}
		return echo.NewHTTPError(http.StatusBadGateway, "topic organization failed; please try again")
	}
	return c.JSON(http.StatusOK, response)
}

func parseBoundedInteger(raw string, defaultValue, minimum, maximum int) (int, error) {
	if raw == "" {
		return defaultValue, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < minimum || value > maximum {
		return 0, errors.New("integer is outside allowed range")
	}
	return value, nil
}

func parseKnowledgeTime(raw string, endOfDay bool) (time.Time, error) {
	if value, err := time.Parse(time.RFC3339, raw); err == nil {
		return value, nil
	}
	value, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return time.Time{}, err
	}
	if endOfDay {
		value = value.Add(24*time.Hour - time.Nanosecond)
	}
	return value, nil
}
