package service

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/basketikun/infinite-canvas/model"
	"github.com/basketikun/infinite-canvas/repository"
)

func TestAdminChannelValidationCarriesBadRequestStatus(t *testing.T) {
	setupServiceTestDB(t)

	if _, err := AdminChannelModels(nil, model.ModelChannel{APIKey: "test-key"}); statusCode(err) != http.StatusBadRequest {
		t.Fatalf("expected missing base URL to carry HTTP 400, got %T %v", err, err)
	}
	if _, err := AdminChannelModels(nil, model.ModelChannel{BaseURL: "https://example.invalid"}); statusCode(err) != http.StatusBadRequest {
		t.Fatalf("expected missing API key to carry HTTP 400, got %T %v", err, err)
	}
}

func TestAdminChannelInvalidBaseURLCarriesBadRequestStatus(t *testing.T) {
	setupServiceTestDB(t)
	channel := model.ModelChannel{BaseURL: "://bad", APIKey: "test-key"}

	if _, err := AdminChannelModels(nil, channel); statusCode(err) != http.StatusBadRequest {
		t.Fatalf("expected malformed base URL to carry HTTP 400, got %T %v", err, err)
	}
	if _, err := AdminTestChannelModel(nil, channel, "gpt-test"); statusCode(err) != http.StatusBadRequest {
		t.Fatalf("expected malformed test base URL to carry HTTP 400, got %T %v", err, err)
	}
}

func TestAdminChannelBaseURLWhitespaceIsTrimmedBeforeRequest(t *testing.T) {
	setupServiceTestDB(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/models":
			_, _ = w.Write([]byte(`{"data":[{"id":"gpt-test"}]}`))
		case "/v1/chat/completions":
			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"pong"}}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()
	channel := model.ModelChannel{BaseURL: " \t" + upstream.URL + "/ \n", APIKey: "test-key"}

	models, err := AdminChannelModels(nil, channel)
	if err != nil {
		t.Fatalf("expected whitespace-padded base URL to fetch models, got %T %v", err, err)
	}
	if len(models) != 1 || models[0] != "gpt-test" {
		t.Fatalf("expected fetched model list, got %#v", models)
	}
	result, err := AdminTestChannelModel(nil, channel, "gpt-test")
	if err != nil {
		t.Fatalf("expected whitespace-padded base URL to test model, got %T %v", err, err)
	}
	if result != "pong" {
		t.Fatalf("expected test response content, got %q", result)
	}
}

func TestAdminChannelWhitespaceAPIKeyFallsBackToSavedChannel(t *testing.T) {
	setupServiceTestDB(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer saved-key" {
			t.Fatalf("expected saved API key, got %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"gpt-test"}]}`))
	}))
	defer upstream.Close()
	if _, err := repository.SaveSettings(model.Settings{
		Private: model.PrivateSetting{Channels: []model.ModelChannel{{
			Name:    "saved",
			BaseURL: upstream.URL,
			APIKey:  "saved-key",
		}}},
	}, "now"); err != nil {
		t.Fatal(err)
	}
	index := 0
	channel := model.ModelChannel{Name: "saved", BaseURL: upstream.URL, APIKey: " \t "}

	models, err := AdminChannelModels(&index, channel)
	if err != nil {
		t.Fatalf("expected whitespace API key to reuse saved key, got %T %v", err, err)
	}
	if len(models) != 1 || models[0] != "gpt-test" {
		t.Fatalf("expected fetched model list, got %#v", models)
	}
}

func TestSaveSettingsTrimsModelChannelBaseURLAndAPIKey(t *testing.T) {
	setupServiceTestDB(t)

	if _, err := SaveSettings(model.Settings{
		Private: model.PrivateSetting{Channels: []model.ModelChannel{{
			Name:    "saved",
			BaseURL: " \thttps://api.example.invalid/ \n",
			APIKey:  " saved-key \n",
		}}},
	}); err != nil {
		t.Fatal(err)
	}
	saved, err := repository.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.Private.Channels) != 1 {
		t.Fatalf("expected one saved channel, got %#v", saved.Private.Channels)
	}
	if saved.Private.Channels[0].BaseURL != "https://api.example.invalid/" {
		t.Fatalf("expected trimmed base URL, got %q", saved.Private.Channels[0].BaseURL)
	}
	if saved.Private.Channels[0].APIKey != "saved-key" {
		t.Fatalf("expected trimmed API key, got %q", saved.Private.Channels[0].APIKey)
	}
}

func TestAdminChannelUpstreamErrorsCarryBadGatewayStatus(t *testing.T) {
	for _, tt := range []struct {
		name       string
		body       []byte
		statusCode int
	}{
		{name: "upstream error message", body: []byte(`{"error":{"message":"invalid api key"}}`), statusCode: http.StatusUnauthorized},
		{name: "upstream msg", body: []byte(`{"msg":"model unavailable"}`), statusCode: http.StatusBadRequest},
		{name: "fallback status", body: nil, statusCode: http.StatusInternalServerError},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := readAdminChannelError(tt.body, tt.statusCode, "测试失败")

			if statusCode(err) != http.StatusBadGateway {
				t.Fatalf("expected upstream channel error to carry HTTP 502, got %T %v", err, err)
			}
		})
	}
}

func TestAdminChannelFallbackErrorCarriesBadGatewayStatus(t *testing.T) {
	if err := readAdminChannelError(nil, 0, "测试失败"); statusCode(err) != http.StatusBadGateway {
		t.Fatalf("expected fallback upstream channel error to carry HTTP 502, got %T %v", err, err)
	}
}

func TestAdminChannelNetworkErrorsCarryBadGatewayStatus(t *testing.T) {
	setupServiceTestDB(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	upstream.Close()
	channel := model.ModelChannel{BaseURL: upstream.URL, APIKey: "test-key"}

	if _, err := AdminChannelModels(nil, channel); statusCode(err) != http.StatusBadGateway {
		t.Fatalf("expected model fetch network error to carry HTTP 502, got %T %v", err, err)
	}
	if _, err := AdminTestChannelModel(nil, channel, "gpt-test"); statusCode(err) != http.StatusBadGateway {
		t.Fatalf("expected model test network error to carry HTTP 502, got %T %v", err, err)
	}
}

func TestAdminChannelHTTP200BusinessErrorsCarryBadGatewayStatus(t *testing.T) {
	setupServiceTestDB(t)

	for _, tt := range []struct {
		name string
		body string
	}{
		{name: "error message", body: `{"error":{"message":"bad key"}}`},
		{name: "non zero code", body: `{"code":1,"msg":"failed"}`},
		{name: "success false", body: `{"success":false,"msg":"blocked"}`},
		{name: "failed status", body: `{"status":"failed","message":"model failed"}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tt.body))
			}))
			defer upstream.Close()
			channel := model.ModelChannel{BaseURL: upstream.URL, APIKey: "test-key"}

			if _, err := AdminChannelModels(nil, channel); statusCode(err) != http.StatusBadGateway {
				t.Fatalf("expected model fetch HTTP 200 business error to carry HTTP 502, got %T %v", err, err)
			}
			if _, err := AdminTestChannelModel(nil, channel, "gpt-test"); statusCode(err) != http.StatusBadGateway {
				t.Fatalf("expected model test HTTP 200 business error to carry HTTP 502, got %T %v", err, err)
			}
		})
	}
}

func TestAdminChannelHTTP200InvalidJSONCarriesBadGatewayStatus(t *testing.T) {
	setupServiceTestDB(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`not json`))
	}))
	defer upstream.Close()
	channel := model.ModelChannel{BaseURL: upstream.URL, APIKey: "test-key"}

	if _, err := AdminChannelModels(nil, channel); statusCode(err) != http.StatusBadGateway {
		t.Fatalf("expected model fetch invalid JSON to carry HTTP 502, got %T %v", err, err)
	}
	if _, err := AdminTestChannelModel(nil, channel, "gpt-test"); statusCode(err) != http.StatusBadGateway {
		t.Fatalf("expected model test invalid JSON to carry HTTP 502, got %T %v", err, err)
	}
}

func TestAdminChannelTestModelMissingContentCarriesBadGatewayStatus(t *testing.T) {
	setupServiceTestDB(t)

	for _, tt := range []struct {
		name string
		body string
	}{
		{name: "missing choices", body: `{}`},
		{name: "empty choices", body: `{"choices":[]}`},
		{name: "empty content", body: `{"choices":[{"message":{"content":" "}}]}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tt.body))
			}))
			defer upstream.Close()
			channel := model.ModelChannel{BaseURL: upstream.URL, APIKey: "test-key"}

			if _, err := AdminTestChannelModel(nil, channel, "gpt-test"); statusCode(err) != http.StatusBadGateway {
				t.Fatalf("expected model test missing content to carry HTTP 502, got %T %v", err, err)
			}
		})
	}
}

func TestAdminChannelMissingModelCarriesBadRequestStatus(t *testing.T) {
	setupServiceTestDB(t)

	channel := model.ModelChannel{BaseURL: "https://example.invalid", APIKey: "test-key"}
	if _, err := AdminTestChannelModel(nil, channel, " "); statusCode(err) != http.StatusBadRequest {
		t.Fatalf("expected missing model name to carry HTTP 400, got %T %v", err, err)
	}
}
