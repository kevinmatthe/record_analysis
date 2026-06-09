package storage

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalObjectStoreSavesUploadsByRelationship(t *testing.T) {
	tmp := t.TempDir()
	source := filepath.Join(tmp, "chat.txt")
	if err := os.WriteFile(source, []byte("[2026-06-01 20:00:00] 我: 你好"), 0o644); err != nil {
		t.Fatal(err)
	}

	store := NewLocalObjectStore(filepath.Join(tmp, "objects"))
	stored, err := store.PutChatFile(source, "rel_test")
	if err != nil {
		t.Fatal(err)
	}
	if stored.RelationshipID != "rel_test" {
		t.Fatalf("relationship id = %s", stored.RelationshipID)
	}
	if filepath.Ext(stored.ObjectKey) != ".txt" {
		t.Fatalf("object key should preserve extension: %s", stored.ObjectKey)
	}
	if _, err := os.Stat(stored.URI); err != nil {
		t.Fatalf("stored file missing: %v", err)
	}
}

func TestMinioObjectStoreUploadsFile(t *testing.T) {
	tmp := t.TempDir()
	source := filepath.Join(tmp, "chat.txt")
	content := []byte("[2026-06-01 20:00:00] 我: 你好")
	if err := os.WriteFile(source, content, 0o644); err != nil {
		t.Fatal(err)
	}
	var uploadedPath string
	var uploadedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method == http.MethodGet && r.URL.Query().Has("location") {
			w.Header().Set("Content-Type", "application/xml")
			_, _ = w.Write([]byte(`<LocationConstraint></LocationConstraint>`))
			return
		}
		if r.Method != http.MethodPut {
			t.Fatalf("method = %s", r.Method)
		}
		uploadedPath = r.URL.Path
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		uploadedBody = body
		w.Header().Set("ETag", `"test-etag"`)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	endpoint := strings.TrimPrefix(server.URL, "http://")
	store, err := NewMinioObjectStore(MinioConfig{
		Internal:  EndpointConfig{Endpoint: endpoint},
		External:  EndpointConfig{Endpoint: endpoint},
		AccessKey: "access-key",
		SecretKey: "secret-key",
		Bucket:    "record-analysis",
	})
	if err != nil {
		t.Fatal(err)
	}

	stored, err := store.PutChatFile(source, "rel_test")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.HasPrefix(uploadedPath, "/record-analysis/relationships/rel_test/uploads/") {
		t.Fatalf("uploaded path = %s", uploadedPath)
	}
	if !strings.Contains(string(uploadedBody), string(content)) {
		t.Fatalf("uploaded body = %q", string(uploadedBody))
	}
	if stored.URI == "" || stored.ObjectKey == "" {
		t.Fatalf("stored object missing uri/key: %+v", stored)
	}
}
