package infra

import (
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"

	"github.com/tnqbao/gau-cloud-service/config"
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
	isHash bool,
) (*UploadResponse, error) {
	return p.uploadFileInternal(file, filename, contentType, bucket, path, isHash)
}

// UploadChunkToService uploads a chunk to the upload service without hashing the filename
// This is used for chunked uploads where we want to preserve the original chunk name
func (p *UploadService) UploadChunkToService(
	chunkData io.Reader,
	filename string,
	contentType string,
	bucket string,
	path string,
) (*UploadResponse, error) {
	return p.uploadFileInternalFromReader(chunkData, filename, contentType, bucket, path, false)
}

// uploadFileInternal handles the actual upload logic with is_hash parameter
func (p *UploadService) uploadFileInternal(
	file multipart.File,
	filename string,
	contentType string,
	bucket string,
	path string,
	isHash bool,
) (*UploadResponse, error) {
	return p.uploadFileInternalFromReader(file, filename, contentType, bucket, path, isHash)
}

// uploadFileInternalFromReader handles the actual upload logic with io.Reader using streaming
func (p *UploadService) uploadFileInternalFromReader(
	fileData io.Reader,
	filename string,
	contentType string,
	bucket string,
	path string,
	isHash bool,
) (*UploadResponse, error) {

	url := fmt.Sprintf("%s/api/v2/upload/file", p.UploadServiceURL)

	// Use io.Pipe for true streaming - no buffering
	pr, pw := io.Pipe()
	w := multipart.NewWriter(pw)

	// Channel to capture errors from goroutine
	errChan := make(chan error, 1)

	// Write multipart form in a goroutine
	go func() {
		defer pw.Close()
		defer w.Close()

		if err := w.WriteField("bucket", bucket); err != nil {
			errChan <- fmt.Errorf("failed to write bucket field: %w", err)
			return
		}

		if err := w.WriteField("path", path); err != nil {
			errChan <- fmt.Errorf("failed to write path field: %w", err)
			return
		}

		if err := w.WriteField("is_hash", fmt.Sprintf("%t", isHash)); err != nil {
			errChan <- fmt.Errorf("failed to write is_hash field: %w", err)
			return
		}

		h := make(map[string][]string)
		h["Content-Disposition"] = []string{
			fmt.Sprintf(`form-data; name="file"; filename="%s"`, filename),
		}
		h["Content-Type"] = []string{contentType}

		fw, err := w.CreatePart(h)
		if err != nil {
			errChan <- fmt.Errorf("failed to create form file: %w", err)
			return
		}

		// Stream file data directly - no buffering
		if _, err := io.Copy(fw, fileData); err != nil {
			errChan <- fmt.Errorf("failed to stream file data: %w", err)
			return
		}

		errChan <- nil
	}()

	req, err := http.NewRequest(http.MethodPost, url, pr)
	if err != nil {
		pr.Close()
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Private-Key", p.PrivateKey)

	client := &http.Client{}
	resp, err := client.Do(req)

	// Wait for write goroutine to finish and check for errors
	writeErr := <-errChan
	if writeErr != nil {
		if resp != nil {
			resp.Body.Close()
		}
		return nil, writeErr
	}

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
