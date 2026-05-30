package service

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/basketikun/infinite-canvas/model"
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

func TestAdminChannelMissingModelCarriesBadRequestStatus(t *testing.T) {
	setupServiceTestDB(t)

	channel := model.ModelChannel{BaseURL: "https://example.invalid", APIKey: "test-key"}
	if _, err := AdminTestChannelModel(nil, channel, " "); statusCode(err) != http.StatusBadRequest {
		t.Fatalf("expected missing model name to carry HTTP 400, got %T %v", err, err)
	}
}
