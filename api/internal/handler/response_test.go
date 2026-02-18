package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWriteJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	data := map[string]string{"name": "test", "value": "42"}
	writeJSON(rec, http.StatusOK, data)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type=application/json, got %s", ct)
	}

	var result map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if result["name"] != "test" || result["value"] != "42" {
		t.Errorf("unexpected body: %v", result)
	}
}

func TestWriteJSONWithStatus(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, http.StatusCreated, map[string]int{"id": 1})
	if rec.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", rec.Code)
	}
}

func TestWriteError(t *testing.T) {
	rec := httptest.NewRecorder()
	writeError(rec, http.StatusBadRequest, "missing field")

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}

	var result map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if result["error"] != "missing field" {
		t.Errorf("expected error=missing field, got %s", result["error"])
	}
}

func TestDecodeJSON(t *testing.T) {
	body := `{"name":"alice","age":30}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))

	var data struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}
	if err := decodeJSON(req, &data); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if data.Name != "alice" {
		t.Errorf("expected name=alice, got %s", data.Name)
	}
	if data.Age != 30 {
		t.Errorf("expected age=30, got %d", data.Age)
	}
}

func TestDecodeJSONInvalidBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("not json"))
	var data struct{}
	if err := decodeJSON(req, &data); err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestDecodeJSONEmptyBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(""))
	var data struct{}
	if err := decodeJSON(req, &data); err == nil {
		t.Error("expected error for empty body")
	}
}

func TestWriteJSONSlice(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, http.StatusOK, []string{"a", "b", "c"})

	var result []string
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(result) != 3 {
		t.Errorf("expected 3 elements, got %d", len(result))
	}
}

func TestWriteJSONEmptySlice(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, http.StatusOK, []struct{}{})

	body := strings.TrimSpace(rec.Body.String())
	if body != "[]" {
		t.Errorf("expected [], got %s", body)
	}
}
