package service

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type AnalysisRecord struct {
	ID             string    `json:"id"`
	RelationshipID string    `json:"relationship_id"`
	CreatedAt      time.Time `json:"created_at"`
	PeriodStart    time.Time `json:"period_start"`
	PeriodEnd      time.Time `json:"period_end"`
	MessageCount   int       `json:"message_count"`
	ActionCount    int       `json:"action_count"`
	EventCount     int       `json:"event_count"`
	ModelName      string    `json:"model_name"`
	ObjectKey      string    `json:"object_key"`
	ObjectURI      string    `json:"object_uri"`
	ReportPath     string    `json:"report_path"`
	Status         string    `json:"status"`
}

type AnalysisHistoryStore interface {
	Save(record AnalysisRecord) error
	List(filter AnalysisRecordFilter) ([]AnalysisRecord, error)
	Get(id string) (AnalysisRecord, error)
}

type AnalysisRecordFilter struct {
	RelationshipID string
	Status         string
}

type JSONLAnalysisHistoryStore struct {
	path string
}

func NewJSONLAnalysisHistoryStore(path string) *JSONLAnalysisHistoryStore {
	return &JSONLAnalysisHistoryStore{path: path}
}

func (s *JSONLAnalysisHistoryStore) Save(record AnalysisRecord) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(s.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	if _, err := file.Write(append(data, '\n')); err != nil {
		return err
	}
	return nil
}

func (s *JSONLAnalysisHistoryStore) List(filter AnalysisRecordFilter) ([]AnalysisRecord, error) {
	records, err := s.readAll()
	if err != nil {
		return nil, err
	}
	filtered := make([]AnalysisRecord, 0, len(records))
	for _, record := range records {
		if filter.RelationshipID != "" && record.RelationshipID != filter.RelationshipID {
			continue
		}
		if filter.Status != "" && record.Status != filter.Status {
			continue
		}
		filtered = append(filtered, record)
	}
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
	})
	return filtered, nil
}

func (s *JSONLAnalysisHistoryStore) Get(id string) (AnalysisRecord, error) {
	records, err := s.readAll()
	if err != nil {
		return AnalysisRecord{}, err
	}
	for i := len(records) - 1; i >= 0; i-- {
		if records[i].ID == id {
			return records[i], nil
		}
	}
	return AnalysisRecord{}, os.ErrNotExist
}

func (s *JSONLAnalysisHistoryStore) readAll() ([]AnalysisRecord, error) {
	file, err := os.Open(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()
	var records []AnalysisRecord
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024), 1024*1024*16)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var record AnalysisRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, scanner.Err()
}
