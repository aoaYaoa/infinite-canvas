package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"github.com/tigerowo/infinite-canvas/model"
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

func TestNormalizeVideoCreateBodyConvertsGrok2APIMultipartToJSON(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("model", "grok-imagine-video")
	_ = writer.WriteField("prompt", "slow camera push")
	_ = writer.WriteField("seconds", "6")
	_ = writer.WriteField("size", "720x1280")
	_ = writer.WriteField("resolution_name", "480p")
	file, err := writer.CreateFormFile("input_reference[]", "input.png")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = file.Write([]byte("\x89PNG\r\n\x1a\nimage"))
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	converted, contentType, err := normalizeVideoCreateBody(body.Bytes(), writer.FormDataContentType(), "grok-imagine-video", model.ModelChannel{
		Name:    "Grok2API",
		BaseURL: "https://grok.uonoe.com/v1",
	}, "/videos")
	if err != nil {
		t.Fatal(err)
	}
	if contentType != "application/json" {
		t.Fatalf("contentType = %q", contentType)
	}
	var payload struct {
		Model          string `json:"model"`
		Prompt         string `json:"prompt"`
		Seconds        int    `json:"seconds"`
		Size           string `json:"size"`
		Quality        string `json:"quality"`
		ImageReference *struct {
			ImageURL string `json:"image_url"`
		} `json:"image_reference"`
	}
	if err := json.Unmarshal(converted, &payload); err != nil {
		t.Fatalf("json = %s err=%v", string(converted), err)
	}
	if payload.Model != "grok-imagine-video" || payload.Prompt != "slow camera push" || payload.Seconds != 6 || payload.Size != "720x1280" || payload.Quality != "480p" {
		t.Fatalf("payload = %#v", payload)
	}
	if payload.ImageReference == nil || !strings.HasPrefix(payload.ImageReference.ImageURL, "data:image/png;base64,") {
		t.Fatalf("image_reference = %#v", payload.ImageReference)
	}
	if strings.Contains(string(converted), "resolution_name") || strings.Contains(string(converted), "input_reference") {
		t.Fatalf("unsupported fields leaked into upstream payload: %s", string(converted))
	}
}

func TestHasVideoCreateResultAcceptsSynchronousVideoURL(t *testing.T) {
	parsed := parseVideoTaskPayload([]byte(`{"url":"https://cdn.example.com/generated.mp4","video_url":"https://cdn.example.com/generated.mp4"}`), "grok-imagine-video")

	if !hasVideoCreateResult(parsed) {
		t.Fatalf("expected synchronous video URL response to be accepted: %#v", parsed)
	}
}

func TestTransformVideoCreatePayloadAbsolutizesGrok2APIRelativeVideoURL(t *testing.T) {
	request, err := http.NewRequest(http.MethodPost, "https://grok.uonoe.com/v1/videos", nil)
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte(`{"request_id":"video_123","url":"/v1/files/video/generated.mp4","video_url":"/v1/files/video/generated.mp4"}`)

	transformed := transformVideoCreatePayload(payload, request, model.ModelChannel{
		Name:    "Grok2API",
		BaseURL: "https://grok.uonoe.com/v1",
	}, "grok-imagine-video")
	parsed := parseVideoTaskPayload(transformed, "grok-imagine-video")

	if parsed.VideoURL != "https://grok.uonoe.com/v1/files/video/generated.mp4" {
		t.Fatalf("video URL = %q, payload=%s", parsed.VideoURL, string(transformed))
	}
	if strings.Contains(string(transformed), `"/v1/files/video/generated.mp4"`) {
		t.Fatalf("relative URL leaked into transformed payload: %s", string(transformed))
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

func TestShouldRetryCanvasImageTaskFailureForWrappedIncompleteGrok2APIEdit(t *testing.T) {
	payload := []byte(`{"code":1,"data":null,"msg":"上游未返回可用的编辑图片"}`)

	if !shouldRetryCanvasImageTaskFailure(200, payload, errors.New("上游未返回可用的编辑图片")) {
		t.Fatal("expected wrapped incomplete Grok2API image edit to be retried")
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
