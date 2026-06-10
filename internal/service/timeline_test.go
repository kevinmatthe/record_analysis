package service

import (
	"testing"
	"time"

	"github.com/kevinmatthe/record_analysis/internal/model"
)

func TestBuildTimelineBucketsByHour(t *testing.T) {
	messages := []model.Message{
		{ID: "MSG_000001", Sender: "PERSON_A", OriginalSender: "我", MsgTime: time.Date(2026, 6, 1, 10, 5, 0, 0, time.Local), Content: "早上好"},
		{ID: "MSG_000002", Sender: "PERSON_B", OriginalSender: "小青", MsgTime: time.Date(2026, 6, 1, 10, 20, 0, 0, time.Local), Content: "早"},
		{ID: "MSG_000003", Sender: "PERSON_A", OriginalSender: "我", MsgTime: time.Date(2026, 6, 1, 11, 1, 0, 0, time.Local), Content: "吃饭了吗"},
	}

	buckets, err := BuildTimelineBuckets(messages, "hour")
	if err != nil {
		t.Fatal(err)
	}
	if len(buckets) != 2 {
		t.Fatalf("bucket count = %d, want 2", len(buckets))
	}
	if buckets[0].MessageCount != 2 || buckets[0].ParticipantCount != 2 {
		t.Fatalf("first bucket = %+v", buckets[0])
	}
	if buckets[0].ParticipantMessages["我"] != 1 || buckets[0].ParticipantMessages["小青"] != 1 {
		t.Fatalf("participant counts = %+v", buckets[0].ParticipantMessages)
	}
	if buckets[0].Preview == "" || buckets[0].FirstMessageID != "MSG_000001" || buckets[0].LastMessageID != "MSG_000002" {
		t.Fatalf("first bucket preview fields = %+v", buckets[0])
	}
}

func TestBuildTimelineBucketsRejectsUnsupportedGranularity(t *testing.T) {
	_, err := BuildTimelineBuckets(nil, "quarter")
	if err == nil {
		t.Fatal("expected granularity error")
	}
}

func TestBuildTimelineClustersMergesAdjacentBuckets(t *testing.T) {
	messages := []model.Message{
		{ID: "MSG_000001", Sender: "PERSON_A", OriginalSender: "我", MsgTime: time.Date(2026, 6, 1, 10, 5, 0, 0, time.Local), Content: "今天中午吃什么"},
		{ID: "MSG_000002", Sender: "PERSON_B", OriginalSender: "小青", MsgTime: time.Date(2026, 6, 1, 10, 20, 0, 0, time.Local), Content: "去吃面吧"},
		{ID: "MSG_000003", Sender: "PERSON_A", OriginalSender: "我", MsgTime: time.Date(2026, 6, 1, 11, 1, 0, 0, time.Local), Content: "那我们十一点半出门"},
		{ID: "MSG_000004", Sender: "PERSON_B", OriginalSender: "小青", MsgTime: time.Date(2026, 6, 1, 11, 8, 0, 0, time.Local), Content: "可以"},
		{ID: "MSG_000005", Sender: "PERSON_A", OriginalSender: "我", MsgTime: time.Date(2026, 6, 1, 15, 5, 0, 0, time.Local), Content: "晚上看电影吗"},
	}

	buckets, err := BuildTimelineBuckets(messages, "hour")
	if err != nil {
		t.Fatal(err)
	}
	clusters := BuildTimelineClusters(buckets)
	if len(clusters) != 2 {
		t.Fatalf("cluster count = %d, want 2", len(clusters))
	}
	if clusters[0].BucketCount != 2 || clusters[0].MessageCount != 4 {
		t.Fatalf("first cluster = %+v", clusters[0])
	}
	if clusters[0].Status != "unseen" || len(clusters[0].BucketIDs) != 2 {
		t.Fatalf("cluster metadata = %+v", clusters[0])
	}
}

func TestBuildTimelineClustersDoesNotUseAccumulatedClusterDurationAsGap(t *testing.T) {
	messages := []model.Message{
		{ID: "MSG_000001", Sender: "PERSON_A", MsgTime: time.Date(2026, 6, 1, 10, 5, 0, 0, time.Local), Content: "十点"},
		{ID: "MSG_000002", Sender: "PERSON_A", MsgTime: time.Date(2026, 6, 1, 11, 5, 0, 0, time.Local), Content: "十一点"},
		{ID: "MSG_000003", Sender: "PERSON_A", MsgTime: time.Date(2026, 6, 1, 12, 5, 0, 0, time.Local), Content: "十二点"},
		{ID: "MSG_000004", Sender: "PERSON_A", MsgTime: time.Date(2026, 6, 1, 13, 5, 0, 0, time.Local), Content: "十三点"},
		{ID: "MSG_000005", Sender: "PERSON_A", MsgTime: time.Date(2026, 6, 1, 16, 5, 0, 0, time.Local), Content: "十六点"},
	}

	buckets, err := BuildTimelineBuckets(messages, "hour")
	if err != nil {
		t.Fatal(err)
	}
	clusters := BuildTimelineClusters(buckets)
	if len(clusters) != 2 {
		t.Fatalf("cluster count = %d, want 2: %+v", len(clusters), clusters)
	}
	if clusters[0].BucketCount != 4 || clusters[1].BucketCount != 1 {
		t.Fatalf("clusters = %+v", clusters)
	}
}

func TestPreviewBranchUsesRequestedRange(t *testing.T) {
	messages := []model.Message{
		{ID: "MSG_000001", Sender: "PERSON_A", OriginalSender: "我", MsgTime: time.Date(2026, 6, 1, 10, 5, 0, 0, time.Local), Content: "今天中午吃什么"},
		{ID: "MSG_000002", Sender: "PERSON_B", OriginalSender: "小青", MsgTime: time.Date(2026, 6, 1, 10, 20, 0, 0, time.Local), Content: "去吃面吧"},
		{ID: "MSG_000003", Sender: "PERSON_A", OriginalSender: "我", MsgTime: time.Date(2026, 6, 1, 11, 1, 0, 0, time.Local), Content: "那我们十一点半出门"},
		{ID: "MSG_000004", Sender: "PERSON_B", OriginalSender: "小青", MsgTime: time.Date(2026, 6, 1, 11, 8, 0, 0, time.Local), Content: "可以"},
		{ID: "MSG_000005", Sender: "PERSON_A", OriginalSender: "我", MsgTime: time.Date(2026, 6, 1, 15, 5, 0, 0, time.Local), Content: "晚上看电影吗"},
	}

	preview, err := PreviewBranch(messages, "hour", time.Date(2026, 6, 1, 11, 0, 0, 0, time.Local), time.Date(2026, 6, 1, 12, 0, 0, 0, time.Local))
	if err != nil {
		t.Fatal(err)
	}
	if preview.MessageCount != 2 || len(preview.BucketIDs) != 1 {
		t.Fatalf("preview = %+v", preview)
	}
	if preview.StartTime.Hour() != 11 || preview.EndTime.Hour() != 12 {
		t.Fatalf("preview window = %s - %s", preview.StartTime, preview.EndTime)
	}
}

func TestPreviewBranchCoversEntireRequestedRange(t *testing.T) {
	messages := []model.Message{
		{ID: "MSG_000001", Sender: "PERSON_A", MsgTime: time.Date(2026, 6, 1, 10, 5, 0, 0, time.Local), Content: "第一天"},
		{ID: "MSG_000002", Sender: "PERSON_A", MsgTime: time.Date(2026, 6, 2, 10, 5, 0, 0, time.Local), Content: "第二天"},
		{ID: "MSG_000003", Sender: "PERSON_A", MsgTime: time.Date(2026, 6, 3, 10, 5, 0, 0, time.Local), Content: "第三天"},
	}

	preview, err := PreviewBranch(messages, "day", time.Date(2026, 6, 1, 0, 0, 0, 0, time.Local), time.Date(2026, 6, 4, 0, 0, 0, 0, time.Local))
	if err != nil {
		t.Fatal(err)
	}
	if preview.MessageCount != 3 || len(preview.BucketIDs) != 3 {
		t.Fatalf("preview = %+v", preview)
	}
	if !preview.StartTime.Equal(time.Date(2026, 6, 1, 0, 0, 0, 0, time.Local)) || !preview.EndTime.Equal(time.Date(2026, 6, 4, 0, 0, 0, 0, time.Local)) {
		t.Fatalf("preview range = %s - %s", preview.StartTime, preview.EndTime)
	}
}
