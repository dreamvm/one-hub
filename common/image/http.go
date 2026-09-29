package image

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"

	"one-api/common/config"
	"one-api/common/utils"
)

var ImageHttpClients = &http.Client{
	Transport: newMediaTransport(),
	Timeout:   15 * time.Second,
}

// Workers are administrator-configured delegates. Their own outbound policy
// must be enforced by the Worker; the gateway never follows Worker redirects.
var workerImageClient = &http.Client{
	Transport: &http.Transport{
		DialContext: utils.Socks5ProxyFunc,
		Proxy:       utils.ProxyFunc,
	},
	Timeout:       15 * time.Second,
	CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
}

var maxFileSize int64 = 20 * 1024 * 1024 // 20MB

type CFRequest struct {
	Action string `json:"action"`
	APIKey string `json:"api_key"`
	URL    string `json:"url"`
}

type CFResponse struct {
	Status   bool   `json:"status"`
	Message  string `json:"message,omitempty"`
	Data     string `json:"data,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
}

func RequestFile(url, action string) (*http.Response, error) {
	return requestMediaFile(context.Background(), url, action, config.CFWorkerImageUrl, config.ChatImageRequestProxy)
}

// RequestPublicFile returns a bounded raw media response with the shared destination
// policy. Worker actions have a different response contract and are not used.
func RequestPublicFile(ctx context.Context, url string) (*http.Response, error) {
	proxyAddress := config.ChatImageRequestProxy
	if proxyAddress == "" {
		// Evaluate environment rules for each redirect URL in the transport.
		ctx = context.WithValue(ctx, mediaEnvironmentProxyKey{}, true)
	}
	return requestMediaFile(ctx, url, "", "", proxyAddress)
}

func requestMediaFile(parent context.Context, url, action, workerURL, proxyAddress string) (*http.Response, error) {
	reqUrl := url
	method := http.MethodGet
	var requestBody any
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	client := ImageHttpClients

	if workerURL != "" {
		if err := validateWorkerMediaTarget(ctx, url); err != nil {
			cancel()
			return nil, err
		}
		client = workerImageClient
		requestBody = &CFRequest{
			Action: action,
			APIKey: config.CFWorkerImageKey,
			URL:    url,
		}
		reqUrl = workerURL
		method = http.MethodPost
	}

	res, err := utils.RequestBuilder(utils.SetProxy(proxyAddress, ctx), method, reqUrl, requestBody, nil)

	if err != nil {
		cancel()
		return nil, errMediaTarget
	}

	response, err := client.Do(res)
	if err != nil {
		cancel()
		// Client errors include the original URL, including signed query data.
		if errors.Is(err, errMediaTarget) {
			return nil, errMediaTarget
		}
		return nil, errors.New("media download failed")
	}

	response.Body = &mediaResponseBody{ReadCloser: http.MaxBytesReader(nil, response.Body, maxFileSize), cancel: cancel}

	if response.StatusCode != http.StatusOK && workerURL != "" {
		defer response.Body.Close()
		var cfResp CFResponse
		err = json.NewDecoder(response.Body).Decode(&cfResp)
		if err != nil {
			return nil, err
		}
		return nil, errors.New(cfResp.Message)
	}

	return response, err
}

func validateWorkerMediaTarget(ctx context.Context, rawURL string) error {
	target, err := url.Parse(rawURL)
	if err != nil {
		return errMediaTarget
	}
	_, err = mediaAddresses(ctx, target, net.DefaultResolver.LookupIPAddr)
	return err
}

type mediaResponseBody struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (b *mediaResponseBody) Close() error {
	b.cancel()
	return b.ReadCloser.Close()
}
