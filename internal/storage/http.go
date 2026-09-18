package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/kalw/spm/internal/config"
)

// httpStore publishes to either a local directory (served by some web server)
// or an authenticated WebDAV/PUT endpoint. Consumers always download over the
// remote's download_base_url; this type only handles the write side.
type httpStore struct {
	mode    string // config.HTTPPublishLocal | config.HTTPPublishWebDAV
	dir     string // local mode
	baseURL string // webdav mode
	client  *http.Client
	user    string
	pass    string
	token   string
}

func newHTTP(r config.Remote) (Storage, error) {
	return &httpStore{
		mode:    r.Publish,
		dir:     r.PublishDir,
		baseURL: strings.TrimRight(r.PublishURL, "/"),
		client:  http.DefaultClient,
		user:    os.Getenv("SPM_HTTP_USER"),
		pass:    os.Getenv("SPM_HTTP_PASS"),
		token:   os.Getenv("SPM_HTTP_TOKEN"),
	}, nil
}

func (h *httpStore) Put(ctx context.Context, key string, data []byte, contentType string) error {
	if h.mode == config.HTTPPublishLocal {
		full := filepath.Join(h.dir, filepath.FromSlash(key))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		return os.WriteFile(full, data, 0o644)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, h.url(key), bytes.NewReader(data))
	if err != nil {
		return err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	h.auth(req)
	resp, err := h.client.Do(req)
	if err != nil {
		return fmt.Errorf("http put %s: %w", key, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("http put %s: unexpected status %s", key, resp.Status)
	}
	return nil
}

func (h *httpStore) Get(ctx context.Context, key string) ([]byte, bool, error) {
	if h.mode == config.HTTPPublishLocal {
		data, err := os.ReadFile(filepath.Join(h.dir, filepath.FromSlash(key)))
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		if err != nil {
			return nil, false, err
		}
		return data, true, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.url(key), nil)
	if err != nil {
		return nil, false, err
	}
	h.auth(req)
	resp, err := h.client.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("http get %s: %w", key, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, false, nil
	}
	if resp.StatusCode >= 300 {
		return nil, false, fmt.Errorf("http get %s: unexpected status %s", key, resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false, err
	}
	return data, true, nil
}

func (h *httpStore) Exists(ctx context.Context, key string) (bool, error) {
	if h.mode == config.HTTPPublishLocal {
		_, err := os.Stat(filepath.Join(h.dir, filepath.FromSlash(key)))
		if os.IsNotExist(err) {
			return false, nil
		}
		return err == nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, h.url(key), nil)
	if err != nil {
		return false, err
	}
	h.auth(req)
	resp, err := h.client.Do(req)
	if err != nil {
		return false, fmt.Errorf("http head %s: %w", key, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if resp.StatusCode >= 300 {
		return false, fmt.Errorf("http head %s: unexpected status %s", key, resp.Status)
	}
	return true, nil
}

func (h *httpStore) url(key string) string { return h.baseURL + "/" + key }

func (h *httpStore) auth(req *http.Request) {
	switch {
	case h.token != "":
		req.Header.Set("Authorization", "Bearer "+h.token)
	case h.user != "":
		req.SetBasicAuth(h.user, h.pass)
	}
}
