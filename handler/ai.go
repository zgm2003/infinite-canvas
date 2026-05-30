package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/basketikun/infinite-canvas/service"
)

const (
	maxAIResponseInspectBytes = 1 << 20
	maxAIRequestCount         = 15
)

func AIImagesGenerations(w http.ResponseWriter, r *http.Request) {
	proxyAIRequest(w, r, "/images/generations")
}

func AIImagesEdits(w http.ResponseWriter, r *http.Request) {
	proxyAIRequest(w, r, "/images/edits")
}

func AIChatCompletions(w http.ResponseWriter, r *http.Request) {
	proxyAIRequest(w, r, "/chat/completions")
}

func AIVideos(w http.ResponseWriter, r *http.Request) {
	proxyAIRequest(w, r, "/videos")
}

func AIVideo(w http.ResponseWriter, r *http.Request, id string) {
	proxyAIVideoGetRequest(w, r, id, "/videos/"+id, true)
}

func AIVideoContent(w http.ResponseWriter, r *http.Request, id string) {
	proxyAIVideoGetRequest(w, r, id, "/videos/"+id+"/content", false)
}

func proxyAIRequest(w http.ResponseWriter, r *http.Request, path string) {
	body, contentType, modelName, err := readAIRequest(r)
	if err != nil {
		log.Printf("AI proxy request read failed: %v", err)
		FailStatus(w, http.StatusBadRequest, "AI 接口请求失败")
		return
	}
	user, ok := service.UserFromContext(r.Context())
	if !ok {
		FailStatus(w, http.StatusUnauthorized, "未登录或权限不足")
		return
	}
	credits, err := service.ModelCost(modelName)
	if err != nil {
		log.Printf("AI proxy read model cost failed: model=%s err=%v", modelName, err)
		FailStatus(w, http.StatusBadGateway, "AI 接口请求失败")
		return
	}
	count := readAIRequestCount(body, contentType)
	totalCredits, creditOK := multiplyAICredits(credits, count)
	if !creditOK {
		log.Printf("AI proxy credit calculation overflow: model=%s credits=%d count=%d", modelName, credits, count)
		FailStatus(w, http.StatusBadRequest, "AI 接口请求失败")
		return
	}
	credits = totalCredits
	channel, err := service.SelectModelChannel(modelName)
	if err != nil {
		log.Printf("AI proxy select channel failed: model=%s err=%v", modelName, err)
		FailStatus(w, http.StatusBadGateway, "AI 接口请求失败")
		return
	}
	request, err := http.NewRequest(http.MethodPost, service.BuildModelChannelURL(channel, path), bytes.NewReader(body))
	if err != nil {
		log.Printf("AI proxy build request failed: url=%s err=%v", service.BuildModelChannelURL(channel, path), err)
		FailStatus(w, http.StatusBadGateway, "AI 接口请求失败")
		return
	}
	request.Header.Set("Authorization", "Bearer "+channel.APIKey)
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	consumeLog, err := service.ConsumeUserCredits(user.ID, modelName, credits, path)
	if err != nil {
		FailError(w, err)
		return
	}
	videoTaskID := ""
	videoTaskBound := false
	copyAIResponse(w, request, func() {
		if path == "/videos" && videoTaskID != "" && videoTaskBound {
			if err := service.RefundVideoTaskCredits(user.ID, modelName, videoTaskID); err != nil {
				log.Printf("AI proxy refund video task credits failed: user=%s model=%s task=%s err=%v", user.ID, modelName, videoTaskID, err)
			}
			return
		}
		if err := service.RefundUserCredits(user.ID, modelName, credits, path); err != nil {
			log.Printf("AI proxy refund credits failed: user=%s model=%s credits=%d err=%v", user.ID, modelName, credits, err)
		}
	}, func(body []byte) error {
		if path != "/videos" {
			return nil
		}
		taskID := readAIResponseID(body)
		if taskID == "" {
			return fmt.Errorf("video create response missing task id")
		}
		videoTaskID = taskID
		if err := service.BindVideoTaskCreditLog(consumeLog, taskID, channel); err != nil {
			return fmt.Errorf("bind video credit log: %w", err)
		}
		videoTaskBound = true
		return nil
	})
}

func proxyAIVideoGetRequest(w http.ResponseWriter, r *http.Request, id string, path string, refundOnFinalFailure bool) {
	modelName := r.URL.Query().Get("model")
	if strings.TrimSpace(modelName) == "" {
		modelName = "grok-imagine-video"
	}
	user, hasUser := service.UserFromContext(r.Context())
	if !hasUser {
		FailStatus(w, http.StatusUnauthorized, "未登录或权限不足")
		return
	}
	channel, ok, err := service.VideoTaskChannel(user.ID, id)
	if err != nil {
		log.Printf("AI proxy select bound video channel failed: user=%s task=%s model=%s err=%v", user.ID, id, modelName, err)
		FailStatus(w, http.StatusBadGateway, "AI 接口请求失败")
		return
	}
	if !ok {
		log.Printf("AI proxy video task has no bound channel: user=%s task=%s model=%s", user.ID, id, modelName)
		FailStatus(w, http.StatusBadGateway, "AI 接口请求失败")
		return
	}
	request, err := http.NewRequest(http.MethodGet, service.BuildModelChannelURL(channel, path), nil)
	if err != nil {
		log.Printf("AI proxy build bound video request failed: user=%s task=%s model=%s err=%v", user.ID, id, modelName, err)
		FailStatus(w, http.StatusBadGateway, "AI 接口请求失败")
		return
	}
	request.Header.Set("Authorization", "Bearer "+channel.APIKey)
	if !refundOnFinalFailure {
		copyAIResponse(w, request, nil)
		return
	}
	copyAIResponse(w, request, nil, func(body []byte) error {
		if !isAIUpstreamBusinessFailure(body) {
			return nil
		}
		if !hasUser {
			return nil
		}
		if err := service.RefundVideoTaskCredits(user.ID, modelName, id); err != nil {
			log.Printf("AI proxy refund failed video task credits failed: user=%s model=%s task=%s err=%v", user.ID, modelName, id, err)
		}
		return nil
	})
}

func copyAIResponse(w http.ResponseWriter, request *http.Request, onFailure func(), onSuccessJSON ...func([]byte) error) {
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		log.Printf("AI proxy request failed: url=%s err=%v", request.URL.String(), err)
		if onFailure != nil {
			onFailure()
		}
		FailStatus(w, http.StatusBadGateway, "AI 接口请求失败")
		return
	}
	defer response.Body.Close()

	if response.StatusCode >= http.StatusBadRequest {
		payload, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		log.Printf("AI upstream error: url=%s status=%d body=%s", request.URL.String(), response.StatusCode, strings.TrimSpace(string(payload)))
		if onFailure != nil {
			onFailure()
		}
		FailStatus(w, http.StatusBadGateway, "AI 接口请求失败")
		return
	}

	body := io.Reader(response.Body)
	mustValidateJSON := len(onSuccessJSON) > 0
	shouldInspectFailure := onFailure != nil && shouldInspectAIResponseBody(response.Header.Get("Content-Type"), response.ContentLength, isStreamingAIRequest(request))
	if mustValidateJSON || shouldInspectFailure {
		prefix, overflow, err := readAIResponseBodyForInspection(response.Body)
		if err != nil {
			log.Printf("AI proxy response read failed: url=%s err=%v", request.URL.String(), err)
			if onFailure != nil {
				onFailure()
			}
			FailStatus(w, http.StatusBadGateway, "AI 接口请求失败")
			return
		}
		if overflow && mustValidateJSON {
			log.Printf("AI proxy response too large to validate: url=%s bytes>%d", request.URL.String(), maxAIResponseInspectBytes)
			if onFailure != nil {
				onFailure()
			}
			FailStatus(w, http.StatusBadGateway, "AI 接口请求失败")
			return
		}
		if shouldInspectFailure && !overflow && isAIUpstreamBusinessFailure(prefix) {
			log.Printf("AI upstream business error: url=%s body=%s", request.URL.String(), strings.TrimSpace(string(prefix)))
			onFailure()
			FailStatus(w, http.StatusBadGateway, "AI 接口请求失败")
			return
		}
		if !overflow {
			for _, onSuccess := range onSuccessJSON {
				if onSuccess != nil {
					if err := onSuccess(prefix); err != nil {
						log.Printf("AI proxy response validation failed: url=%s err=%v", request.URL.String(), err)
						if onFailure != nil {
							onFailure()
						}
						FailStatus(w, http.StatusBadGateway, "AI 接口请求失败")
						return
					}
				}
			}
		}
		body = io.MultiReader(bytes.NewReader(prefix), response.Body)
	}

	for key, values := range response.Header {
		if strings.EqualFold(key, "Content-Length") {
			continue
		}
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(response.StatusCode)
	if written, err := io.Copy(w, body); err != nil {
		log.Printf("AI proxy response copy failed: url=%s bytes=%d err=%v", request.URL.String(), written, err)
		if onFailure != nil {
			onFailure()
		}
	}
}

func readAIResponseBodyForInspection(body io.Reader) ([]byte, bool, error) {
	payload, err := io.ReadAll(io.LimitReader(body, maxAIResponseInspectBytes+1))
	if err != nil {
		return nil, false, err
	}
	return payload, len(payload) > maxAIResponseInspectBytes, nil
}

func readAIResponseID(body []byte) string {
	var payload struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(body, &payload)
	return strings.TrimSpace(payload.ID)
}

func isJSONResponse(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		mediaType = contentType
	}
	return strings.Contains(strings.ToLower(mediaType), "json")
}

func shouldInspectAIResponseBody(contentType string, contentLength int64, streamingRequest bool) bool {
	if isJSONResponse(contentType) {
		if streamingRequest {
			return contentLength >= 0 && contentLength <= maxAIResponseInspectBytes
		}
		return true
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		mediaType = contentType
	}
	mediaType = strings.ToLower(strings.TrimSpace(mediaType))
	if streamingRequest {
		return contentLength >= 0 && contentLength <= maxAIResponseInspectBytes && (mediaType == "" || mediaType == "text/plain")
	}
	return mediaType == "" || mediaType == "text/plain"
}

func isStreamingAIRequest(request *http.Request) bool {
	if request == nil || request.GetBody == nil {
		return false
	}
	body, err := request.GetBody()
	if err != nil {
		return false
	}
	defer body.Close()
	payload, err := io.ReadAll(body)
	if err != nil {
		return false
	}
	var requestBody struct {
		Stream bool `json:"stream"`
	}
	_ = json.Unmarshal(payload, &requestBody)
	return requestBody.Stream
}

func isAIUpstreamBusinessFailure(body []byte) bool {
	body = bytes.TrimSpace(body)
	if len(body) == 0 || !json.Valid(body) {
		return false
	}
	var payload struct {
		Error   json.RawMessage `json:"error"`
		Code    json.RawMessage `json:"code"`
		Status  string          `json:"status"`
		Success *bool           `json:"success"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return false
	}
	if aiResponseCodeFailed(payload.Code) {
		return true
	}
	if payload.Success != nil && !*payload.Success {
		return true
	}
	if len(payload.Error) > 0 {
		errorValue := bytes.TrimSpace(payload.Error)
		if len(errorValue) > 0 &&
			!bytes.Equal(errorValue, []byte("null")) &&
			!bytes.Equal(errorValue, []byte("false")) &&
			!bytes.Equal(errorValue, []byte(`""`)) {
			return true
		}
	}
	switch strings.ToLower(strings.TrimSpace(payload.Status)) {
	case "error", "failed", "cancelled", "canceled":
		return true
	default:
		return false
	}
}

func aiResponseCodeFailed(raw json.RawMessage) bool {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return false
	}
	var numeric float64
	if err := json.Unmarshal(raw, &numeric); err == nil {
		return numeric != 0
	}
	var text string
	if err := json.Unmarshal(raw, &text); err != nil {
		return false
	}
	text = strings.ToLower(strings.TrimSpace(text))
	return text != "" && text != "0" && text != "ok" && text != "success"
}

func readAIRequest(r *http.Request) ([]byte, string, string, error) {
	contentType := r.Header.Get("Content-Type")
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, "", "", err
	}
	modelName := ""
	if strings.HasPrefix(contentType, "multipart/form-data") {
		modelName = readMultipartModel(body, contentType)
	} else {
		var payload struct {
			Model string `json:"model"`
		}
		_ = json.Unmarshal(body, &payload)
		modelName = payload.Model
	}
	if strings.TrimSpace(modelName) == "" {
		return nil, "", "", errMissingModel
	}
	return body, contentType, modelName, nil
}

func readMultipartModel(body []byte, contentType string) string {
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return ""
	}
	reader := multipart.NewReader(bytes.NewReader(body), params["boundary"])
	form, err := reader.ReadForm(32 << 20)
	if err != nil {
		return ""
	}
	defer form.RemoveAll()
	if values := form.Value["model"]; len(values) > 0 {
		return values[0]
	}
	return ""
}

func readAIRequestCount(body []byte, contentType string) int {
	count := 1
	if strings.HasPrefix(contentType, "multipart/form-data") {
		_, params, err := mime.ParseMediaType(contentType)
		if err != nil {
			return count
		}
		form, err := multipart.NewReader(bytes.NewReader(body), params["boundary"]).ReadForm(32 << 20)
		if err != nil {
			return count
		}
		defer form.RemoveAll()
		if values := form.Value["n"]; len(values) > 0 {
			_, _ = fmt.Sscan(values[0], &count)
		}
	} else {
		var payload struct {
			N int `json:"n"`
		}
		_ = json.Unmarshal(body, &payload)
		count = payload.N
	}
	if count < 1 {
		return 1
	}
	if count > maxAIRequestCount {
		return maxAIRequestCount
	}
	return count
}

func multiplyAICredits(credits int, count int) (int, bool) {
	if credits <= 0 || count <= 0 {
		return credits, true
	}
	maxInt := int(^uint(0) >> 1)
	if count > maxInt/credits {
		return 0, false
	}
	return credits * count, true
}

var errMissingModel = &aiError{"缺少模型名称"}

type aiError struct {
	message string
}

func (err *aiError) Error() string {
	return err.message
}
