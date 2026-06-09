package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"time"

	"github.com/kevinmatthe/record_analysis/internal/analyzer"
	"github.com/kevinmatthe/record_analysis/internal/importer"
	"github.com/kevinmatthe/record_analysis/internal/llm"
	"github.com/kevinmatthe/record_analysis/internal/model"
	"github.com/kevinmatthe/record_analysis/internal/storage"
)

type UploadAnalysisResult struct {
	StoredObject storage.StoredObject `json:"stored_object"`
	Analysis     model.AnalysisResult `json:"analysis"`
	ReportPath   string               `json:"report_path"`
	Record       AnalysisRecord       `json:"record"`
}

type UploadAnalyzeOptions struct {
	Context        context.Context
	IncludeSystem  bool
	From           *time.Time
	To             *time.Time
	MaxLLMMessages int
	AnalysisMode   string
	Progress       func(stage string, current int, total int)
}

type ChatAnalysisService struct {
	objectStore storage.ObjectStore
	reportRoot  string
	extractor   llm.Extractor
	history     AnalysisHistoryStore
}

func NewChatAnalysisService(objectStore storage.ObjectStore, reportRoot string) *ChatAnalysisService {
	return &ChatAnalysisService{objectStore: objectStore, reportRoot: reportRoot}
}

func (s *ChatAnalysisService) WithExtractor(extractor llm.Extractor) *ChatAnalysisService {
	s.extractor = extractor
	return s
}

func (s *ChatAnalysisService) Extractor() llm.Extractor {
	return s.extractor
}

func (s *ChatAnalysisService) WithHistoryStore(history AnalysisHistoryStore) *ChatAnalysisService {
	s.history = history
	return s
}

func (s *ChatAnalysisService) UploadAndAnalyze(chatFile string, relationshipID string, includeSystem bool) (UploadAnalysisResult, error) {
	return s.UploadAndAnalyzeWithOptions(chatFile, relationshipID, UploadAnalyzeOptions{IncludeSystem: includeSystem})
}

func (s *ChatAnalysisService) UploadAndAnalyzeWithOptions(chatFile string, relationshipID string, options UploadAnalyzeOptions) (UploadAnalysisResult, error) {
	ctx := options.Context
	if ctx == nil {
		ctx = context.Background()
	}
	reportServiceProgress(options.Progress, "object_store_upload", 0, 1)
	stored, err := s.objectStore.PutChatFile(chatFile, relationshipID)
	if err != nil {
		return UploadAnalysisResult{}, err
	}
	reportServiceProgress(options.Progress, "object_store_upload", 1, 1)
	reportServiceProgress(options.Progress, "message_parse", 0, 1)
	messages, err := importer.ParseChatFile(chatFile, relationshipID, options.IncludeSystem)
	if err != nil {
		return UploadAnalysisResult{}, err
	}
	messages = filterPeriod(messages, options.From, options.To)
	reportServiceProgress(options.Progress, "message_parse", len(messages), len(messages))
	analysis, err := analyzer.AnalyzeMessagesWithOptions(ctx, messages, relationshipID, analyzer.AnalyzeOptions{
		Extractor:      s.extractor,
		MaxLLMMessages: options.MaxLLMMessages,
		Mode:           options.AnalysisMode,
		Progress:       options.Progress,
	})
	if err != nil {
		return UploadAnalysisResult{}, err
	}
	analysis = denormalizeAnalysis(analysis)
	reportServiceProgress(options.Progress, "report_persist", 0, 1)
	reportPath := filepath.Join(s.reportRoot, relationshipID, "latest.md")
	if err := os.MkdirAll(filepath.Dir(reportPath), 0o755); err != nil {
		return UploadAnalysisResult{}, err
	}
	if err := os.WriteFile(reportPath, []byte(analysis.Report.Markdown), 0o644); err != nil {
		return UploadAnalysisResult{}, err
	}
	reportServiceProgress(options.Progress, "report_persist", 1, 1)
	record := AnalysisRecord{
		ID:             newAnalysisID(),
		RelationshipID: relationshipID,
		CreatedAt:      time.Now(),
		PeriodStart:    analysis.Metrics.PeriodStart,
		PeriodEnd:      analysis.Metrics.PeriodEnd,
		MessageCount:   len(analysis.Messages),
		ActionCount:    len(analysis.Actions),
		EventCount:     len(analysis.Events),
		ModelName:      analysis.Report.ModelName,
		ObjectKey:      stored.ObjectKey,
		ObjectURI:      stored.URI,
		ReportPath:     reportPath,
		Status:         "completed",
	}
	if s.history != nil {
		if err := s.history.Save(record); err != nil {
			return UploadAnalysisResult{}, err
		}
	}
	return UploadAnalysisResult{StoredObject: stored, Analysis: analysis, ReportPath: reportPath, Record: record}, nil
}

func reportServiceProgress(progress func(stage string, current int, total int), stage string, current int, total int) {
	if progress != nil {
		progress(stage, current, total)
	}
}

func (s *ChatAnalysisService) ListAnalyses(filter AnalysisRecordFilter) ([]AnalysisRecord, error) {
	if s.history == nil {
		return nil, nil
	}
	return s.history.List(filter)
}

func (s *ChatAnalysisService) GetAnalysisRecord(id string) (AnalysisRecord, error) {
	if s.history == nil {
		return AnalysisRecord{}, os.ErrNotExist
	}
	return s.history.Get(id)
}

func (s *ChatAnalysisService) ReadReport(id string) (string, AnalysisRecord, error) {
	record, err := s.GetAnalysisRecord(id)
	if err != nil {
		return "", AnalysisRecord{}, err
	}
	data, err := os.ReadFile(record.ReportPath)
	if err != nil {
		return "", AnalysisRecord{}, err
	}
	return string(data), record, nil
}

func newAnalysisID() string {
	var data [8]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "ana_" + time.Now().Format("20060102150405")
	}
	return "ana_" + hex.EncodeToString(data[:])
}

func filterPeriod(messages []model.Message, from *time.Time, to *time.Time) []model.Message {
	if from == nil && to == nil {
		return messages
	}
	filtered := make([]model.Message, 0, len(messages))
	for _, message := range messages {
		if from != nil && message.MsgTime.Before(*from) {
			continue
		}
		if to != nil && !message.MsgTime.Before(*to) {
			continue
		}
		filtered = append(filtered, message)
	}
	return filtered
}
