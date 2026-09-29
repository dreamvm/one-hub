package image_test

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"one-api/common/config"
	img "one-api/common/image"
)

func TestMediaWorkerContractAndTargetValidation(t *testing.T) {
	var hits atomic.Int32
	worker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		var payload img.CFRequest
		if r.Method != http.MethodPost || json.NewDecoder(r.Body).Decode(&payload) != nil {
			t.Error("invalid Worker request")
			return
		}
		if payload.URL != "https://8.8.8.8/pixel?sig=fixture" || payload.APIKey != "fixture-key" {
			t.Error("Worker payload changed")
		}
		switch payload.Action {
		case "base64":
			_ = json.NewEncoder(w).Encode(img.CFResponse{Status: true, Data: mediaPixel, MimeType: "image/png"})
		case "get16kb":
			pixel, _ := base64.StdEncoding.DecodeString(mediaPixel)
			_, _ = w.Write(pixel)
		default:
			t.Error("Worker action changed")
		}
	}))
	defer worker.Close()
	setMediaFixture(t, img.ImageHttpClients, "")
	config.CFWorkerImageUrl, config.CFWorkerImageKey = worker.URL, "fixture-key"
	mime, data, err := img.GetImageFromUrl("https://8.8.8.8/pixel?sig=fixture")
	if err != nil || mime != "image/png" || data != mediaPixel {
		t.Fatalf("Worker base64 changed: %s %v", mime, err)
	}
	if w, h, err := img.GetImageSizeFromUrl("https://8.8.8.8/pixel?sig=fixture"); err != nil || w != 1 || h != 1 {
		t.Fatalf("Worker dimensions changed: %dx%d %v", w, h, err)
	}
	if _, _, err := img.GetImageFromUrl("http://127.0.0.1/private"); err == nil {
		t.Fatal("Worker accepted restricted target")
	}
	if hits.Load() != 2 {
		t.Errorf("Worker received restricted input: %d", hits.Load())
	}
}

func TestMediaWorkerRedirectAndErrorBody(t *testing.T) {
	var redirected atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { redirected.Add(1) }))
	defer target.Close()
	worker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer worker.Close()
	setMediaFixture(t, img.ImageHttpClients, "")
	config.CFWorkerImageUrl = worker.URL
	if _, err := img.RequestFile("https://8.8.8.8/pixel", "base64"); err == nil {
		t.Fatal("Worker redirect accepted")
	}
	if redirected.Load() != 0 {
		t.Fatal("Worker redirected API key to another endpoint")
	}
	for _, response := range []string{`{"message":"fixture failure"}`, `null`, `invalid`} {
		body := &mediaTrackedBody{Reader: strings.NewReader(response)}
		restore := img.SetWorkerClientForTest(&http.Client{Transport: mediaRoundTrip(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusBadGateway, Body: body, Header: make(http.Header)}, nil
		})})
		_, err := img.RequestFile("https://8.8.8.8/pixel", "base64")
		restore()
		if err == nil || !body.closed {
			t.Fatalf("Worker error body not closed: %v closed=%v", err, body.closed)
		}
	}
}

type mediaRoundTrip func(*http.Request) (*http.Response, error)

func (f mediaRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type mediaTrackedBody struct {
	io.Reader
	closed bool
}

func (b *mediaTrackedBody) Close() error { b.closed = true; return nil }
