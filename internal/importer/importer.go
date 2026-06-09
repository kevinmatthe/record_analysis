package importer

import (
	"bufio"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/kevinmatthe/record_analysis/internal/model"
)

var txtMessagePattern = regexp.MustCompile(`^\[([^\]]+)\]\s+([^:：]+)[:：]\s?(.*)$`)

type rawMessage struct {
	Time      string      `json:"time"`
	Timestamp interface{} `json:"timestamp"`
	Sender    string      `json:"sender"`
	Type      interface{} `json:"type"`
	TypeName  string      `json:"type_name"`
	Content   string      `json:"content"`
}

func ParseChatFile(path string, relationshipID string, includeSystem bool) ([]model.Message, error) {
	ext := strings.ToLower(filepath.Ext(path))
	var rows []rawMessage
	var err error
	switch ext {
	case ".txt":
		rows, err = parseTXT(path)
	case ".csv":
		rows, err = parseCSV(path)
	case ".json":
		rows, err = parseJSON(path)
	default:
		return nil, fmt.Errorf("unsupported chat file format: %s", ext)
	}
	if err != nil {
		return nil, err
	}
	messages, err := normalizeRows(rows, relationshipID, strings.TrimPrefix(ext, "."))
	if err != nil {
		return nil, err
	}
	if includeSystem {
		return messages, nil
	}
	filtered := messages[:0]
	for _, message := range messages {
		if message.Sender == "PERSON_A" || message.Sender == "PERSON_B" {
			filtered = append(filtered, message)
		}
	}
	return filtered, nil
}

func parseTXT(path string) ([]rawMessage, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var rows []rawMessage
	var current *rawMessage
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024), 1024*1024*8)
	for scanner.Scan() {
		line := strings.TrimPrefix(scanner.Text(), "\ufeff")
		match := txtMessagePattern.FindStringSubmatch(line)
		if len(match) == 4 {
			rows = append(rows, rawMessage{Time: match[1], Sender: strings.TrimSpace(match[2]), TypeName: "文本", Content: match[3]})
			current = &rows[len(rows)-1]
			continue
		}
		if current != nil && line != "" && !isTXTHeader(line) {
			current.Content += "\n" + line
		}
	}
	return rows, scanner.Err()
}

func parseCSV(path string) ([]rawMessage, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, nil
	}
	header := make(map[string]int)
	for index, name := range records[0] {
		header[strings.TrimPrefix(name, "\ufeff")] = index
	}
	var rows []rawMessage
	for _, record := range records[1:] {
		rows = append(rows, rawMessage{
			Time:     csvField(record, header, "时间"),
			Sender:   csvField(record, header, "发送者"),
			TypeName: csvField(record, header, "类型"),
			Content:  csvField(record, header, "内容"),
		})
	}
	return rows, nil
}

func parseJSON(path string) ([]rawMessage, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var rows []rawMessage
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func normalizeRows(rows []rawMessage, relationshipID string, source string) ([]model.Message, error) {
	senderMap := map[string]string{}
	messages := make([]model.Message, 0, len(rows))
	for _, row := range rows {
		msgTime, err := parseTime(row)
		if err != nil {
			return nil, err
		}
		sender := normalizeSender(strings.TrimSpace(row.Sender), senderMap)
		content := strings.TrimSpace(row.Content)
		msgType := row.TypeName
		if msgType == "" && row.Type != nil {
			msgType = fmt.Sprint(row.Type)
		}
		if msgType == "" {
			msgType = "unknown"
		}
		if !isMeaningfulText(content, msgType) {
			continue
		}
		messages = append(messages, model.Message{
			ID:             model.StableMessageID(len(messages) + 1),
			RelationshipID: relationshipID,
			Sender:         sender,
			OriginalSender: strings.TrimSpace(row.Sender),
			MsgTime:        msgTime,
			MsgType:        msgType,
			Content:        content,
			RawContent:     content,
			Source:         source,
			ContentHash:    contentHash(sender, msgTime, content),
		})
	}
	return messages, nil
}

func parseTime(row rawMessage) (time.Time, error) {
	if row.Timestamp != nil {
		switch value := row.Timestamp.(type) {
		case float64:
			return time.Unix(int64(value), 0), nil
		case string:
			if value != "" {
				unix, err := strconv.ParseInt(value, 10, 64)
				if err == nil {
					return time.Unix(unix, 0), nil
				}
			}
		}
	}
	for _, layout := range []string{"2006-01-02 15:04:05", "2006/01/02 15:04:05"} {
		if t, err := time.ParseInLocation(layout, strings.TrimSpace(row.Time), time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, errors.New("unsupported message time: " + row.Time)
}

func normalizeSender(sender string, senderMap map[string]string) string {
	if sender == "系统" {
		return "系统"
	}
	if sender == "我" {
		return "PERSON_A"
	}
	if sender == "" {
		return "UNKNOWN"
	}
	if existing, ok := senderMap[sender]; ok {
		return existing
	}
	if len(senderMap) == 0 {
		senderMap[sender] = "PERSON_B"
	} else {
		senderMap[sender] = fmt.Sprintf("PERSON_%d", len(senderMap)+2)
	}
	return senderMap[sender]
}

func isMeaningfulText(content string, msgType string) bool {
	content = strings.TrimSpace(content)
	if content == "" {
		return false
	}
	normalizedType := strings.ToLower(strings.TrimSpace(msgType))
	if normalizedType != "" && normalizedType != "文本" && normalizedType != "text" && normalizedType != "unknown" {
		return false
	}
	lower := strings.ToLower(content)
	switch lower {
	case "[图片]", "[image]", "[表情]", "[动画表情]", "[语音]", "[视频]", "[文件]", "[位置]", "[链接]", "[转账]", "[红包]", "[聊天记录]", "<图片>", "<表情>", "<语音>":
		return false
	}
	if strings.Contains(content, "撤回了一条消息") ||
		strings.Contains(content, "你撤回了一条消息") ||
		strings.Contains(content, "对方已取消") ||
		strings.Contains(content, "请在手机上查看") {
		return false
	}
	return true
}

func contentHash(sender string, msgTime time.Time, content string) string {
	sum := sha256.Sum256([]byte(sender + "|" + msgTime.Format(time.RFC3339) + "|" + content))
	return hex.EncodeToString(sum[:])
}

func csvField(record []string, header map[string]int, name string) string {
	index, ok := header[name]
	if !ok || index >= len(record) {
		return ""
	}
	return record[index]
}

func isTXTHeader(line string) bool {
	return strings.HasPrefix(line, "微信聊天记录") ||
		strings.HasPrefix(line, "总消息数") ||
		strings.HasPrefix(line, "时间范围") ||
		strings.HasPrefix(line, "====")
}
