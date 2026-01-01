package infra

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"

	"github.com/tnqbao/gau-cloud-orchestrator/config"
)

type UploadService struct {
	UploadServiceURL string `json:"upload_service_url"`
	CDNServiceURL    string `json:"cdn_service_url"`
	PrivateKey       string `json:"private_key,omitempty"`
}

func InitUploadService(config *config.EnvConfig) *UploadService {
	if config.ExternalService.UploadServiceURL == "" {
		panic("Upload service URL is not configured")
	}

	if config.PrivateKey == "" {
		panic("Private key is not configured")
	}

	return &UploadService{
		UploadServiceURL: config.ExternalService.UploadServiceURL,
		CDNServiceURL:    config.ExternalService.CDNServiceURL,
		PrivateKey:       config.PrivateKey,
	}
}

// UploadResponse represents the response from upload service
type UploadResponse struct {
	Bucket      string `json:"bucket"`
	ContentType string `json:"content_type"`
	Duplicated  bool   `json:"duplicated"`
	FileHash    string `json:"file_hash"`
	FilePath    string `json:"file_path"`
	Message     string `json:"message"`
	Size        int64  `json:"size"`
	Status      int    `json:"status"`
}

// GetCDNURL returns the full CDN URL for a file
func (p *UploadService) GetCDNURL(bucket string, filePath string) string {
	return fmt.Sprintf("%s/%s/%s", p.CDNServiceURL, bucket, filePath)
}

func (p *UploadService) UploadFile(
	file multipart.File,
	filename string,
	contentType string,
	bucket string,
	path string,
) (*UploadResponse, error) {

	url := fmt.Sprintf("%s/api/v2/upload/file", p.UploadServiceURL)

	var b bytes.Buffer
	w := multipart.NewWriter(&b)

	if err := w.WriteField("bucket", bucket); err != nil {
		return nil, fmt.Errorf("failed to write bucket field: %w", err)
	}

	if err := w.WriteField("path", path); err != nil {
		return nil, fmt.Errorf("failed to write path field: %w", err)
	}

	h := make(map[string][]string)
	h["Content-Disposition"] = []string{
		fmt.Sprintf(`form-data; name="file"; filename="%s"`, filename),
	}
	h["Content-Type"] = []string{contentType}

	fw, err := w.CreatePart(h)
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}

	if _, err := io.Copy(fw, file); err != nil {
		return nil, fmt.Errorf("failed to write file data: %w", err)
	}

	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, &b)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Private-Key", p.PrivateKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to upload file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("upload service returned %d: %s", resp.StatusCode, raw)
	}

	var response UploadResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}
