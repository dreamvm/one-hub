package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"one-api/.github/smoke/mockserver"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "request" {
		probe()
		return
	}
	handler := mockserver.New()
	server := func(address string) *http.Server {
		return &http.Server{Addr: address, Handler: handler, ReadHeaderTimeout: 5 * time.Second, WriteTimeout: 15 * time.Second}
	}
	go func() { log.Fatal(server(":8443").ListenAndServeTLS("/fixture/server.crt", "/fixture/server.key")) }()
	log.Fatal(server(":8000").ListenAndServe())
}

// probe transports the runner's synthetic HTTP checks inside the internal network.
// It does not accept external targets and never receives real credentials.
func probe() {
	var input struct {
		URL     string            `json:"url"`
		Method  string            `json:"method"`
		Headers map[string]string `json:"headers"`
		Body    string            `json:"body"`
	}
	fail := func(err error) { json.NewEncoder(os.Stdout).Encode(map[string]string{"error": err.Error()}) }
	if err := json.NewDecoder(io.LimitReader(os.Stdin, 1<<20)).Decode(&input); err != nil {
		fail(err)
		return
	}
	target, err := url.Parse(input.URL)
	if err != nil || target.Scheme != "http" || (target.Host != "gateway:3000" && target.Host != "mock-provider:8000") {
		fail(fmt.Errorf("only isolated fixture hosts are allowed"))
		return
	}
	request, err := http.NewRequest(input.Method, input.URL, strings.NewReader(input.Body))
	if err != nil {
		fail(err)
		return
	}
	for key, value := range input.Headers {
		request.Header.Set(key, value)
	}
	client := &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(request)
	if err != nil {
		fail(err)
		return
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	if err != nil {
		fail(err)
		return
	}
	json.NewEncoder(os.Stdout).Encode(map[string]any{"status": response.StatusCode, "headers": response.Header, "body": string(body)})
}
