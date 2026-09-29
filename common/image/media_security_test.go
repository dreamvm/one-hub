package image_test

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"one-api/common/config"
	img "one-api/common/image"
)

const mediaPixel = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII="

func TestMediaRejectsLoopbackThroughBothEntrypoints(t *testing.T) {
	oldWorker, oldProxy := config.CFWorkerImageUrl, config.ChatImageRequestProxy
	config.CFWorkerImageUrl, config.ChatImageRequestProxy = "", ""
	t.Cleanup(func() { config.CFWorkerImageUrl, config.ChatImageRequestProxy = oldWorker, oldProxy })
	pixel, err := base64.StdEncoding.DecodeString(mediaPixel)
	if err != nil {
		t.Fatal(err)
	}
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pixel)
	}))
	defer server.Close()
	if _, _, err := img.GetImageFromUrl(server.URL + "/private.png"); err == nil {
		t.Error("full download accepted a loopback target")
	}
	if _, _, err := img.GetImageSizeFromUrl(server.URL + "/private.png"); err == nil {
		t.Error("dimension lookup accepted a loopback target")
	}
	if got := hits.Load(); got != 0 {
		t.Errorf("restricted target received %d requests", got)
	}
	inline := "data:image/png;base64," + mediaPixel
	if mime, data, err := img.GetImageFromUrl(inline); err != nil || mime != "image/png" || data != mediaPixel {
		t.Fatalf("inline media changed: mime=%q err=%v", mime, err)
	}
	if w, h, err := img.GetImageSize(inline); err != nil || w != 1 || h != 1 {
		t.Fatalf("inline dimensions changed: %dx%d err=%v", w, h, err)
	}
}
