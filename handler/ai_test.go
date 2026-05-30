package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/basketikun/infinite-canvas/model"
	"github.com/basketikun/infinite-canvas/repository"
	"github.com/basketikun/infinite-canvas/service"
)

func TestReadAIRequestCountCapsOverflowingJSONN(t *testing.T) {
	count := readAIRequestCount([]byte(`{"model":"gpt-image-1","n":9223372036854775807}`), "application/json")

	if count != 15 {
		t.Fatalf("expected count capped at 15, got %d", count)
	}
}

func TestReadAIRequestCountCapsOverflowingMultipartN(t *testing.T) {
	body := "--boundary\r\nContent-Disposition: form-data; name=\"n\"\r\n\r\n9223372036854775807\r\n--boundary--\r\n"
	count := readAIRequestCount([]byte(body), "multipart/form-data; boundary=boundary")

	if count != 15 {
		t.Fatalf("expected count capped at 15, got %d", count)
	}
}

func TestAIProxyRejectsBadRequestsWithHTTPStatus(t *testing.T) {
	for _, tt := range []struct {
		name string
		body string
		want int
	}{
		{name: "missing model", body: `{}`, want: http.StatusBadRequest},
		{name: "missing user context", body: `{"model":"gpt-test"}`, want: http.StatusUnauthorized},
	} {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/v1/chat/completions", strings.NewReader(tt.body))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()

			AIChatCompletions(response, request)

			if response.Code != tt.want {
				t.Fatalf("expected HTTP %d, got %d: %s", tt.want, response.Code, response.Body.String())
			}
		})
	}
}

func TestAIProxyReturnsBadGatewayWhenNoModelChannel(t *testing.T) {
	setupHandlerTestDB(t)
	const modelName = "gpt-no-channel"
	user := model.User{ID: "user_no_channel", Username: "no-channel", Role: model.UserRoleUser, Credits: 10, AffCode: "aff_no_channel", Status: model.UserStatusActive, CreatedAt: "now", UpdatedAt: "now"}
	if _, err := repository.SaveUser(user); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.SaveSettings(model.Settings{
		Public: model.PublicSetting{ModelChannel: model.PublicModelChannelSetting{ModelCosts: []model.ModelCost{{Model: modelName, Credits: 1}}}},
	}, "now"); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/chat/completions", strings.NewReader(`{"model":"`+modelName+`"}`))
	request.Header.Set("Content-Type", "application/json")
	request = request.WithContext(service.WithUser(request.Context(), model.AuthUser{ID: user.ID, Role: model.UserRoleUser}))
	response := httptest.NewRecorder()

	AIChatCompletions(response, request)

	if response.Code != http.StatusBadGateway {
		t.Fatalf("expected no model channel to return HTTP 502, got %d: %s", response.Code, response.Body.String())
	}
}

func TestCopyAIResponseRefundsWhenClientWriteFails(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("generated result"))
	}))
	t.Cleanup(upstream.Close)

	request, err := http.NewRequest(http.MethodGet, upstream.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	refunded := false

	copyAIResponse(errorResponseWriter{header: http.Header{}}, request, func() {
		refunded = true
	})

	if !refunded {
		t.Fatal("expected response copy failure to trigger refund callback")
	}
}

func TestCopyAIResponseRefundsWhenUpstreamBusinessErrorIsHTTP200(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"error":{"message":"quota exceeded"}}`))
	}))
	t.Cleanup(upstream.Close)

	request, err := http.NewRequest(http.MethodGet, upstream.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	refunded := false
	response := httptest.NewRecorder()

	copyAIResponse(response, request, func() {
		refunded = true
	})

	if response.Code != http.StatusBadGateway {
		t.Fatalf("expected HTTP %d, got %d: %s", http.StatusBadGateway, response.Code, response.Body.String())
	}
	if !refunded {
		t.Fatal("expected HTTP 200 business error body to trigger refund callback")
	}
}

func TestCopyAIResponseRefundsWhenUpstreamCodeErrorIsHTTP200(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":1,"msg":"quota exceeded"}`))
	}))
	t.Cleanup(upstream.Close)

	request, err := http.NewRequest(http.MethodGet, upstream.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	refunded := false
	response := httptest.NewRecorder()

	copyAIResponse(response, request, func() {
		refunded = true
	})

	if response.Code != http.StatusBadGateway {
		t.Fatalf("expected HTTP %d, got %d: %s", http.StatusBadGateway, response.Code, response.Body.String())
	}
	if !refunded {
		t.Fatal("expected HTTP 200 code error body to trigger refund callback")
	}
}

func TestCopyAIResponseRefundsWhenUpstreamStringCodeErrorIsHTTP200(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":"400","msg":"quota exceeded"}`))
	}))
	t.Cleanup(upstream.Close)

	request, err := http.NewRequest(http.MethodGet, upstream.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	refunded := false
	response := httptest.NewRecorder()

	copyAIResponse(response, request, func() {
		refunded = true
	})

	if response.Code != http.StatusBadGateway {
		t.Fatalf("expected HTTP %d, got %d: %s", http.StatusBadGateway, response.Code, response.Body.String())
	}
	if !refunded {
		t.Fatal("expected HTTP 200 string code error body to trigger refund callback")
	}
}

func TestAIUpstreamBusinessFailureRecognizesErrorStatusAndSuccessFalse(t *testing.T) {
	for _, body := range [][]byte{
		[]byte(`{"status":"error","message":"quota exceeded"}`),
		[]byte(`{"success":false,"message":"quota exceeded"}`),
	} {
		if !isAIUpstreamBusinessFailure(body) {
			t.Fatalf("expected business failure for body %s", body)
		}
	}
}

func TestCopyAIResponseRefundsWhenBusinessErrorLacksJSONContentType(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"error":{"message":"quota exceeded"}}`))
	}))
	t.Cleanup(upstream.Close)

	request, err := http.NewRequest(http.MethodGet, upstream.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	refunded := false
	response := httptest.NewRecorder()

	copyAIResponse(response, request, func() {
		refunded = true
	})

	if response.Code != http.StatusBadGateway {
		t.Fatalf("expected HTTP %d, got %d: %s", http.StatusBadGateway, response.Code, response.Body.String())
	}
	if !refunded {
		t.Fatal("expected HTTP 200 JSON business error without content-type to trigger refund callback")
	}
}

func TestCopyAIResponseRefundsWhenLargeJSONBusinessErrorIsHTTP200(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"padding":"` + strings.Repeat("x", 70<<10) + `","status":"failed"}`))
	}))
	t.Cleanup(upstream.Close)

	request, err := http.NewRequest(http.MethodGet, upstream.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	refunded := false
	response := httptest.NewRecorder()

	copyAIResponse(response, request, func() {
		refunded = true
	})

	if response.Code != http.StatusBadGateway {
		t.Fatalf("expected HTTP %d, got %d: %s", http.StatusBadGateway, response.Code, response.Body.String())
	}
	if !refunded {
		t.Fatal("expected large HTTP 200 JSON business error body to trigger refund callback")
	}
}

func TestCopyAIResponseDoesNotPreReadStreamingResponseWithoutContentType(t *testing.T) {
	release := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("data: {\"delta\":\"hi\"}\n\n"))
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		<-release
	}))
	t.Cleanup(upstream.Close)

	request, err := http.NewRequest(http.MethodPost, upstream.URL, strings.NewReader(`{"stream":true}`))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	wrote := make(chan []byte, 1)
	done := make(chan struct{})
	go func() {
		copyAIResponse(notifyingResponseWriter{header: http.Header{}, wrote: wrote}, request, func() {})
		close(done)
	}()

	select {
	case body := <-wrote:
		if !strings.Contains(string(body), "data:") {
			t.Fatalf("expected streamed SSE chunk, got %q", string(body))
		}
	case <-time.After(150 * time.Millisecond):
		close(release)
		<-done
		t.Fatal("expected stream chunk before upstream response finished")
	}
	close(release)
	<-done
}

func TestCopyAIResponseDoesNotPreReadStreamingResponseWithJSONContentType(t *testing.T) {
	release := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("data: {\"delta\":\"hi\"}\n\n"))
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		<-release
	}))
	t.Cleanup(upstream.Close)

	request, err := http.NewRequest(http.MethodPost, upstream.URL, strings.NewReader(`{"stream":true}`))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	wrote := make(chan []byte, 1)
	done := make(chan struct{})
	go func() {
		copyAIResponse(notifyingResponseWriter{header: http.Header{}, wrote: wrote}, request, func() {})
		close(done)
	}()

	select {
	case body := <-wrote:
		if !strings.Contains(string(body), "data:") {
			t.Fatalf("expected streamed SSE chunk, got %q", string(body))
		}
	case <-time.After(150 * time.Millisecond):
		close(release)
		<-done
		t.Fatal("expected stream chunk before upstream response finished")
	}
	close(release)
	<-done
}

func TestCopyAIResponseDoesNotPreReadLargeStreamingRequest(t *testing.T) {
	release := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("data: {\"delta\":\"hi\"}\n\n"))
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		<-release
	}))
	t.Cleanup(upstream.Close)

	requestBody := `{"messages":[{"role":"user","content":"` + strings.Repeat("x", 70<<10) + `"}],"stream":true}`
	request, err := http.NewRequest(http.MethodPost, upstream.URL, strings.NewReader(requestBody))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	wrote := make(chan []byte, 1)
	done := make(chan struct{})
	go func() {
		copyAIResponse(notifyingResponseWriter{header: http.Header{}, wrote: wrote}, request, func() {})
		close(done)
	}()

	select {
	case body := <-wrote:
		if !strings.Contains(string(body), "data:") {
			t.Fatalf("expected streamed SSE chunk, got %q", string(body))
		}
	case <-time.After(150 * time.Millisecond):
		close(release)
		<-done
		t.Fatal("expected large stream request to avoid pre-reading upstream response")
	}
	close(release)
	<-done
}

func TestCopyAIResponseRefundsStreamingJSONBusinessError(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"error":{"message":"stream failed"}}`))
	}))
	t.Cleanup(upstream.Close)

	request, err := http.NewRequest(http.MethodPost, upstream.URL, strings.NewReader(`{"stream":true}`))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	refunded := false
	response := httptest.NewRecorder()

	copyAIResponse(response, request, func() {
		refunded = true
	})

	if response.Code != http.StatusBadGateway {
		t.Fatalf("expected HTTP %d, got %d: %s", http.StatusBadGateway, response.Code, response.Body.String())
	}
	if !refunded {
		t.Fatal("expected streaming JSON business error to trigger refund callback")
	}
}

func TestAIVideoQueryFailureRefundsBoundTaskOnce(t *testing.T) {
	setupHandlerTestDB(t)
	const modelName = "grok-imagine-video"
	const taskID = "video_task_1"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/videos":
			_, _ = w.Write([]byte(`{"id":"` + taskID + `","status":"queued"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/videos/"+taskID:
			_, _ = w.Write([]byte(`{"id":"` + taskID + `","status":"failed","error":{"message":"render failed"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(upstream.Close)
	user := model.User{ID: "user_video_1", Username: "video-user", Role: model.UserRoleUser, Credits: 10, AffCode: "aff_video_1", Status: model.UserStatusActive, CreatedAt: "now", UpdatedAt: "now"}
	if _, err := repository.SaveUser(user); err != nil {
		t.Fatal(err)
	}
	_, err := repository.SaveSettings(model.Settings{
		Public:  model.PublicSetting{ModelChannel: model.PublicModelChannelSetting{ModelCosts: []model.ModelCost{{Model: modelName, Credits: 4}}}},
		Private: model.PrivateSetting{Channels: []model.ModelChannel{{BaseURL: upstream.URL, APIKey: "test-key", Models: []string{modelName}, Enabled: true, Weight: 1}}},
	}, "now")
	if err != nil {
		t.Fatal(err)
	}

	createRequest := httptest.NewRequest(http.MethodPost, "/api/v1/videos", strings.NewReader(`{"model":"`+modelName+`"}`))
	createRequest.Header.Set("Content-Type", "application/json")
	createRequest = createRequest.WithContext(service.WithUser(createRequest.Context(), model.AuthUser{ID: user.ID, Role: model.UserRoleUser}))
	createResponse := httptest.NewRecorder()
	AIVideos(createResponse, createRequest)
	if createResponse.Code != http.StatusOK {
		t.Fatalf("create expected HTTP 200, got %d: %s", createResponse.Code, createResponse.Body.String())
	}

	queryRequest := httptest.NewRequest(http.MethodGet, "/api/v1/videos/"+taskID+"?model="+modelName, nil)
	queryRequest = queryRequest.WithContext(service.WithUser(queryRequest.Context(), model.AuthUser{ID: user.ID, Role: model.UserRoleUser}))
	for range 2 {
		queryResponse := httptest.NewRecorder()
		AIVideo(queryResponse, queryRequest, taskID)
		if queryResponse.Code != http.StatusOK {
			t.Fatalf("query expected HTTP 200, got %d: %s", queryResponse.Code, queryResponse.Body.String())
		}
	}

	refreshed, ok, err := repository.GetUserByID(user.ID)
	if err != nil || !ok {
		t.Fatalf("expected user, ok=%v err=%v", ok, err)
	}
	if refreshed.Credits != 10 {
		t.Fatalf("expected failed video task to refund once back to 10 credits, got %d", refreshed.Credits)
	}
	logs, _, err := repository.ListCreditLogs(model.Query{Keyword: taskID, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	refunds := 0
	for _, item := range logs {
		if item.Type == model.CreditLogTypeAIRefund {
			refunds++
		}
	}
	if refunds != 1 {
		t.Fatalf("expected exactly one refund log for task, got %d", refunds)
	}
}

func TestAIVideoCreateWithoutTaskIDRefunds(t *testing.T) {
	setupHandlerTestDB(t)
	const modelName = "grok-imagine-video"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/videos" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"queued"}`))
	}))
	t.Cleanup(upstream.Close)
	user := model.User{ID: "user_video_missing_id", Username: "video-missing-id", Role: model.UserRoleUser, Credits: 10, AffCode: "aff_video_missing_id", Status: model.UserStatusActive, CreatedAt: "now", UpdatedAt: "now"}
	if _, err := repository.SaveUser(user); err != nil {
		t.Fatal(err)
	}
	_, err := repository.SaveSettings(model.Settings{
		Public:  model.PublicSetting{ModelChannel: model.PublicModelChannelSetting{ModelCosts: []model.ModelCost{{Model: modelName, Credits: 4}}}},
		Private: model.PrivateSetting{Channels: []model.ModelChannel{{BaseURL: upstream.URL, APIKey: "test-key", Models: []string{modelName}, Enabled: true, Weight: 1}}},
	}, "now")
	if err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/v1/videos", strings.NewReader(`{"model":"`+modelName+`"}`))
	request.Header.Set("Content-Type", "application/json")
	request = request.WithContext(service.WithUser(request.Context(), model.AuthUser{ID: user.ID, Role: model.UserRoleUser}))
	response := httptest.NewRecorder()

	AIVideos(response, request)

	if response.Code != http.StatusBadGateway {
		t.Fatalf("expected missing video task id to return HTTP 502, got %d: %s", response.Code, response.Body.String())
	}
	refreshed, ok, err := repository.GetUserByID(user.ID)
	if err != nil || !ok {
		t.Fatalf("expected user, ok=%v err=%v", ok, err)
	}
	if refreshed.Credits != 10 {
		t.Fatalf("expected missing task id to refund credits back to 10, got %d", refreshed.Credits)
	}
}

func TestAIVideoCreateWithoutJSONContentTypeStillValidatesTaskID(t *testing.T) {
	setupHandlerTestDB(t)
	const modelName = "grok-imagine-video"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/videos" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"status":"queued"}`))
	}))
	t.Cleanup(upstream.Close)
	user := model.User{ID: "user_video_missing_json_content_type", Username: "video-missing-json-content-type", Role: model.UserRoleUser, Credits: 10, AffCode: "aff_video_missing_json_content_type", Status: model.UserStatusActive, CreatedAt: "now", UpdatedAt: "now"}
	if _, err := repository.SaveUser(user); err != nil {
		t.Fatal(err)
	}
	_, err := repository.SaveSettings(model.Settings{
		Public:  model.PublicSetting{ModelChannel: model.PublicModelChannelSetting{ModelCosts: []model.ModelCost{{Model: modelName, Credits: 4}}}},
		Private: model.PrivateSetting{Channels: []model.ModelChannel{{BaseURL: upstream.URL, APIKey: "test-key", Models: []string{modelName}, Enabled: true, Weight: 1}}},
	}, "now")
	if err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/v1/videos", strings.NewReader(`{"model":"`+modelName+`"}`))
	request.Header.Set("Content-Type", "application/json")
	request = request.WithContext(service.WithUser(request.Context(), model.AuthUser{ID: user.ID, Role: model.UserRoleUser}))
	response := httptest.NewRecorder()

	AIVideos(response, request)

	if response.Code != http.StatusBadGateway {
		t.Fatalf("expected missing task id without JSON content-type to return HTTP 502, got %d: %s", response.Code, response.Body.String())
	}
	refreshed, ok, err := repository.GetUserByID(user.ID)
	if err != nil || !ok {
		t.Fatalf("expected user, ok=%v err=%v", ok, err)
	}
	if refreshed.Credits != 10 {
		t.Fatalf("expected missing task id without JSON content-type to refund credits back to 10, got %d", refreshed.Credits)
	}
}

func TestAIVideoCreateAcceptsLargeJSONResponseWithTaskID(t *testing.T) {
	setupHandlerTestDB(t)
	const modelName = "grok-imagine-video"
	const taskID = "video_task_large_json"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/videos" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"padding":"` + strings.Repeat("x", 70<<10) + `","id":"` + taskID + `","status":"queued"}`))
	}))
	t.Cleanup(upstream.Close)
	user := model.User{ID: "user_video_large_json", Username: "video-large-json", Role: model.UserRoleUser, Credits: 10, AffCode: "aff_video_large_json", Status: model.UserStatusActive, CreatedAt: "now", UpdatedAt: "now"}
	if _, err := repository.SaveUser(user); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.SaveSettings(model.Settings{
		Public:  model.PublicSetting{ModelChannel: model.PublicModelChannelSetting{ModelCosts: []model.ModelCost{{Model: modelName, Credits: 4}}}},
		Private: model.PrivateSetting{Channels: []model.ModelChannel{{Name: "large-json", BaseURL: upstream.URL, APIKey: "test-key", Models: []string{modelName}, Enabled: true, Weight: 1}}},
	}, "now"); err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/v1/videos", strings.NewReader(`{"model":"`+modelName+`"}`))
	request.Header.Set("Content-Type", "application/json")
	request = request.WithContext(service.WithUser(request.Context(), model.AuthUser{ID: user.ID, Role: model.UserRoleUser}))
	response := httptest.NewRecorder()

	AIVideos(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected large video create JSON to return HTTP 200, got %d: %s", response.Code, response.Body.String())
	}
	if _, ok, err := repository.GetCreditLogByRelatedID(user.ID, taskID, model.CreditLogTypeAIConsume); err != nil || !ok {
		t.Fatalf("expected credit log bound to large JSON task, ok=%v err=%v", ok, err)
	}
}

func TestAIVideoCreateRefundsWhenTaskBindingFails(t *testing.T) {
	setupHandlerTestDB(t)
	const modelName = "grok-imagine-video"
	const taskID = "video_task_bind_failure"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/videos" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"` + taskID + `","status":"queued"}`))
	}))
	t.Cleanup(upstream.Close)
	user := model.User{ID: "user_video_bind_failure", Username: "video-bind-failure", Role: model.UserRoleUser, Credits: 10, AffCode: "aff_video_bind_failure", Status: model.UserStatusActive, CreatedAt: "now", UpdatedAt: "now"}
	if _, err := repository.SaveUser(user); err != nil {
		t.Fatal(err)
	}
	_, err := repository.SaveSettings(model.Settings{
		Public:  model.PublicSetting{ModelChannel: model.PublicModelChannelSetting{ModelCosts: []model.ModelCost{{Model: modelName, Credits: 4}}}},
		Private: model.PrivateSetting{Channels: []model.ModelChannel{{BaseURL: upstream.URL, APIKey: "test-key", Models: []string{modelName}, Enabled: true, Weight: 1}}},
	}, "now")
	if err != nil {
		t.Fatal(err)
	}
	db, err := repository.DB()
	if err != nil {
		t.Fatal(err)
	}
	const triggerName = "block_video_credit_log_task_binding"
	_ = db.Exec("DROP TRIGGER IF EXISTS " + triggerName).Error
	if err := db.Exec("CREATE TRIGGER " + triggerName + " BEFORE UPDATE OF related_id ON credit_logs BEGIN SELECT RAISE(ABORT, 'video task binding blocked'); END;").Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = db.Exec("DROP TRIGGER IF EXISTS " + triggerName).Error
	})

	request := httptest.NewRequest(http.MethodPost, "/api/v1/videos", strings.NewReader(`{"model":"`+modelName+`"}`))
	request.Header.Set("Content-Type", "application/json")
	request = request.WithContext(service.WithUser(request.Context(), model.AuthUser{ID: user.ID, Role: model.UserRoleUser}))
	response := httptest.NewRecorder()

	AIVideos(response, request)

	if response.Code != http.StatusBadGateway {
		t.Fatalf("expected binding failure to return HTTP 502, got %d: %s", response.Code, response.Body.String())
	}
	refreshed, ok, err := repository.GetUserByID(user.ID)
	if err != nil || !ok {
		t.Fatalf("expected user, ok=%v err=%v", ok, err)
	}
	if refreshed.Credits != 10 {
		t.Fatalf("expected binding failure to refund credits back to 10, got %d", refreshed.Credits)
	}
}

func TestAIVideoQueryUsesBoundTaskChannel(t *testing.T) {
	setupHandlerTestDB(t)
	const modelName = "grok-imagine-video"
	const taskID = "video_task_bound_channel"
	wrongUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "wrong channel", http.StatusNotFound)
	}))
	t.Cleanup(wrongUpstream.Close)
	correctUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/videos/"+taskID {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"` + taskID + `","status":"completed"}`))
	}))
	t.Cleanup(correctUpstream.Close)
	user := model.User{ID: "user_video_bound_channel", Username: "video-bound-channel", Role: model.UserRoleUser, Credits: 10, AffCode: "aff_video_bound_channel", Status: model.UserStatusActive, CreatedAt: "now", UpdatedAt: "now"}
	if _, err := repository.SaveUser(user); err != nil {
		t.Fatal(err)
	}
	_, err := repository.SaveSettings(model.Settings{
		Private: model.PrivateSetting{Channels: []model.ModelChannel{
			{Name: "wrong", BaseURL: wrongUpstream.URL, APIKey: "wrong-key", Models: []string{modelName}, Enabled: true, Weight: 1},
			{Name: "correct", BaseURL: correctUpstream.URL, APIKey: "correct-key", Models: []string{}, Enabled: true, Weight: 1},
		}},
	}, "now")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.SaveCreditLog(model.CreditLog{
		ID:        "credit_bound_channel",
		UserID:    user.ID,
		Type:      model.CreditLogTypeAIConsume,
		Amount:    -4,
		Balance:   6,
		RelatedID: taskID,
		Extra:     `{"model":"` + modelName + `","path":"/videos","taskId":"` + taskID + `","channelName":"correct","channelBaseUrl":"` + correctUpstream.URL + `"}`,
		CreatedAt: "now",
	}); err != nil {
		t.Fatal(err)
	}

	queryRequest := httptest.NewRequest(http.MethodGet, "/api/v1/videos/"+taskID+"?model="+modelName, nil)
	queryRequest = queryRequest.WithContext(service.WithUser(queryRequest.Context(), model.AuthUser{ID: user.ID, Role: model.UserRoleUser}))
	queryResponse := httptest.NewRecorder()

	AIVideo(queryResponse, queryRequest, taskID)

	if queryResponse.Code != http.StatusOK {
		t.Fatalf("expected bound task channel HTTP 200, got %d: %s", queryResponse.Code, queryResponse.Body.String())
	}
	if !strings.Contains(queryResponse.Body.String(), `"status":"completed"`) {
		t.Fatalf("expected response from correct channel, got %s", queryResponse.Body.String())
	}
}

func TestAIVideoQueryDoesNotFallbackWhenBoundChannelIsStale(t *testing.T) {
	setupHandlerTestDB(t)
	const modelName = "grok-imagine-video"
	const taskID = "video_task_stale_bound_channel"
	fallbackHits := 0
	fallbackUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fallbackHits++
		_, _ = w.Write([]byte(`{"id":"` + taskID + `","status":"completed"}`))
	}))
	t.Cleanup(fallbackUpstream.Close)
	user := model.User{ID: "user_video_stale_bound_channel", Username: "video-stale-bound-channel", Role: model.UserRoleUser, Credits: 10, AffCode: "aff_video_stale_bound_channel", Status: model.UserStatusActive, CreatedAt: "now", UpdatedAt: "now"}
	if _, err := repository.SaveUser(user); err != nil {
		t.Fatal(err)
	}
	_, err := repository.SaveSettings(model.Settings{
		Private: model.PrivateSetting{Channels: []model.ModelChannel{
			{Name: "wrong", BaseURL: fallbackUpstream.URL, APIKey: "wrong-key", Models: []string{modelName}, Enabled: true, Weight: 1},
		}},
	}, "now")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.SaveCreditLog(model.CreditLog{
		ID:        "credit_stale_bound_channel",
		UserID:    user.ID,
		Type:      model.CreditLogTypeAIConsume,
		Amount:    -4,
		Balance:   6,
		RelatedID: taskID,
		Extra:     `{"model":"` + modelName + `","path":"/videos","taskId":"` + taskID + `","channelName":"correct","channelBaseUrl":"` + fallbackUpstream.URL + `"}`,
		CreatedAt: "now",
	}); err != nil {
		t.Fatal(err)
	}

	queryRequest := httptest.NewRequest(http.MethodGet, "/api/v1/videos/"+taskID+"?model="+modelName, nil)
	queryRequest = queryRequest.WithContext(service.WithUser(queryRequest.Context(), model.AuthUser{ID: user.ID, Role: model.UserRoleUser}))
	queryResponse := httptest.NewRecorder()

	AIVideo(queryResponse, queryRequest, taskID)

	if queryResponse.Code != http.StatusBadGateway {
		t.Fatalf("expected stale bound channel to fail closed with HTTP 502, got %d: %s", queryResponse.Code, queryResponse.Body.String())
	}
	if fallbackHits != 0 {
		t.Fatalf("expected stale bound channel not to hit fallback channel, got %d hits", fallbackHits)
	}
}

func TestAIVideoQueryInvalidBoundChannelURLReturnsBadGateway(t *testing.T) {
	setupHandlerTestDB(t)
	const modelName = "grok-imagine-video"
	const taskID = "video_task_invalid_bound_channel_url"
	const badBaseURL = "://bad"
	user := model.User{ID: "user_video_invalid_bound_channel_url", Username: "video-invalid-bound-channel-url", Role: model.UserRoleUser, Credits: 10, AffCode: "aff_video_invalid_bound_channel_url", Status: model.UserStatusActive, CreatedAt: "now", UpdatedAt: "now"}
	if _, err := repository.SaveUser(user); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.SaveSettings(model.Settings{
		Private: model.PrivateSetting{Channels: []model.ModelChannel{
			{Name: "bad", BaseURL: badBaseURL, APIKey: "bad-key", Models: []string{modelName}, Enabled: true, Weight: 1},
		}},
	}, "now"); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.SaveCreditLog(model.CreditLog{
		ID:        "credit_invalid_bound_channel_url",
		UserID:    user.ID,
		Type:      model.CreditLogTypeAIConsume,
		Amount:    -4,
		Balance:   6,
		RelatedID: taskID,
		Extra:     `{"model":"` + modelName + `","path":"/videos","taskId":"` + taskID + `","channelName":"bad","channelBaseUrl":"` + badBaseURL + `"}`,
		CreatedAt: "now",
	}); err != nil {
		t.Fatal(err)
	}

	queryRequest := httptest.NewRequest(http.MethodGet, "/api/v1/videos/"+taskID+"?model="+modelName, nil)
	queryRequest = queryRequest.WithContext(service.WithUser(queryRequest.Context(), model.AuthUser{ID: user.ID, Role: model.UserRoleUser}))
	queryResponse := httptest.NewRecorder()

	AIVideo(queryResponse, queryRequest, taskID)

	if queryResponse.Code != http.StatusBadGateway {
		t.Fatalf("expected invalid bound channel URL to return HTTP 502, got %d: %s", queryResponse.Code, queryResponse.Body.String())
	}
}

func TestAIVideoQueryFailsClosedWhenTaskIsNotBoundToCurrentUser(t *testing.T) {
	setupHandlerTestDB(t)
	const modelName = "grok-imagine-video"
	const taskID = "video_task_other_user"
	fallbackHits := 0
	fallbackUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fallbackHits++
		_, _ = w.Write([]byte(`{"id":"` + taskID + `","status":"completed"}`))
	}))
	t.Cleanup(fallbackUpstream.Close)
	userA := model.User{ID: "user_video_owner", Username: "video-owner", Role: model.UserRoleUser, Credits: 10, AffCode: "aff_video_owner", Status: model.UserStatusActive, CreatedAt: "now", UpdatedAt: "now"}
	userB := model.User{ID: "user_video_intruder", Username: "video-intruder", Role: model.UserRoleUser, Credits: 10, AffCode: "aff_video_intruder", Status: model.UserStatusActive, CreatedAt: "now", UpdatedAt: "now"}
	if _, err := repository.SaveUser(userA); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.SaveUser(userB); err != nil {
		t.Fatal(err)
	}
	_, err := repository.SaveSettings(model.Settings{
		Private: model.PrivateSetting{Channels: []model.ModelChannel{
			{Name: "fallback", BaseURL: fallbackUpstream.URL, APIKey: "fallback-key", Models: []string{modelName}, Enabled: true, Weight: 1},
		}},
	}, "now")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.SaveCreditLog(model.CreditLog{
		ID:        "credit_other_user_task",
		UserID:    userA.ID,
		Type:      model.CreditLogTypeAIConsume,
		Amount:    -4,
		Balance:   6,
		RelatedID: taskID,
		Extra:     `{"model":"` + modelName + `","path":"/videos","taskId":"` + taskID + `","channelName":"fallback","channelBaseUrl":"` + fallbackUpstream.URL + `"}`,
		CreatedAt: "now",
	}); err != nil {
		t.Fatal(err)
	}

	queryRequest := httptest.NewRequest(http.MethodGet, "/api/v1/videos/"+taskID+"?model="+modelName, nil)
	queryRequest = queryRequest.WithContext(service.WithUser(queryRequest.Context(), model.AuthUser{ID: userB.ID, Role: model.UserRoleUser}))
	queryResponse := httptest.NewRecorder()

	AIVideo(queryResponse, queryRequest, taskID)

	if queryResponse.Code != http.StatusBadGateway {
		t.Fatalf("expected unbound task to fail closed with HTTP 502, got %d: %s", queryResponse.Code, queryResponse.Body.String())
	}
	if fallbackHits != 0 {
		t.Fatalf("expected unbound task not to hit fallback channel, got %d hits", fallbackHits)
	}
}

func TestAIVideoContentFailsClosedWhenTaskIsNotBoundToCurrentUser(t *testing.T) {
	setupHandlerTestDB(t)
	const modelName = "grok-imagine-video"
	const taskID = "video_content_other_user"
	fallbackHits := 0
	fallbackUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fallbackHits++
		_, _ = w.Write([]byte("video bytes"))
	}))
	t.Cleanup(fallbackUpstream.Close)
	user := model.User{ID: "user_video_content_intruder", Username: "video-content-intruder", Role: model.UserRoleUser, Credits: 10, AffCode: "aff_video_content_intruder", Status: model.UserStatusActive, CreatedAt: "now", UpdatedAt: "now"}
	if _, err := repository.SaveUser(user); err != nil {
		t.Fatal(err)
	}
	_, err := repository.SaveSettings(model.Settings{
		Private: model.PrivateSetting{Channels: []model.ModelChannel{
			{Name: "fallback", BaseURL: fallbackUpstream.URL, APIKey: "fallback-key", Models: []string{modelName}, Enabled: true, Weight: 1},
		}},
	}, "now")
	if err != nil {
		t.Fatal(err)
	}

	queryRequest := httptest.NewRequest(http.MethodGet, "/api/v1/videos/"+taskID+"/content?model="+modelName, nil)
	queryRequest = queryRequest.WithContext(service.WithUser(queryRequest.Context(), model.AuthUser{ID: user.ID, Role: model.UserRoleUser}))
	queryResponse := httptest.NewRecorder()

	AIVideoContent(queryResponse, queryRequest, taskID)

	if queryResponse.Code != http.StatusBadGateway {
		t.Fatalf("expected unbound task content to fail closed with HTTP 502, got %d: %s", queryResponse.Code, queryResponse.Body.String())
	}
	if fallbackHits != 0 {
		t.Fatalf("expected unbound task content not to hit fallback channel, got %d hits", fallbackHits)
	}
}

func TestAIVideoZeroCostCreateStillBindsTaskChannel(t *testing.T) {
	setupHandlerTestDB(t)
	const modelName = "grok-imagine-video"
	const taskID = "video_task_zero_cost_bound_channel"
	wrongUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "wrong channel", http.StatusNotFound)
	}))
	t.Cleanup(wrongUpstream.Close)
	correctUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/videos":
			_, _ = w.Write([]byte(`{"id":"` + taskID + `","status":"queued"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/videos/"+taskID:
			_, _ = w.Write([]byte(`{"id":"` + taskID + `","status":"completed"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(correctUpstream.Close)
	user := model.User{ID: "user_video_zero_cost_bound_channel", Username: "video-zero-cost-bound-channel", Role: model.UserRoleUser, Credits: 0, AffCode: "aff_video_zero_cost_bound_channel", Status: model.UserStatusActive, CreatedAt: "now", UpdatedAt: "now"}
	if _, err := repository.SaveUser(user); err != nil {
		t.Fatal(err)
	}
	_, err := repository.SaveSettings(model.Settings{
		Private: model.PrivateSetting{Channels: []model.ModelChannel{{Name: "correct", BaseURL: correctUpstream.URL, APIKey: "correct-key", Models: []string{modelName}, Enabled: true, Weight: 1}}},
	}, "now")
	if err != nil {
		t.Fatal(err)
	}

	createRequest := httptest.NewRequest(http.MethodPost, "/api/v1/videos", strings.NewReader(`{"model":"`+modelName+`"}`))
	createRequest.Header.Set("Content-Type", "application/json")
	createRequest = createRequest.WithContext(service.WithUser(createRequest.Context(), model.AuthUser{ID: user.ID, Role: model.UserRoleUser}))
	createResponse := httptest.NewRecorder()
	AIVideos(createResponse, createRequest)
	if createResponse.Code != http.StatusOK {
		t.Fatalf("create expected HTTP 200, got %d: %s", createResponse.Code, createResponse.Body.String())
	}
	_, err = repository.SaveSettings(model.Settings{
		Private: model.PrivateSetting{Channels: []model.ModelChannel{
			{Name: "wrong", BaseURL: wrongUpstream.URL, APIKey: "wrong-key", Models: []string{modelName}, Enabled: true, Weight: 1},
			{Name: "correct", BaseURL: correctUpstream.URL, APIKey: "correct-key", Models: []string{}, Enabled: true, Weight: 1},
		}},
	}, "now")
	if err != nil {
		t.Fatal(err)
	}

	queryRequest := httptest.NewRequest(http.MethodGet, "/api/v1/videos/"+taskID+"?model="+modelName, nil)
	queryRequest = queryRequest.WithContext(service.WithUser(queryRequest.Context(), model.AuthUser{ID: user.ID, Role: model.UserRoleUser}))
	queryResponse := httptest.NewRecorder()
	AIVideo(queryResponse, queryRequest, taskID)

	if queryResponse.Code != http.StatusOK {
		t.Fatalf("expected zero-cost task to use bound channel, got HTTP %d: %s", queryResponse.Code, queryResponse.Body.String())
	}
	if !strings.Contains(queryResponse.Body.String(), `"status":"completed"`) {
		t.Fatalf("expected zero-cost task response from correct channel, got %s", queryResponse.Body.String())
	}
}

type errorResponseWriter struct {
	header http.Header
}

func (writer errorResponseWriter) Header() http.Header {
	return writer.header
}

func (writer errorResponseWriter) WriteHeader(_ int) {}

func (writer errorResponseWriter) Write(_ []byte) (int, error) {
	return 0, errors.New("client write failed")
}

type notifyingResponseWriter struct {
	header http.Header
	wrote  chan []byte
}

func (writer notifyingResponseWriter) Header() http.Header {
	return writer.header
}

func (writer notifyingResponseWriter) WriteHeader(_ int) {}

func (writer notifyingResponseWriter) Write(body []byte) (int, error) {
	select {
	case writer.wrote <- append([]byte(nil), body...):
	default:
	}
	return len(body), nil
}
