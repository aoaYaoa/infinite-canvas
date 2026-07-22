package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"mime/multipart"
	"strings"
	"testing"
)

func TestReadUpstreamAIErrorMessage(t *testing.T) {
	got := readUpstreamAIErrorMessage([]byte(`{"error":{"code":"InvalidParameter","message":"reference video fps is invalid"}}`), 400)
	if got != "reference video fps is invalid" {
		t.Fatalf("message = %q", got)
	}
}

func TestNormalizeGrok2APIImageEditBodyConvertsMultipartToJSON(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("_canvas_endpoint", "/images/edits")
	_ = writer.WriteField("model", "grok-imagine-image-quality")
	_ = writer.WriteField("prompt", "make it brighter")
	_ = writer.WriteField("n", "1")
	_ = writer.WriteField("size", "1024x1024")
	_ = writer.WriteField("response_format", "b64_json")
	file, err := writer.CreateFormFile("image", "input.png")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = file.Write([]byte("\x89PNG\r\n\x1a\nimage"))
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	converted, contentType, err := normalizeGrok2APIImageEditBody(body.Bytes(), writer.FormDataContentType())
	if err != nil {
		t.Fatal(err)
	}
	if contentType != "application/json" {
		t.Fatalf("contentType = %q", contentType)
	}
	var payload struct {
		Model          string `json:"model"`
		Prompt         string `json:"prompt"`
		Count          int    `json:"n"`
		Size           string `json:"size"`
		ResponseFormat string `json:"response_format"`
		Images         []struct {
			URL string `json:"url"`
		} `json:"images"`
	}
	if err := json.Unmarshal(converted, &payload); err != nil {
		t.Fatalf("json = %s err=%v", string(converted), err)
	}
	if payload.Model != "grok-imagine-image-quality" || payload.Prompt != "make it brighter" || payload.Count != 1 || payload.Size != "1024x1024" || payload.ResponseFormat != "b64_json" {
		t.Fatalf("payload = %#v", payload)
	}
	if len(payload.Images) != 1 || !strings.HasPrefix(payload.Images[0].URL, "data:image/png;base64,") {
		t.Fatalf("images = %#v", payload.Images)
	}
	if strings.Contains(string(converted), "_canvas_endpoint") {
		t.Fatalf("canvas metadata leaked into upstream payload: %s", string(converted))
	}
}

func TestImageURLFromAIResponseReadsImageEditSSE(t *testing.T) {
	payload := []byte("event: image_edit.completed\n" +
		"data: {\"type\":\"image_edit.completed\",\"b64_json\":\"iVBORw0KGgo=\",\"size\":\"1024x1024\"}\n\n" +
		"data: [DONE]\n\n")

	url, mimeType, size, err := imageURLFromAIResponse(payload)
	if err != nil {
		t.Fatal(err)
	}
	if url != "data:image/png;base64,iVBORw0KGgo=" || mimeType != "image/png" || size != 8 {
		t.Fatalf("url=%q mime=%q size=%d", url, mimeType, size)
	}
}

func TestShouldRetryCanvasImageTaskFailureForIncompleteGrok2APIEdit(t *testing.T) {
	payload := []byte(`{"error":{"code":"image_edit_incomplete","message":"上游未返回可用的编辑图片","type":"server_error"}}`)

	if !shouldRetryCanvasImageTaskFailure(502, payload, nil) {
		t.Fatal("expected incomplete Grok2API image edit to be retried")
	}
}

func TestShouldRetryCanvasImageTaskFailureForEmptySuccessfulResponse(t *testing.T) {
	if !shouldRetryCanvasImageTaskFailure(200, nil, errors.New("unexpected end of JSON input")) {
		t.Fatal("expected empty successful image response to be retried")
	}
}

func TestShouldRetryCanvasImageTaskFailureKeepsQuotaErrorsFinal(t *testing.T) {
	payload := []byte(`{"error":{"code":"upstream_unavailable","message":"上游账号额度等待恢复","type":"server_error"}}`)

	if shouldRetryCanvasImageTaskFailure(429, payload, nil) {
		t.Fatal("quota errors should not be retried by canvas")
	}
}
