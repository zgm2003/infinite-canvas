package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

type statusSafeError struct {
	message string
	status  int
}

func (err statusSafeError) Error() string {
	return err.message
}

func (err statusSafeError) SafeMessage() string {
	return err.message
}

func (err statusSafeError) StatusCode() int {
	return err.status
}

func TestFailErrorUsesTypedHTTPStatus(t *testing.T) {
	response := httptest.NewRecorder()

	FailError(response, statusSafeError{message: "用户名或密码错误", status: http.StatusUnauthorized})

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
	if payload.Code != 1 || payload.Msg != "用户名或密码错误" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestFailErrorUsesWrappedTypedHTTPStatus(t *testing.T) {
	response := httptest.NewRecorder()

	FailError(response, fmt.Errorf("login failed: %w", statusSafeError{message: "用户名或密码错误", status: http.StatusUnauthorized}))

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
	if payload.Code != 1 || payload.Msg != "用户名或密码错误" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}
