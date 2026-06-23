package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
)

// API is a thin REST client for the dfs CLI.
type API struct {
	baseURL string
	apiKey  string
	token   string
	client  *http.Client
}

// NewAPI builds a client with auth headers.
func NewAPI(baseURL, apiKey, token string) *API {
	return &API{
		baseURL: baseURL,
		apiKey:  apiKey,
		token:   token,
		client:  &http.Client{},
	}
}

func (a *API) setAuth(req *http.Request) {
	if a.token != "" {
		req.Header.Set("Authorization", "Bearer "+a.token)
	} else if a.apiKey != "" {
		req.Header.Set("X-API-Key", a.apiKey)
	}
}

// Upload sends a local file via multipart upload.
func (a *API) Upload(path string, progress io.Writer) (*UploadResponse, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, err := w.CreateFormFile("file", filepath.Base(path))
	if err != nil {
		return nil, err
	}

	reader := io.Reader(f)
	if progress != nil {
		reader = io.TeeReader(f, progress)
	}

	if _, err := io.Copy(part, reader); err != nil {
		return nil, err
	}
	_ = w.Close()

	req, err := http.NewRequest(http.MethodPost, a.baseURL+"/api/v1/upload", &body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	a.setAuth(req)

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("upload failed: %s", readBody(resp.Body))
	}

	var out UploadResponse
	return &out, json.NewDecoder(resp.Body).Decode(&out)
}

// Download fetches a file to output path.
func (a *API) Download(fileID, outputDir string, progress io.Writer) error {
	req, err := http.NewRequest(http.MethodGet, a.baseURL+"/api/v1/files/"+fileID+"/download", nil)
	if err != nil {
		return err
	}
	a.setAuth(req)

	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("download failed: %s", readBody(resp.Body))
	}

	outPath := filepath.Join(outputDir, fileID)
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	reader := io.Reader(resp.Body)
	if progress != nil {
		reader = io.TeeReader(resp.Body, progress)
	}
	_, err = io.Copy(f, reader)
	return err
}

// Delete removes a file by id.
func (a *API) Delete(fileID string) error {
	req, err := http.NewRequest(http.MethodDelete, a.baseURL+"/api/v1/files/"+fileID, nil)
	if err != nil {
		return err
	}
	a.setAuth(req)
	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("delete failed: %s", readBody(resp.Body))
	}
	return nil
}

// List returns paginated files.
func (a *API) List(page, limit int) (*ListResponse, error) {
	url := fmt.Sprintf("%s/api/v1/files?page=%d&limit=%d", a.baseURL, page, limit)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	a.setAuth(req)
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	var out ListResponse
	return &out, json.NewDecoder(resp.Body).Decode(&out)
}

// Status returns cluster status.
func (a *API) Status() (*StatusResponse, error) {
	req, err := http.NewRequest(http.MethodGet, a.baseURL+"/api/v1/cluster/status", nil)
	if err != nil {
		return nil, err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	var out StatusResponse
	return &out, json.NewDecoder(resp.Body).Decode(&out)
}

// UploadResponse mirrors master upload json.
type UploadResponse struct {
	FileID   string `json:"file_id"`
	FileName string `json:"file_name"`
	Size     int64  `json:"size"`
	Chunks   int    `json:"chunks"`
}

// ListResponse is paginated file list.
type ListResponse struct {
	Files []any `json:"files"`
	Total int   `json:"total"`
	Page  int   `json:"page"`
}

// StatusResponse is cluster status payload.
type StatusResponse struct {
	Nodes              []map[string]any `json:"nodes"`
	TotalStorage       int64            `json:"total_storage"`
	UsedStorage        int64            `json:"used_storage"`
	ReplicationFactor  int              `json:"replication_factor"`
	AliveNodes         int              `json:"alive_nodes"`
}

func readBody(r io.Reader) string {
	b, _ := io.ReadAll(r)
	return string(b)
}
