package handlers

import (
	"buybuddy-api/models"
	"buybuddy-api/repository"
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

func TestValidateCreateKnowledgeTopicRequest(t *testing.T) {
	if err := validateCreateKnowledgeTopicRequest(models.CreateKnowledgeTopicRequest{Name: "Projects"}); err != nil {
		t.Fatalf("valid request error = %v", err)
	}
	if err := validateCreateKnowledgeTopicRequest(models.CreateKnowledgeTopicRequest{}); err == nil {
		t.Fatal("empty name unexpectedly accepted")
	}
	emptyParent := " "
	if err := validateCreateKnowledgeTopicRequest(models.CreateKnowledgeTopicRequest{Name: "Projects", ParentID: &emptyParent}); err == nil {
		t.Fatal("empty parentId unexpectedly accepted")
	}
}

func TestParseBoundedInteger(t *testing.T) {
	if got, err := parseBoundedInteger("", 30, 1, 50); err != nil || got != 30 {
		t.Fatalf("default = %d, %v; want 30", got, err)
	}
	if _, err := parseBoundedInteger("51", 30, 1, 50); err == nil {
		t.Fatal("out-of-range integer unexpectedly accepted")
	}
}

func TestParseKnowledgeTimeDateEnd(t *testing.T) {
	got, err := parseKnowledgeTime("2026-08-24", true)
	if err != nil {
		t.Fatalf("parseKnowledgeTime() error = %v", err)
	}
	want := time.Date(2026, 8, 24, 23, 59, 59, 999999999, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("parseKnowledgeTime() = %s, want %s", got, want)
	}
}

func TestValidateUpdateKnowledgeEntryRequestReplaceAttributes(t *testing.T) {
	empty := models.KnowledgeAttributes{}
	valid := models.UpdateKnowledgeEntryRequest{
		ExpectedVersion:   2,
		Attributes:        &empty,
		ReplaceAttributes: true,
	}
	if err := validateUpdateKnowledgeEntryRequest(valid); err != nil {
		t.Fatalf("empty replacement object rejected: %v", err)
	}

	invalid := models.UpdateKnowledgeEntryRequest{
		ExpectedVersion:   2,
		ReplaceAttributes: true,
	}
	if err := validateUpdateKnowledgeEntryRequest(invalid); err == nil {
		t.Fatal("replaceAttributes without attributes unexpectedly accepted")
	}
}

func TestKnowledgeEntryUpdateBindingRetainsEmptyReplacementObject(t *testing.T) {
	e := echo.New()
	request := httptest.NewRequest(
		http.MethodPut,
		"/knowledge/entries/"+uuid.NewString(),
		bytes.NewBufferString(`{"expectedVersion":2,"attributes":{},"replaceAttributes":true}`),
	)
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

	var update models.UpdateKnowledgeEntryRequest
	if err := e.NewContext(request, httptest.NewRecorder()).Bind(&update); err != nil {
		t.Fatalf("Bind() error = %v", err)
	}
	if !update.ReplaceAttributes || update.Attributes == nil || len(*update.Attributes) != 0 {
		t.Fatalf("bound update = %#v, want explicit empty attribute replacement", update)
	}
}

func TestKnowledgeHTTPErrorMapping(t *testing.T) {
	tests := []struct {
		err  error
		code int
	}{
		{err: repository.ErrKnowledgeNotFound, code: http.StatusNotFound},
		{err: repository.ErrKnowledgeConflict, code: http.StatusConflict},
		{err: repository.ErrKnowledgeInvalid, code: http.StatusBadRequest},
		{err: errors.New("database unavailable"), code: http.StatusInternalServerError},
	}
	for _, test := range tests {
		if got := knowledgeHTTPError(test.err, "fallback"); got.Code != test.code {
			t.Errorf("knowledgeHTTPError(%v) status = %d, want %d", test.err, got.Code, test.code)
		}
	}
}

func TestKnowledgeHandlersRejectMalformedPathUUIDsBeforeRepositoryUse(t *testing.T) {
	handler := NewKnowledgeHandler(nil)
	tests := []struct {
		name   string
		method string
		target string
		body   string
		call   func(echo.Context) error
	}{
		{name: "get topic", method: http.MethodGet, target: "/knowledge/topics/not-a-uuid", call: handler.GetTopic},
		{name: "update topic", method: http.MethodPut, target: "/knowledge/topics/not-a-uuid", body: `{}`, call: handler.UpdateTopic},
		{name: "delete topic", method: http.MethodDelete, target: "/knowledge/topics/not-a-uuid", call: handler.DeleteTopic},
		{name: "list entries", method: http.MethodGet, target: "/knowledge/topics/not-a-uuid/entries", call: handler.ListTopicEntries},
		{name: "get entry", method: http.MethodGet, target: "/knowledge/entries/not-a-uuid", call: handler.GetEntry},
		{name: "update entry", method: http.MethodPut, target: "/knowledge/entries/not-a-uuid", body: `{}`, call: handler.UpdateEntry},
		{name: "delete entry", method: http.MethodDelete, target: "/knowledge/entries/not-a-uuid?expectedVersion=1", call: handler.DeleteEntry},
		{name: "undo entry", method: http.MethodPost, target: "/knowledge/entries/not-a-uuid/undo", body: `{}`, call: handler.UndoEntry},
		{name: "organize topic", method: http.MethodPost, target: "/knowledge/topics/not-a-uuid/organize", call: handler.OrganizeTopic},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			e := echo.New()
			request := httptest.NewRequest(test.method, test.target, bytes.NewBufferString(test.body))
			request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			context := e.NewContext(request, httptest.NewRecorder())
			context.Set("userID", uuid.NewString())
			context.SetParamNames("id")
			context.SetParamValues("not-a-uuid")

			assertKnowledgeBadRequest(t, test.call(context))
		})
	}
}

func TestKnowledgeHandlersRejectMalformedBodyAndQueryUUIDsBeforeRepositoryUse(t *testing.T) {
	handler := NewKnowledgeHandler(nil)
	validID := uuid.NewString()
	tests := []struct {
		name   string
		method string
		target string
		body   string
		pathID string
		call   func(echo.Context) error
	}{
		{
			name:   "create topic parentId",
			method: http.MethodPost,
			target: "/knowledge/topics",
			body:   `{"name":"Projects","parentId":"bad"}`,
			call:   handler.CreateTopic,
		},
		{
			name:   "update topic parentId",
			method: http.MethodPut,
			target: "/knowledge/topics/" + validID,
			body:   `{"parentId":"bad"}`,
			pathID: validID,
			call:   handler.UpdateTopic,
		},
		{
			name:   "create entry topicId",
			method: http.MethodPost,
			target: "/knowledge/entries",
			body:   `{"topicId":"bad","title":"Title","body":"Body"}`,
			call:   handler.CreateEntry,
		},
		{
			name:   "update entry topicId",
			method: http.MethodPut,
			target: "/knowledge/entries/" + validID,
			body:   `{"expectedVersion":1,"topicId":"bad"}`,
			pathID: validID,
			call:   handler.UpdateEntry,
		},
		{
			name:   "search topicId",
			method: http.MethodGet,
			target: "/knowledge/search?topicId=bad",
			call:   handler.Search,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			e := echo.New()
			request := httptest.NewRequest(test.method, test.target, bytes.NewBufferString(test.body))
			request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			context := e.NewContext(request, httptest.NewRecorder())
			context.Set("userID", uuid.NewString())
			if test.pathID != "" {
				context.SetParamNames("id")
				context.SetParamValues(test.pathID)
			}

			assertKnowledgeBadRequest(t, test.call(context))
		})
	}
}

func assertKnowledgeBadRequest(t *testing.T, err error) {
	t.Helper()
	httpErr, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("handler error = %T %v, want *echo.HTTPError", err, err)
	}
	if httpErr.Code != http.StatusBadRequest {
		t.Fatalf("handler status = %d, want %d", httpErr.Code, http.StatusBadRequest)
	}
}

type fakeKnowledgeOrganizer struct {
	response *models.KnowledgeOrganizationResponse
	err      error
	userID   string
	topicID  string
}

func (f *fakeKnowledgeOrganizer) OrganizeTopic(_ context.Context, userID, topicID string) (*models.KnowledgeOrganizationResponse, error) {
	f.userID = userID
	f.topicID = topicID
	return f.response, f.err
}

func TestOrganizeTopicUsesAuthenticatedUserAndReturnsUsefulState(t *testing.T) {
	userID := uuid.NewString()
	topicID := uuid.NewString()
	now := time.Now().UTC()
	runner := &fakeKnowledgeOrganizer{response: &models.KnowledgeOrganizationResponse{
		Status: "organized",
		Topic: models.KnowledgeTopic{
			ID:        topicID,
			Name:      "Notes",
			CreatedAt: now,
			UpdatedAt: now,
		},
		Result: models.KnowledgeOrganizationApplyResult{
			OperationsApplied: 2,
			ChangedEntryIDs:   []string{uuid.NewString()},
			AffectedTopicIDs:  []string{topicID},
		},
	}}
	handler := NewKnowledgeHandler(nil, runner)
	echoServer := echo.New()
	request := httptest.NewRequest(http.MethodPost, "/knowledge/topics/"+topicID+"/organize", nil)
	recorder := httptest.NewRecorder()
	context := echoServer.NewContext(request, recorder)
	context.Set("userID", userID)
	context.SetParamNames("id")
	context.SetParamValues(topicID)

	if err := handler.OrganizeTopic(context); err != nil {
		t.Fatalf("OrganizeTopic() error = %v", err)
	}
	if runner.userID != userID || runner.topicID != topicID {
		t.Fatalf("runner received user/topic %q/%q, want %q/%q", runner.userID, runner.topicID, userID, topicID)
	}
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"operationsApplied":2`) {
		t.Fatalf("response status/body = %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestOrganizeTopicMapsOwnershipAndLeaseFailures(t *testing.T) {
	for _, test := range []struct {
		err  error
		code int
	}{
		{err: repository.ErrKnowledgeNotFound, code: http.StatusNotFound},
		{err: repository.ErrKnowledgeConflict, code: http.StatusConflict},
	} {
		t.Run(test.err.Error(), func(t *testing.T) {
			topicID := uuid.NewString()
			handler := NewKnowledgeHandler(nil, &fakeKnowledgeOrganizer{err: test.err})
			echoServer := echo.New()
			context := echoServer.NewContext(
				httptest.NewRequest(http.MethodPost, "/knowledge/topics/"+topicID+"/organize", nil),
				httptest.NewRecorder(),
			)
			context.Set("userID", uuid.NewString())
			context.SetParamNames("id")
			context.SetParamValues(topicID)
			httpErr, ok := handler.OrganizeTopic(context).(*echo.HTTPError)
			if !ok || httpErr.Code != test.code {
				t.Fatalf("error = %#v, want HTTP %d", httpErr, test.code)
			}
		})
	}
}
