package service

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kevinmatthe/record_analysis/internal/storage"
)

func TestServiceUploadsParsesAnalyzesAndPersistsReport(t *testing.T) {
	tmp := t.TempDir()
	svc := NewChatAnalysisService(
		storage.NewLocalObjectStore(filepath.Join(tmp, "objects")),
		filepath.Join(tmp, "reports"),
	)

	result, err := svc.UploadAndAnalyze("../../records/某人/chat.csv", "rel_test", false)
	if err != nil {
		t.Fatal(err)
	}
	if result.StoredObject.ObjectKey == "" {
		t.Fatal("expected stored object key")
	}
	content, err := os.ReadFile(result.ReportPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != result.Analysis.Report.Markdown {
		t.Fatal("persisted report should match analysis report")
	}
}

func TestServiceCanFilterMessagesByPeriod(t *testing.T) {
	tmp := t.TempDir()
	source := filepath.Join(tmp, "chat.csv")
	err := os.WriteFile(source, []byte("时间,发送者,类型,内容\n2026-06-01 10:00:00,我,文本,第一天\n2026-06-02 10:00:00,某人,文本,第二天\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	svc := NewChatAnalysisService(
		storage.NewLocalObjectStore(filepath.Join(tmp, "objects")),
		filepath.Join(tmp, "reports"),
	)

	from := time.Date(2026, 6, 2, 0, 0, 0, 0, time.Local)
	to := time.Date(2026, 6, 3, 0, 0, 0, 0, time.Local)
	result, err := svc.UploadAndAnalyzeWithOptions(source, "rel_test", UploadAnalyzeOptions{
		From: &from,
		To:   &to,
	})
	if err != nil {
		t.Fatal(err)
	}

	if got := result.Analysis.Metrics.Values["message_volume"]; got != 1 {
		t.Fatalf("message volume = %v, want 1", got)
	}
	if result.Analysis.Messages[0].Content != "第二天" {
		t.Fatalf("message content = %q", result.Analysis.Messages[0].Content)
	}
}
