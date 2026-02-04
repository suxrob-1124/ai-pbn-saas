package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/http/cookiejar"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type httpClient struct {
	baseURL string
	client  *http.Client
}

func main() {
	apiURL := env("API_URL", "http://localhost:8080")
	dsn := env("DB_DSN", "postgres://auth:auth@localhost:5432/auth?sslmode=disable")
	baseDir := env("SERVER_DIR", "server")

	verifyEmail := env("VERIFY_EMAIL", "fileapi@example.com")
	verifyPass := env("VERIFY_PASSWORD", "Password123!!")
	projectName := env("VERIFY_PROJECT", "fileapi-project")
	domainURL := env("VERIFY_DOMAIN", "fileapi-example.com")

	jar, _ := cookiejar.New(nil)
	client := &httpClient{
		baseURL: apiURL,
		client:  &http.Client{Jar: jar, Timeout: 10 * time.Second},
	}

	if err := ensureLogin(client, verifyEmail, verifyPass); err != nil {
		die("login failed: %v", err)
	}

	projectID, err := ensureProject(client, projectName)
	if err != nil {
		die("project setup failed: %v", err)
	}

	domainID, err := ensureDomain(client, projectID, domainURL)
	if err != nil {
		die("domain setup failed: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(baseDir, domainURL), 0o755); err != nil {
		die("failed to create domain dir: %v", err)
	}

	filePath := filepath.Join(baseDir, domainURL, "index.html")
	content := []byte("hello file api")
	if err := os.WriteFile(filePath, content, 0o644); err != nil {
		die("failed to write file: %v", err)
	}

	fileID, err := upsertSiteFile(dsn, domainID, "index.html", content)
	if err != nil {
		die("failed to upsert site_files: %v", err)
	}

	files, err := client.getJSON(fmt.Sprintf("/api/domains/%s/files", domainID))
	if err != nil {
		die("list files failed: %v", err)
	}
	if !strings.Contains(string(files), "index.html") {
		die("file list missing index.html: %s", string(files))
	}

	if _, err := client.getJSON(fmt.Sprintf("/api/domains/%s/files/index.html", domainID)); err != nil {
		die("get file failed: %v", err)
	}

	updatePayload := []byte(`{"content":"updated content"}`)
	if err := client.putJSON(fmt.Sprintf("/api/domains/%s/files/index.html", domainID), updatePayload); err != nil {
		die("update file failed: %v", err)
	}

	history, err := client.getJSON(fmt.Sprintf("/api/domains/%s/files/%s/history", domainID, fileID))
	if err != nil {
		die("history failed: %v", err)
	}
	if len(history) == 0 {
		die("empty history response")
	}

	if err := client.putJSON(fmt.Sprintf("/api/domains/%s/files/../evil.txt", domainID), updatePayload); err == nil {
		die("expected path traversal error")
	}

	fmt.Println("OK: file API verification passed")
}

func ensureLogin(c *httpClient, email, password string) error {
	payload := map[string]string{"email": email, "password": password}
	body, _ := json.Marshal(payload)
	status, resp, err := c.doJSON(http.MethodPost, "/api/login", body)
	if err != nil {
		return err
	}
	if status == http.StatusOK {
		return nil
	}

	status, _, err = c.doJSON(http.MethodPost, "/api/register", body)
	if err != nil {
		return err
	}
	if status != http.StatusCreated && status != http.StatusBadRequest {
		return fmt.Errorf("register failed: %s", string(resp))
	}

	status, resp, err = c.doJSON(http.MethodPost, "/api/login", body)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("login failed: %s", string(resp))
	}
	return nil
}

func ensureProject(c *httpClient, name string) (string, error) {
	body, err := c.getJSON("/api/projects")
	if err != nil {
		return "", err
	}
	var list []map[string]any
	_ = json.Unmarshal(body, &list)
	for _, item := range list {
		if item["name"] == name {
			if id, ok := item["id"].(string); ok {
				return id, nil
			}
		}
	}

	payload := map[string]string{
		"name":     name,
		"country":  "se",
		"language": "sv",
		"status":   "draft",
	}
	data, _ := json.Marshal(payload)
	status, resp, err := c.doJSON(http.MethodPost, "/api/projects", data)
	if err != nil {
		return "", err
	}
	if status != http.StatusCreated {
		return "", fmt.Errorf("create project failed: %s", string(resp))
	}
	var created map[string]any
	if err := json.Unmarshal(resp, &created); err != nil {
		return "", err
	}
	id, _ := created["id"].(string)
	if id == "" {
		return "", fmt.Errorf("project id missing")
	}
	return id, nil
}

func ensureDomain(c *httpClient, projectID, domainURL string) (string, error) {
	body, err := c.getJSON(fmt.Sprintf("/api/projects/%s/domains", projectID))
	if err != nil {
		return "", err
	}
	var list []map[string]any
	_ = json.Unmarshal(body, &list)
	for _, item := range list {
		if item["url"] == domainURL {
			if id, ok := item["id"].(string); ok {
				return id, nil
			}
		}
	}

	payload := map[string]string{"url": domainURL}
	data, _ := json.Marshal(payload)
	status, resp, err := c.doJSON(http.MethodPost, fmt.Sprintf("/api/projects/%s/domains", projectID), data)
	if err != nil {
		return "", err
	}
	if status != http.StatusCreated {
		return "", fmt.Errorf("create domain failed: %s", string(resp))
	}
	var created map[string]any
	if err := json.Unmarshal(resp, &created); err != nil {
		return "", err
	}
	id, _ := created["id"].(string)
	if id == "" {
		return "", fmt.Errorf("domain id missing")
	}
	return id, nil
}

func upsertSiteFile(dsn, domainID, relPath string, content []byte) (string, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return "", err
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var id string
	err = db.QueryRowContext(ctx, `SELECT id FROM site_files WHERE domain_id=$1 AND path=$2`, domainID, relPath).Scan(&id)
	if err != nil && err != sql.ErrNoRows {
		return "", err
	}

	hash := sha256.Sum256(content)
	hashStr := hex.EncodeToString(hash[:])
	mimeType := detectMimeType(relPath, content)
	size := int64(len(content))

	if err == sql.ErrNoRows {
		newID := uuid()
		_, err = db.ExecContext(ctx, `INSERT INTO site_files(id, domain_id, path, content_hash, size_bytes, mime_type, created_at, updated_at)
			VALUES($1,$2,$3,$4,$5,$6,NOW(),NOW())`, newID, domainID, relPath, hashStr, size, mimeType)
		if err != nil {
			return "", err
		}
		return newID, nil
	}

	_, err = db.ExecContext(ctx, `UPDATE site_files SET content_hash=$1, size_bytes=$2, mime_type=$3, updated_at=NOW() WHERE id=$4`, hashStr, size, mimeType, id)
	if err != nil {
		return "", err
	}
	return id, nil
}

func (c *httpClient) getJSON(path string) ([]byte, error) {
	status, body, err := c.doJSON(http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("status %d: %s", status, string(body))
	}
	return body, nil
}

func (c *httpClient) putJSON(path string, payload []byte) error {
	status, body, err := c.doJSON(http.MethodPut, path, payload)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("status %d: %s", status, string(body))
	}
	return nil
}

func (c *httpClient) doJSON(method, path string, payload []byte) (int, []byte, error) {
	var body io.Reader
	if len(payload) > 0 {
		body = bytes.NewReader(payload)
	}
	req, err := http.NewRequest(method, c.baseURL+path, body)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, data, nil
}

func detectMimeType(path string, content []byte) string {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != "" {
		if t := mime.TypeByExtension(ext); t != "" {
			return t
		}
	}
	if len(content) > 0 {
		return http.DetectContentType(content)
	}
	return "application/octet-stream"
}

func uuid() string {
	b := sha256.Sum256([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
	return hex.EncodeToString(b[:16])
}

func env(key, fallback string) string {
	if val := strings.TrimSpace(os.Getenv(key)); val != "" {
		return val
	}
	return fallback
}

func die(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
