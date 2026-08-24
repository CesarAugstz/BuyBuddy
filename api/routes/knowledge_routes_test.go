package routes

import (
	"buybuddy-api/config"
	"buybuddy-api/utils"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

func TestKnowledgeRoutesIncludeOrganizer(t *testing.T) {
	echoServer := echo.New()
	Setup(echoServer, &config.Config{JWTSecret: "test-secret"}, &gorm.DB{})

	routes := make(map[string]bool)
	for _, route := range echoServer.Routes() {
		routes[route.Method+" "+route.Path] = true
	}
	expected := []string{
		"GET /api/knowledge/topics/tree",
		"GET /api/knowledge/topics",
		"GET /api/knowledge/topics/:id",
		"POST /api/knowledge/topics",
		"PUT /api/knowledge/topics/:id",
		"DELETE /api/knowledge/topics/:id",
		"GET /api/knowledge/topics/:id/entries",
		"GET /api/knowledge/entries/:id",
		"POST /api/knowledge/entries",
		"PUT /api/knowledge/entries/:id",
		"DELETE /api/knowledge/entries/:id",
		"GET /api/knowledge/search",
		"POST /api/knowledge/entries/:id/undo",
		"POST /api/knowledge/topics/:id/organize",
	}
	for _, route := range expected {
		if !routes[route] {
			t.Errorf("missing route %s", route)
		}
	}
}

func TestKnowledgeRoutesReturnBadRequestForMalformedUUIDs(t *testing.T) {
	const secret = "test-secret"
	echoServer := echo.New()
	Setup(echoServer, &config.Config{JWTSecret: secret}, &gorm.DB{})
	token, err := utils.GenerateJWT(uuid.NewString(), "knowledge@example.test", secret)
	if err != nil {
		t.Fatalf("GenerateJWT() error = %v", err)
	}

	tests := []struct {
		method string
		target string
		body   string
	}{
		{method: http.MethodGet, target: "/api/knowledge/topics/not-a-uuid"},
		{method: http.MethodGet, target: "/api/knowledge/search?topicId=not-a-uuid"},
		{method: http.MethodPost, target: "/api/knowledge/entries", body: `{"topicId":"not-a-uuid","title":"Title","body":"Body"}`},
	}
	for _, test := range tests {
		request := httptest.NewRequest(test.method, test.target, strings.NewReader(test.body))
		request.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
		request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		recorder := httptest.NewRecorder()

		echoServer.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusBadRequest {
			t.Errorf("%s %s status = %d, want %d; body=%s", test.method, test.target, recorder.Code, http.StatusBadRequest, recorder.Body.String())
		}
		if strings.Contains(strings.ToLower(recorder.Body.String()), "postgres") {
			t.Errorf("%s %s exposed driver details: %s", test.method, test.target, recorder.Body.String())
		}
	}
}

func TestKnowledgeOrganizeRouteRequiresAuthentication(t *testing.T) {
	echoServer := echo.New()
	Setup(echoServer, &config.Config{JWTSecret: "test-secret"}, &gorm.DB{})
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/knowledge/topics/"+uuid.NewString()+"/organize",
		nil,
	)
	recorder := httptest.NewRecorder()

	echoServer.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated organize status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}
