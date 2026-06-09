package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/kevinmatthe/record_analysis/internal/model"
)

type TimelineBucket struct {
	ID                  string         `json:"id"`
	Granularity         string         `json:"granularity"`
	StartTime           time.Time      `json:"start_time"`
	EndTime             time.Time      `json:"end_time"`
	MessageCount        int            `json:"message_count"`
	ParticipantCount    int            `json:"participant_count"`
	ParticipantMessages map[string]int `json:"participant_messages"`
	FirstMessageID      string         `json:"first_message_id"`
	LastMessageID       string         `json:"last_message_id"`
	Preview             string         `json:"preview"`
	AnalysisStatus      string         `json:"analysis_status,omitempty"`
	WordCloudStatus     string         `json:"word_cloud_status,omitempty"`
	SummaryStatus       string         `json:"summary_status,omitempty"`
	SummaryTitle        string         `json:"summary_title,omitempty"`
	SummaryTopics       []string       `json:"summary_topics,omitempty"`
	TotalTokens         int            `json:"total_tokens,omitempty"`
}

type TimelineCluster struct {
	ID           string    `json:"id"`
	Granularity  string    `json:"granularity"`
	StartTime    time.Time `json:"start_time"`
	EndTime      time.Time `json:"end_time"`
	MessageCount int       `json:"message_count"`
	BucketCount  int       `json:"bucket_count"`
	BucketIDs    []string  `json:"bucket_ids"`
	TopicHint    string    `json:"topic_hint"`
	Status       string    `json:"status"`
}

type BranchPreview struct {
	Granularity  string    `json:"granularity"`
	StartTime    time.Time `json:"start_time"`
	EndTime      time.Time `json:"end_time"`
	MessageCount int       `json:"message_count"`
	BucketIDs    []string  `json:"bucket_ids"`
	ClusterID    string    `json:"cluster_id"`
	TopicHint    string    `json:"topic_hint"`
	Status       string    `json:"status"`
}

func BuildTimelineBuckets(messages []model.Message, granularity string) ([]TimelineBucket, error) {
	granularity = normalizeGranularity(granularity)
	if granularity == "" {
		return nil, fmt.Errorf("unsupported granularity")
	}
	if len(messages) == 0 {
		return nil, nil
	}

	buckets := make([]TimelineBucket, 0)
	var current *TimelineBucket
	for _, message := range messages {
		start := bucketStart(message.MsgTime, granularity)
		end := bucketEnd(start, granularity)
		id := timelineBucketID(start, granularity)
		if current == nil || current.ID != id {
			if current != nil {
				buckets = append(buckets, *current)
			}
			current = &TimelineBucket{
				ID:                  id,
				Granularity:         granularity,
				StartTime:           start,
				EndTime:             end,
				ParticipantMessages: map[string]int{},
				FirstMessageID:      message.ID,
				LastMessageID:       message.ID,
				Preview:             shortPreview(message.Content),
			}
		}
		sender := message.DisplaySender()
		current.MessageCount++
		current.ParticipantMessages[sender]++
		current.ParticipantCount = len(current.ParticipantMessages)
		current.LastMessageID = message.ID
	}
	if current != nil {
		buckets = append(buckets, *current)
	}
	return buckets, nil
}

func FilterMessagesByTimeRange(messages []model.Message, start *time.Time, end *time.Time) []model.Message {
	if start == nil && end == nil {
		return messages
	}
	filtered := make([]model.Message, 0, len(messages))
	for _, message := range messages {
		if start != nil && message.MsgTime.Before(*start) {
			continue
		}
		if end != nil && !message.MsgTime.Before(*end) {
			continue
		}
		filtered = append(filtered, message)
	}
	return filtered
}

func FilterMessagesForBucket(messages []model.Message, bucketID string) ([]model.Message, error) {
	start, granularity, err := parseTimelineBucketID(bucketID)
	if err != nil {
		return nil, err
	}
	end := bucketEnd(start, granularity)
	filtered := make([]model.Message, 0)
	for _, message := range messages {
		if !message.MsgTime.Before(start) && message.MsgTime.Before(end) {
			filtered = append(filtered, message)
		}
	}
	return filtered, nil
}

func BuildTimelineClusters(buckets []TimelineBucket) []TimelineCluster {
	if len(buckets) == 0 {
		return nil
	}
	clusters := make([]TimelineCluster, 0)
	var current *TimelineCluster
	for _, bucket := range buckets {
		if current == nil || !shouldMergeCluster(*current, bucket) {
			if current != nil {
				clusters = append(clusters, *current)
			}
			current = &TimelineCluster{
				ID:           fmt.Sprintf("cluster_%03d", len(clusters)+1),
				Granularity:  bucket.Granularity,
				StartTime:    bucket.StartTime,
				EndTime:      bucket.EndTime,
				MessageCount: bucket.MessageCount,
				BucketCount:  1,
				BucketIDs:    []string{bucket.ID},
				TopicHint:    timelineBucketTopicHint(bucket),
				Status:       timelineBucketStatus(bucket),
			}
			continue
		}
		current.EndTime = bucket.EndTime
		current.MessageCount += bucket.MessageCount
		current.BucketCount++
		current.BucketIDs = append(current.BucketIDs, bucket.ID)
		hint := timelineBucketTopicHint(bucket)
		if current.TopicHint == "" {
			current.TopicHint = hint
		} else if hint != "" && !strings.Contains(current.TopicHint, hint) {
			current.TopicHint = shortPreview(current.TopicHint + " / " + hint)
		}
		current.Status = mergeTimelineStatus(current.Status, timelineBucketStatus(bucket))
	}
	if current != nil {
		clusters = append(clusters, *current)
	}
	return clusters
}

func timelineBucketTopicHint(bucket TimelineBucket) string {
	if bucket.SummaryTitle != "" {
		return bucket.SummaryTitle
	}
	return bucket.Preview
}

func timelineBucketStatus(bucket TimelineBucket) string {
	if bucket.AnalysisStatus != "" {
		return bucket.AnalysisStatus
	}
	return "unseen"
}

func mergeTimelineStatus(current string, next string) string {
	rank := map[string]int{
		"running":   5,
		"queued":    4,
		"failed":    3,
		"completed": 2,
		"unseen":    1,
		"":          0,
	}
	if rank[next] > rank[current] {
		return next
	}
	if current == "" {
		return "unseen"
	}
	return current
}

func PreviewBranch(messages []model.Message, granularity string, start time.Time, end time.Time) (BranchPreview, error) {
	buckets, err := BuildTimelineBuckets(messages, granularity)
	if err != nil {
		return BranchPreview{}, err
	}
	clusters := BuildTimelineClusters(buckets)
	for _, cluster := range clusters {
		if rangesOverlap(cluster.StartTime, cluster.EndTime, start, end) {
			return BranchPreview{
				Granularity:  granularity,
				StartTime:    cluster.StartTime,
				EndTime:      cluster.EndTime,
				MessageCount: cluster.MessageCount,
				BucketIDs:    append([]string(nil), cluster.BucketIDs...),
				ClusterID:    cluster.ID,
				TopicHint:    cluster.TopicHint,
				Status:       cluster.Status,
			}, nil
		}
	}
	return BranchPreview{
		Granularity: granularity,
		StartTime:   start,
		EndTime:     end,
		Status:      "unseen",
	}, nil
}

func normalizeGranularity(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "hour":
		return "hour"
	case "year":
		return "year"
	case "month":
		return "month"
	case "week":
		return "week"
	case "day":
		return "day"
	case "15m":
		return "15m"
	case "5m":
		return "5m"
	default:
		return ""
	}
}

func timelineBucketID(start time.Time, granularity string) string {
	return fmt.Sprintf("%s_%s", granularity, start.Format(time.RFC3339))
}

func parseTimelineBucketID(value string) (time.Time, string, error) {
	parts := strings.SplitN(value, "_", 2)
	if len(parts) != 2 {
		return time.Time{}, "", fmt.Errorf("invalid bucket id")
	}
	granularity := normalizeGranularity(parts[0])
	if granularity == "" {
		return time.Time{}, "", fmt.Errorf("invalid bucket granularity")
	}
	start, err := time.Parse(time.RFC3339, parts[1])
	if err != nil {
		return time.Time{}, "", fmt.Errorf("invalid bucket start")
	}
	return start, granularity, nil
}

func bucketStart(ts time.Time, granularity string) time.Time {
	switch granularity {
	case "year":
		return time.Date(ts.Year(), 1, 1, 0, 0, 0, 0, ts.Location())
	case "month":
		return time.Date(ts.Year(), ts.Month(), 1, 0, 0, 0, 0, ts.Location())
	case "week":
		weekday := int(ts.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		start := ts.AddDate(0, 0, -(weekday - 1))
		return time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, ts.Location())
	case "day":
		return time.Date(ts.Year(), ts.Month(), ts.Day(), 0, 0, 0, 0, ts.Location())
	case "15m":
		return time.Date(ts.Year(), ts.Month(), ts.Day(), ts.Hour(), (ts.Minute()/15)*15, 0, 0, ts.Location())
	case "5m":
		return time.Date(ts.Year(), ts.Month(), ts.Day(), ts.Hour(), (ts.Minute()/5)*5, 0, 0, ts.Location())
	default:
		return time.Date(ts.Year(), ts.Month(), ts.Day(), ts.Hour(), 0, 0, 0, ts.Location())
	}
}

func bucketEnd(start time.Time, granularity string) time.Time {
	switch granularity {
	case "year":
		return start.AddDate(1, 0, 0)
	case "month":
		return start.AddDate(0, 1, 0)
	case "week":
		return start.AddDate(0, 0, 7)
	case "day":
		return start.Add(24 * time.Hour)
	case "15m":
		return start.Add(15 * time.Minute)
	case "5m":
		return start.Add(5 * time.Minute)
	default:
		return start.Add(time.Hour)
	}
}

func shortPreview(content string) string {
	content = strings.TrimSpace(content)
	runes := []rune(content)
	if len(runes) > 48 {
		return string(runes[:48]) + "..."
	}
	return content
}

func shouldMergeCluster(cluster TimelineCluster, bucket TimelineBucket) bool {
	maxGap := bucket.EndTime.Sub(bucket.StartTime)
	if maxGap <= 0 {
		maxGap = bucketEnd(bucket.StartTime, bucket.Granularity).Sub(bucket.StartTime)
	}
	if maxGap <= 0 {
		maxGap = time.Hour
	}
	if bucket.StartTime.Sub(cluster.EndTime) > maxGap {
		return false
	}
	return true
}

func rangesOverlap(aStart time.Time, aEnd time.Time, bStart time.Time, bEnd time.Time) bool {
	return aStart.Before(bEnd) && bStart.Before(aEnd)
}
