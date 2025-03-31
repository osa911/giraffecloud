package handlers

import (
	"context"
	"io"
	"net"
	"net/http"
	"sync"
)

type HTTPHandler struct {
	localAddr string
	client    *http.Client
	mu        sync.Mutex
}

func NewHTTPHandler(localAddr string) *HTTPHandler {
	return &HTTPHandler{
		localAddr: localAddr,
		client: &http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					return net.Dial("tcp", localAddr)
				},
			},
		},
	}
}

func (h *HTTPHandler) HandleRequest(w http.ResponseWriter, r *http.Request) error {
	// Create a new request to the local service
	req, err := http.NewRequest(r.Method, r.URL.String(), r.Body)
	if err != nil {
		return err
	}

	// Copy headers
	for key, values := range r.Header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	// Send request to local service
	resp, err := h.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Copy response headers
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Set status code
	w.WriteHeader(resp.StatusCode)

	// Copy response body
	_, err = io.Copy(w, resp.Body)
	return err
}

func (h *HTTPHandler) Start() error {
	// Start HTTP server
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if err := h.HandleRequest(w, r); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	return http.ListenAndServe(h.localAddr, nil)
}