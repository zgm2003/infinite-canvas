package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"testing"
)

func TestRegisteredRouteSurface(t *testing.T) {
	t.Setenv("GIN_MODE", "test")
	engine := New()

	var routes []string
	for _, route := range engine.Routes() {
		routes = append(routes, route.Method+" "+route.Path)
	}
	sort.Strings(routes)

	expected := []string{
		"DELETE /api/admin/assets/:id",
		"DELETE /api/admin/credit-logs/:id",
		"DELETE /api/admin/prompts/:id",
		"DELETE /api/admin/users/:id",
		"GET /api/admin/assets",
		"GET /api/admin/credit-logs",
		"GET /api/admin/prompt-categories",
		"GET /api/admin/prompts",
		"GET /api/admin/settings",
		"GET /api/admin/users",
		"GET /api/assets",
		"GET /api/auth/linux-do/authorize",
		"GET /api/auth/linux-do/callback",
		"GET /api/auth/me",
		"GET /api/health",
		"GET /api/prompts",
		"GET /api/settings",
		"GET /api/v1/videos/:id",
		"GET /api/v1/videos/:id/content",
		"POST /api/admin/assets",
		"POST /api/admin/credit-logs",
		"POST /api/admin/login",
		"POST /api/admin/prompt-categories/sync",
		"POST /api/admin/prompts",
		"POST /api/admin/prompts/batch-delete",
		"POST /api/admin/settings",
		"POST /api/admin/settings/channel-models",
		"POST /api/admin/settings/channel-test",
		"POST /api/admin/users",
		"POST /api/admin/users/:id/credits",
		"POST /api/auth/login",
		"POST /api/auth/register",
		"POST /api/v1/chat/completions",
		"POST /api/v1/images/edits",
		"POST /api/v1/images/generations",
		"POST /api/v1/videos",
	}

	if !reflect.DeepEqual(routes, expected) {
		t.Fatalf("route surface changed\nexpected: %#v\nactual:   %#v", expected, routes)
	}
}

func TestProtectedRoutesUseHTTPUnauthorized(t *testing.T) {
	t.Setenv("GIN_MODE", "test")
	engine := New()

	for _, route := range []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "user api", method: http.MethodPost, path: "/api/v1/chat/completions", body: `{}`},
		{name: "admin api", method: http.MethodGet, path: "/api/admin/users"},
	} {
		t.Run(route.name, func(t *testing.T) {
			request := httptest.NewRequest(route.method, route.path, nil)
			response := httptest.NewRecorder()

			engine.ServeHTTP(response, request)

			if response.Code != http.StatusUnauthorized {
				t.Fatalf("expected HTTP 401, got %d: %s", response.Code, response.Body.String())
			}

			var payload struct {
				Code int    `json:"code"`
				Msg  string `json:"msg"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
				t.Fatalf("response is not JSON: %v", err)
			}
			if payload.Code != 1 || payload.Msg == "" {
				t.Fatalf("unexpected error payload: %+v", payload)
			}
		})
	}
}
