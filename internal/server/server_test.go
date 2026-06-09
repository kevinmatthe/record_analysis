package server

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kevinmatthe/record_analysis/internal/llm"
	"github.com/kevinmatthe/record_analysis/internal/model"
	"github.com/kevinmatthe/record_analysis/internal/service"
	"github.com/kevinmatthe/record_analysis/internal/storage"
)

func TestAnalyzeEndpointUploadsAndReturnsAnalysis(t *testing.T) {
	tempDir := t.TempDir()
	svc := service.NewChatAnalysisService(storage.NewLocalObjectStore(filepath.Join(tempDir, "objects")), filepath.Join(tempDir, "reports")).
		WithHistoryStore(service.NewJSONLAnalysisHistoryStore(filepath.Join(tempDir, "analysis", "index.jsonl")))
	handler := NewHandler(svc, Options{MaxUploadBytes: 1 << 20, MaxLLMMessages: 0})

	body, contentType := multipartBody(t, "chat.csv", "时间,发送者,类型,内容\n2026-06-01 20:00:00,我,文本,你好\n2026-06-01 20:01:00,小青,文本,你好呀\n")
	req := httptest.NewRequest(http.MethodPost, "/api/analyze", body)
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	var response struct {
		StoredObject struct {
			ObjectKey string `json:"object_key"`
		} `json:"stored_object"`
		Analysis struct {
			RelationshipID string `json:"relationship_id"`
			Metrics        struct {
				Values map[string]interface{} `json:"metrics"`
			} `json:"behavior_metrics"`
		} `json:"analysis"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.StoredObject.ObjectKey == "" {
		t.Fatal("expected stored object key")
	}
	if response.Analysis.RelationshipID != "rel_web" {
		t.Fatalf("relationship_id = %s", response.Analysis.RelationshipID)
	}
	if response.Analysis.Metrics.Values["message_volume"] != float64(2) {
		t.Fatalf("metrics = %+v", response.Analysis.Metrics.Values)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/analyses?relationship_id=rel_web", nil)
	listRec := httptest.NewRecorder()
	handler.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d body = %s", listRec.Code, listRec.Body.String())
	}
	if !strings.Contains(listRec.Body.String(), `"relationship_id":"rel_web"`) {
		t.Fatalf("list body = %s", listRec.Body.String())
	}
}

func TestJobEndpointTracksProgressPreviewAndHistory(t *testing.T) {
	tempDir := t.TempDir()
	history := service.NewJSONLAnalysisHistoryStore(filepath.Join(tempDir, "analysis", "index.jsonl"))
	svc := service.NewChatAnalysisService(storage.NewLocalObjectStore(filepath.Join(tempDir, "objects")), filepath.Join(tempDir, "reports")).
		WithHistoryStore(history)
	handler := NewHandler(svc, Options{MaxUploadBytes: 1 << 20, MaxLLMMessages: 1})

	body, contentType := multipartBodyWithFields(t, "chat.csv", "时间,发送者,类型,内容\n2026-06-01 20:00:00,我,文本,第一句\n2026-06-01 20:01:00,小青,文本,第二句\n2026-06-01 20:02:00,我,文本,第三句\n", map[string]string{
		"max_llm_messages": "1",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/jobs", body)
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID              string `json:"id"`
		MessageCount    int    `json:"message_count"`
		LLMMessageCount int    `json:"llm_message_count"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.MessageCount != 3 || created.LLMMessageCount != 1 {
		t.Fatalf("created job = %+v", created)
	}

	previewReq := httptest.NewRequest(http.MethodGet, "/api/jobs/"+created.ID+"/preview?page=1&page_size=2", nil)
	previewRec := httptest.NewRecorder()
	handler.ServeHTTP(previewRec, previewReq)
	if previewRec.Code != http.StatusOK || !strings.Contains(previewRec.Body.String(), `"total":3`) {
		t.Fatalf("preview status = %d body = %s", previewRec.Code, previewRec.Body.String())
	}

	timelineReq := httptest.NewRequest(http.MethodGet, "/api/jobs/"+created.ID+"/timeline?granularity=hour", nil)
	timelineRec := httptest.NewRecorder()
	handler.ServeHTTP(timelineRec, timelineReq)
	if timelineRec.Code != http.StatusOK {
		t.Fatalf("timeline status = %d body = %s", timelineRec.Code, timelineRec.Body.String())
	}
	if !strings.Contains(timelineRec.Body.String(), `"message_count":3`) || !strings.Contains(timelineRec.Body.String(), `"granularity":"hour"`) {
		t.Fatalf("timeline body = %s", timelineRec.Body.String())
	}

	var status string
	for i := 0; i < 20; i++ {
		statusReq := httptest.NewRequest(http.MethodGet, "/api/jobs/"+created.ID, nil)
		statusRec := httptest.NewRecorder()
		handler.ServeHTTP(statusRec, statusReq)
		if statusRec.Code != http.StatusOK {
			t.Fatalf("job status code = %d body = %s", statusRec.Code, statusRec.Body.String())
		}
		var job struct {
			Status         string  `json:"status"`
			ResultRecordID string  `json:"result_record_id"`
			Progress       float64 `json:"progress"`
		}
		if err := json.Unmarshal(statusRec.Body.Bytes(), &job); err != nil {
			t.Fatal(err)
		}
		status = job.Status
		if job.Status == string(JobStatusCompleted) {
			if job.ResultRecordID == "" || job.Progress < 1 {
				t.Fatalf("completed job missing result/progress: %+v", job)
			}
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if status != string(JobStatusCompleted) {
		t.Fatalf("job did not complete, status=%s", status)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/analyses?relationship_id=rel_web", nil)
	listRec := httptest.NewRecorder()
	handler.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK || !strings.Contains(listRec.Body.String(), `"relationship_id":"rel_web"`) {
		t.Fatalf("history status = %d body = %s", listRec.Code, listRec.Body.String())
	}
}

func TestJobTimelineBucketMessagesEndpoint(t *testing.T) {
	tempDir := t.TempDir()
	svc := service.NewChatAnalysisService(storage.NewLocalObjectStore(filepath.Join(tempDir, "objects")), filepath.Join(tempDir, "reports"))
	handler := NewHandler(svc, Options{MaxUploadBytes: 1 << 20})

	body, contentType := multipartBody(t, "chat.csv", "时间,发送者,类型,内容\n2026-06-01 20:00:00,我,文本,第一句\n2026-06-01 20:10:00,小青,文本,第二句\n2026-06-01 21:00:00,我,文本,第三句\n")
	req := httptest.NewRequest(http.MethodPost, "/api/jobs", body)
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("create status = %d body = %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	timelineReq := httptest.NewRequest(http.MethodGet, "/api/jobs/"+created.ID+"/timeline?granularity=hour", nil)
	timelineRec := httptest.NewRecorder()
	handler.ServeHTTP(timelineRec, timelineReq)
	if timelineRec.Code != http.StatusOK {
		t.Fatalf("timeline status = %d body = %s", timelineRec.Code, timelineRec.Body.String())
	}
	var timeline struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(timelineRec.Body.Bytes(), &timeline); err != nil {
		t.Fatal(err)
	}
	if len(timeline.Items) != 2 {
		t.Fatalf("timeline items = %+v", timeline.Items)
	}

	bucketReq := httptest.NewRequest(http.MethodGet, "/api/jobs/"+created.ID+"/timeline/"+timeline.Items[0].ID+"/messages?page=1&page_size=10", nil)
	bucketRec := httptest.NewRecorder()
	handler.ServeHTTP(bucketRec, bucketReq)
	if bucketRec.Code != http.StatusOK {
		t.Fatalf("bucket messages status = %d body = %s", bucketRec.Code, bucketRec.Body.String())
	}
	if !strings.Contains(bucketRec.Body.String(), `"total":2`) || !strings.Contains(bucketRec.Body.String(), "第一句") {
		t.Fatalf("bucket body = %s", bucketRec.Body.String())
	}

	for i := 0; i < 20; i++ {
		statusReq := httptest.NewRequest(http.MethodGet, "/api/jobs/"+created.ID, nil)
		statusRec := httptest.NewRecorder()
		handler.ServeHTTP(statusRec, statusReq)
		if strings.Contains(statusRec.Body.String(), `"status":"completed"`) || strings.Contains(statusRec.Body.String(), `"status":"failed"`) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("job %s did not finish in time", created.ID)
}

func TestJobTimelineEndpointFiltersByRequestedRange(t *testing.T) {
	tempDir := t.TempDir()
	svc := service.NewChatAnalysisService(storage.NewLocalObjectStore(filepath.Join(tempDir, "objects")), filepath.Join(tempDir, "reports"))
	handler := NewHandler(svc, Options{MaxUploadBytes: 1 << 20})

	body, contentType := multipartBody(t, "chat.csv", "时间,发送者,类型,内容\n2026-06-01 20:00:00,我,文本,第一天\n2026-06-02 09:10:00,小青,文本,第二天早上\n2026-06-02 10:05:00,我,文本,第二天上午\n")
	req := httptest.NewRequest(http.MethodPost, "/api/jobs", body)
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("create status = %d body = %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	timelineReq := httptest.NewRequest(http.MethodGet, "/api/jobs/"+created.ID+"/timeline?granularity=hour&start_time=2026-06-02T00:00:00%2B08:00&end_time=2026-06-03T00:00:00%2B08:00", nil)
	timelineRec := httptest.NewRecorder()
	handler.ServeHTTP(timelineRec, timelineReq)
	if timelineRec.Code != http.StatusOK {
		t.Fatalf("timeline status = %d body = %s", timelineRec.Code, timelineRec.Body.String())
	}
	var timeline struct {
		Items []struct {
			StartTime    time.Time `json:"start_time"`
			MessageCount int       `json:"message_count"`
		} `json:"items"`
	}
	if err := json.Unmarshal(timelineRec.Body.Bytes(), &timeline); err != nil {
		t.Fatal(err)
	}
	if len(timeline.Items) != 2 {
		t.Fatalf("timeline items = %+v", timeline.Items)
	}
	for _, item := range timeline.Items {
		if item.StartTime.Day() != 2 {
			t.Fatalf("unexpected bucket outside requested range: %+v", timeline.Items)
		}
	}

	for i := 0; i < 20; i++ {
		statusReq := httptest.NewRequest(http.MethodGet, "/api/jobs/"+created.ID, nil)
		statusRec := httptest.NewRecorder()
		handler.ServeHTTP(statusRec, statusReq)
		if strings.Contains(statusRec.Body.String(), `"status":"completed"`) || strings.Contains(statusRec.Body.String(), `"status":"failed"`) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("job %s did not finish in time", created.ID)
}

func TestJobBranchPreviewEndpoint(t *testing.T) {
	tempDir := t.TempDir()
	svc := service.NewChatAnalysisService(storage.NewLocalObjectStore(filepath.Join(tempDir, "objects")), filepath.Join(tempDir, "reports"))
	handler := NewHandler(svc, Options{MaxUploadBytes: 1 << 20})

	body, contentType := multipartBody(t, "chat.csv", "时间,发送者,类型,内容\n2026-06-01 10:05:00,我,文本,今天中午吃什么\n2026-06-01 10:20:00,小青,文本,去吃面吧\n2026-06-01 11:01:00,我,文本,那我们十一点半出门\n2026-06-01 11:08:00,小青,文本,可以\n2026-06-01 15:05:00,我,文本,晚上看电影吗\n")
	req := httptest.NewRequest(http.MethodPost, "/api/jobs", body)
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("create status = %d body = %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	previewBody := strings.NewReader(`{"granularity":"hour","start_time":"2026-06-01T11:00:00+08:00","end_time":"2026-06-01T12:00:00+08:00"}`)
	previewReq := httptest.NewRequest(http.MethodPost, "/api/jobs/"+created.ID+"/branches/preview", previewBody)
	previewReq.Header.Set("Content-Type", "application/json")
	previewRec := httptest.NewRecorder()
	handler.ServeHTTP(previewRec, previewReq)
	if previewRec.Code != http.StatusOK {
		t.Fatalf("preview status = %d body = %s", previewRec.Code, previewRec.Body.String())
	}
	if !strings.Contains(previewRec.Body.String(), `"message_count":4`) || !strings.Contains(previewRec.Body.String(), `"status":"unseen"`) {
		t.Fatalf("preview body = %s", previewRec.Body.String())
	}

	for i := 0; i < 20; i++ {
		statusReq := httptest.NewRequest(http.MethodGet, "/api/jobs/"+created.ID, nil)
		statusRec := httptest.NewRecorder()
		handler.ServeHTTP(statusRec, statusReq)
		if strings.Contains(statusRec.Body.String(), `"status":"completed"`) || strings.Contains(statusRec.Body.String(), `"status":"failed"`) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("job %s did not finish in time", created.ID)
}

func TestJobBranchCreateAndListEndpoints(t *testing.T) {
	tempDir := t.TempDir()
	svc := service.NewChatAnalysisService(storage.NewLocalObjectStore(filepath.Join(tempDir, "objects")), filepath.Join(tempDir, "reports"))
	handler := NewHandler(svc, Options{MaxUploadBytes: 1 << 20})

	body, contentType := multipartBody(t, "chat.csv", "时间,发送者,类型,内容\n2026-06-01 10:05:00,我,文本,今天中午吃什么\n2026-06-01 10:20:00,小青,文本,去吃面吧\n2026-06-01 11:01:00,我,文本,那我们十一点半出门\n2026-06-01 11:08:00,小青,文本,可以\n2026-06-01 15:05:00,我,文本,晚上看电影吗\n")
	req := httptest.NewRequest(http.MethodPost, "/api/jobs", body)
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("create status = %d body = %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	createBody := strings.NewReader(`{"granularity":"hour","start_time":"2026-06-01T11:00:00+08:00","end_time":"2026-06-01T12:00:00+08:00","title":"午饭话题"}`)
	createReq := httptest.NewRequest(http.MethodPost, "/api/jobs/"+created.ID+"/branches", createBody)
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	handler.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("branch create status = %d body = %s", createRec.Code, createRec.Body.String())
	}
	if !strings.Contains(createRec.Body.String(), `"title":"午饭话题"`) || !strings.Contains(createRec.Body.String(), `"message_count":4`) {
		t.Fatalf("branch create body = %s", createRec.Body.String())
	}
	var firstBranch struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &firstBranch); err != nil {
		t.Fatal(err)
	}

	duplicateBody := strings.NewReader(`{"granularity":"hour","start_time":"2026-06-01T11:00:00+08:00","end_time":"2026-06-01T12:00:00+08:00","title":"重复午饭话题"}`)
	duplicateReq := httptest.NewRequest(http.MethodPost, "/api/jobs/"+created.ID+"/branches", duplicateBody)
	duplicateReq.Header.Set("Content-Type", "application/json")
	duplicateRec := httptest.NewRecorder()
	handler.ServeHTTP(duplicateRec, duplicateReq)
	if duplicateRec.Code != http.StatusCreated {
		t.Fatalf("duplicate branch create status = %d body = %s", duplicateRec.Code, duplicateRec.Body.String())
	}
	var duplicateBranch struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(duplicateRec.Body.Bytes(), &duplicateBranch); err != nil {
		t.Fatal(err)
	}
	if duplicateBranch.ID != firstBranch.ID {
		t.Fatalf("duplicate branch id = %s, want %s", duplicateBranch.ID, firstBranch.ID)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/jobs/"+created.ID+"/branches", nil)
	listRec := httptest.NewRecorder()
	handler.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("branch list status = %d body = %s", listRec.Code, listRec.Body.String())
	}
	if !strings.Contains(listRec.Body.String(), `"title":"午饭话题"`) {
		t.Fatalf("branch list body = %s", listRec.Body.String())
	}
	var list struct {
		Items []AnalysisBranch `json:"items"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("branch list items = %+v", list.Items)
	}

	for i := 0; i < 20; i++ {
		statusReq := httptest.NewRequest(http.MethodGet, "/api/jobs/"+created.ID, nil)
		statusRec := httptest.NewRecorder()
		handler.ServeHTTP(statusRec, statusReq)
		if strings.Contains(statusRec.Body.String(), `"status":"completed"`) || strings.Contains(statusRec.Body.String(), `"status":"failed"`) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("job %s did not finish in time", created.ID)
}

func TestJobBranchRunEndpoint(t *testing.T) {
	tempDir := t.TempDir()
	extractor := &trackingExtractor{}
	svc := service.NewChatAnalysisService(storage.NewLocalObjectStore(filepath.Join(tempDir, "objects")), filepath.Join(tempDir, "reports")).
		WithExtractor(extractor)
	handler := NewHandler(svc, Options{MaxUploadBytes: 1 << 20, MaxLLMMessages: 500})

	body, contentType := multipartBodyWithFields(t, "chat.csv", "时间,发送者,类型,内容\n2026-06-01 10:05:00,我,文本,今天中午吃什么\n2026-06-01 10:20:00,小青,文本,去吃面吧\n2026-06-01 11:01:00,我,文本,那我们十一点半出门\n2026-06-01 11:08:00,小青,文本,可以\n2026-06-01 15:05:00,我,文本,晚上看电影吗\n", map[string]string{
		"analysis_mode":    "quick",
		"max_llm_messages": "2",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/jobs", body)
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("create status = %d body = %s", rec.Code, rec.Body.String())
	}
	var createdJob struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &createdJob); err != nil {
		t.Fatal(err)
	}

	createBody := strings.NewReader(`{"granularity":"hour","start_time":"2026-06-01T11:00:00+08:00","end_time":"2026-06-01T12:00:00+08:00","title":"午饭话题"}`)
	createReq := httptest.NewRequest(http.MethodPost, "/api/jobs/"+createdJob.ID+"/branches", createBody)
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	handler.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("branch create status = %d body = %s", createRec.Code, createRec.Body.String())
	}
	var createdBranch struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &createdBranch); err != nil {
		t.Fatal(err)
	}

	runReq := httptest.NewRequest(http.MethodPost, "/api/jobs/"+createdJob.ID+"/branches/"+createdBranch.ID+"/run", strings.NewReader(`{"analysis_mode":"quick","max_llm_messages":2}`))
	runReq.Header.Set("Content-Type", "application/json")
	runRec := httptest.NewRecorder()
	handler.ServeHTTP(runRec, runReq)
	if runRec.Code != http.StatusAccepted {
		t.Fatalf("branch run status = %d body = %s", runRec.Code, runRec.Body.String())
	}

	for i := 0; i < 40; i++ {
		listReq := httptest.NewRequest(http.MethodGet, "/api/jobs/"+createdJob.ID+"/branches", nil)
		listRec := httptest.NewRecorder()
		handler.ServeHTTP(listRec, listReq)
		if listRec.Code != http.StatusOK {
			t.Fatalf("branch list status = %d body = %s", listRec.Code, listRec.Body.String())
		}
		if strings.Contains(listRec.Body.String(), `"status":"completed"`) && strings.Contains(listRec.Body.String(), `"report_markdown":"ok"`) {
			if extractor.messageCount != 2 {
				t.Fatalf("branch extractor message count = %d, want 2", extractor.messageCount)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("branch %s did not finish in time", createdBranch.ID)
}

func TestWorkItemInvalidResultJSONFallsBackToEmptyArray(t *testing.T) {
	item := fromDBWorkItem(RecordAnalysisWorkItem{
		ID:         "wi_test",
		JobID:      "job_test",
		Kind:       "word_cloud",
		ScopeType:  "bucket",
		ScopeID:    "day_2026-06-01T00:00:00+08:00",
		ResultJSON: "[],[]",
	})
	payload, err := json.Marshal(item)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(payload), `"result":[]`) {
		t.Fatalf("payload = %s", payload)
	}
}

func TestParseOptionalRFC3339Range(t *testing.T) {
	start, end, err := parseOptionalRFC3339Range("2026-06-02T00:00:00+08:00", "2026-06-03T00:00:00+08:00")
	if err != nil {
		t.Fatal(err)
	}
	if start == nil || end == nil || !end.After(*start) {
		t.Fatalf("range = %v %v", start, end)
	}

	start, end, err = parseOptionalRFC3339Range("", "")
	if err != nil || start != nil || end != nil {
		t.Fatalf("empty range = %v %v err=%v", start, end, err)
	}

	if _, _, err := parseOptionalRFC3339Range("2026-06-03T00:00:00+08:00", "2026-06-02T00:00:00+08:00"); err == nil {
		t.Fatal("expected inverted range error")
	}
}

func TestBranchDetailEndpoint(t *testing.T) {
	tempDir := t.TempDir()
	extractor := &trackingExtractor{}
	svc := service.NewChatAnalysisService(storage.NewLocalObjectStore(filepath.Join(tempDir, "objects")), filepath.Join(tempDir, "reports")).
		WithExtractor(extractor)
	handler := NewHandler(svc, Options{MaxUploadBytes: 1 << 20, MaxLLMMessages: 500})

	body, contentType := multipartBodyWithFields(t, "chat.csv", "时间,发送者,类型,内容\n2026-06-01 10:05:00,我,文本,今天中午吃什么\n2026-06-01 10:20:00,小青,文本,去吃面吧\n2026-06-01 11:01:00,我,文本,那我们十一点半出门\n2026-06-01 11:08:00,小青,文本,可以\n", map[string]string{
		"analysis_mode":    "quick",
		"max_llm_messages": "2",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/jobs", body)
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("create status = %d body = %s", rec.Code, rec.Body.String())
	}
	var createdJob struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &createdJob); err != nil {
		t.Fatal(err)
	}

	createReq := httptest.NewRequest(http.MethodPost, "/api/jobs/"+createdJob.ID+"/branches", strings.NewReader(`{"granularity":"hour","start_time":"2026-06-01T10:00:00+08:00","end_time":"2026-06-01T12:00:00+08:00","title":"午饭话题"}`))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	handler.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("branch create status = %d body = %s", createRec.Code, createRec.Body.String())
	}
	var branch struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &branch); err != nil {
		t.Fatal(err)
	}

	runReq := httptest.NewRequest(http.MethodPost, "/api/jobs/"+createdJob.ID+"/branches/"+branch.ID+"/run", strings.NewReader(`{"analysis_mode":"quick","max_llm_messages":2}`))
	runReq.Header.Set("Content-Type", "application/json")
	runRec := httptest.NewRecorder()
	handler.ServeHTTP(runRec, runReq)
	if runRec.Code != http.StatusAccepted {
		t.Fatalf("branch run status = %d body = %s", runRec.Code, runRec.Body.String())
	}

	for i := 0; i < 40; i++ {
		detailReq := httptest.NewRequest(http.MethodGet, "/api/branches/"+branch.ID, nil)
		detailRec := httptest.NewRecorder()
		handler.ServeHTTP(detailRec, detailReq)
		if detailRec.Code != http.StatusOK {
			t.Fatalf("branch detail status = %d body = %s", detailRec.Code, detailRec.Body.String())
		}
		if strings.Contains(detailRec.Body.String(), `"status":"completed"`) && strings.Contains(detailRec.Body.String(), `"report_markdown":"ok"`) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("branch %s detail did not reach completed", branch.ID)
}

func TestAnalyzeEndpointRejectsUnsupportedFormat(t *testing.T) {
	tempDir := t.TempDir()
	svc := service.NewChatAnalysisService(storage.NewLocalObjectStore(filepath.Join(tempDir, "objects")), filepath.Join(tempDir, "reports"))
	handler := NewHandler(svc, Options{MaxUploadBytes: 1 << 20})

	body, contentType := multipartBody(t, "chat.xlsx", "not supported")
	req := httptest.NewRequest(http.MethodPost, "/api/analyze", body)
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
}

func TestAnalyzeEndpointCapsMessagesSentToExtractor(t *testing.T) {
	tempDir := t.TempDir()
	extractor := &trackingExtractor{}
	svc := service.NewChatAnalysisService(storage.NewLocalObjectStore(filepath.Join(tempDir, "objects")), filepath.Join(tempDir, "reports")).
		WithExtractor(extractor)
	handler := NewHandler(svc, Options{MaxUploadBytes: 1 << 20, MaxLLMMessages: 500})

	body, contentType := multipartBodyWithFields(t, "chat.csv", "时间,发送者,类型,内容\n2026-06-01 20:00:00,我,文本,第一句\n2026-06-01 20:01:00,小青,文本,第二句\n2026-06-01 20:02:00,我,文本,第三句\n", map[string]string{
		"max_llm_messages": "2",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/analyze", body)
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if !extractor.called {
		t.Fatal("extractor should be called after messages are capped")
	}
	if extractor.messageCount != 2 {
		t.Fatalf("extractor message count = %d, want 2", extractor.messageCount)
	}
	if extractor.firstMsgID != model.StableMessageID(2) || extractor.lastMsgID != model.StableMessageID(3) {
		t.Fatalf("expected latest two messages, got first=%s last=%s", extractor.firstMsgID, extractor.lastMsgID)
	}
}

func TestAuthProtectsAPIWhenConfigured(t *testing.T) {
	tempDir := t.TempDir()
	svc := service.NewChatAnalysisService(storage.NewLocalObjectStore(filepath.Join(tempDir, "objects")), filepath.Join(tempDir, "reports"))
	handler := NewHandler(svc, Options{AuthUsername: "admin", AuthPassword: "secret", AllowedOrigin: "http://localhost:5173"})

	meReq := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	meRec := httptest.NewRecorder()
	handler.ServeHTTP(meRec, meReq)
	if meRec.Code != http.StatusUnauthorized {
		t.Fatalf("me status = %d", meRec.Code)
	}

	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"username":"admin","password":"secret"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginReq.Header.Set("Origin", "http://localhost:5173")
	loginRec := httptest.NewRecorder()
	handler.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login status = %d body = %s", loginRec.Code, loginRec.Body.String())
	}
	if loginRec.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
		t.Fatalf("cors header = %s", loginRec.Header().Get("Access-Control-Allow-Origin"))
	}
	cookies := loginRec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected session cookie")
	}

	authedReq := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	authedReq.AddCookie(cookies[0])
	authedRec := httptest.NewRecorder()
	handler.ServeHTTP(authedRec, authedReq)
	if authedRec.Code != http.StatusOK {
		t.Fatalf("authed me status = %d body = %s", authedRec.Code, authedRec.Body.String())
	}
}

func TestAnalysisDetailAndReportEndpoints(t *testing.T) {
	tempDir := t.TempDir()
	reportPath := filepath.Join(tempDir, "reports", "rel_web", "latest.md")
	if err := os.MkdirAll(filepath.Dir(reportPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(reportPath, []byte("# Report"), 0o644); err != nil {
		t.Fatal(err)
	}
	history := service.NewJSONLAnalysisHistoryStore(filepath.Join(tempDir, "analysis", "index.jsonl"))
	if err := history.Save(service.AnalysisRecord{ID: "ana_test", RelationshipID: "rel_web", ReportPath: reportPath, Status: "completed"}); err != nil {
		t.Fatal(err)
	}
	svc := service.NewChatAnalysisService(storage.NewLocalObjectStore(filepath.Join(tempDir, "objects")), filepath.Join(tempDir, "reports")).
		WithHistoryStore(history)
	handler := NewHandler(svc, Options{})

	detailReq := httptest.NewRequest(http.MethodGet, "/api/analyses/ana_test", nil)
	detailRec := httptest.NewRecorder()
	handler.ServeHTTP(detailRec, detailReq)
	if detailRec.Code != http.StatusOK {
		t.Fatalf("detail status = %d body = %s", detailRec.Code, detailRec.Body.String())
	}

	reportReq := httptest.NewRequest(http.MethodGet, "/api/analyses/ana_test/report", nil)
	reportRec := httptest.NewRecorder()
	handler.ServeHTTP(reportRec, reportReq)
	if reportRec.Code != http.StatusOK || !strings.Contains(reportRec.Body.String(), "# Report") {
		t.Fatalf("report status = %d body = %s", reportRec.Code, reportRec.Body.String())
	}
}

func multipartBody(t *testing.T, filename string, content string) (*bytes.Buffer, string) {
	return multipartBodyWithFields(t, filename, content, nil)
}

func multipartBodyWithFields(t *testing.T, filename string, content string, fields map[string]string) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.WriteField("relationship_id", "rel_web"); err != nil {
		t.Fatal(err)
	}
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatal(err)
		}
	}
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return body, writer.FormDataContentType()
}

type trackingExtractor struct {
	called       bool
	messageCount int
	firstMsgID   string
	lastMsgID    string
}

func (e *trackingExtractor) ExtractActions(_ context.Context, messages []model.Message, _ []model.Segment) ([]model.MessageAction, error) {
	e.called = true
	e.messageCount = len(messages)
	if len(messages) > 0 {
		e.firstMsgID = messages[0].ID
		e.lastMsgID = messages[len(messages)-1].ID
	}
	return nil, nil
}

func (e *trackingExtractor) ExtractEvents(context.Context, []model.Message, []model.Segment, []model.MessageAction) ([]model.RelationshipEvent, error) {
	e.called = true
	return nil, nil
}

func (e *trackingExtractor) GenerateDimensions(_ context.Context, metrics model.BehaviorMetrics, _ []model.RelationshipEvent) (model.PsychologicalDimensions, error) {
	e.called = true
	return model.PsychologicalDimensions{RelationshipID: metrics.RelationshipID, Values: map[string]interface{}{}}, nil
}

func (e *trackingExtractor) GenerateReport(_ context.Context, input llm.ReportInput) (model.PeriodReport, error) {
	e.called = true
	e.messageCount = len(input.Messages)
	if len(input.Messages) > 0 {
		e.firstMsgID = input.Messages[0].ID
		e.lastMsgID = input.Messages[len(input.Messages)-1].ID
	}
	return model.PeriodReport{RelationshipID: input.RelationshipID, Markdown: "ok"}, nil
}
