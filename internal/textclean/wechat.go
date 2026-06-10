package textclean

import (
	"regexp"
	"strings"
	"unicode"
)

var (
	xmlMarkerPattern    = regexp.MustCompile(`(?i)<\??xml|<msg\b|<appmsg\b|</msg>|</appmsg>`)
	xmlTagPattern       = regexp.MustCompile(`(?s)<[^>]+>`)
	cdataBoundary       = strings.NewReplacer("<![CDATA[", " ", "]]>", " ")
	spacePattern        = regexp.MustCompile(`\s+`)
	wechatNoiseKeywords = []string{
		"nickname", "appid", "appmsg", "msgid", "msgsource", "cdata", "cdnthumb", "aeskey", "thumburl",
		"fromusername", "template_id", "sourcedisplayname", "shareurl", "finder", "wxgame", "liteapp",
		"null", "undefined",
	}
)

func CleanMessageText(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	lines := strings.Split(content, "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		for _, piece := range cleanLine(line) {
			if piece != "" {
				cleaned = append(cleaned, piece)
			}
		}
	}
	return strings.TrimSpace(spacePattern.ReplaceAllString(strings.Join(cleaned, "\n"), " "))
}

func IsNaturalMessageText(content string) bool {
	content = strings.TrimSpace(CleanMessageText(content))
	if content == "" {
		return false
	}
	runes := []rune(content)
	if len(runes) < 2 {
		return false
	}
	var natural int
	for _, r := range runes {
		if unicode.Is(unicode.Han, r) || unicode.IsLetter(r) || unicode.IsDigit(r) {
			natural++
		}
	}
	if natural < 2 {
		return false
	}
	return !looksLikeStructuralNoise(content)
}

func cleanLine(line string) []string {
	line = strings.TrimSpace(cdataBoundary.Replace(line))
	if line == "" {
		return nil
	}
	if strings.HasPrefix(line, ">") && xmlMarkerPattern.MatchString(line) {
		return nil
	}
	if xmlMarkerPattern.MatchString(line) {
		return cleanXMLLine(line)
	}
	line = xmlTagPattern.ReplaceAllString(line, " ")
	line = strings.TrimSpace(spacePattern.ReplaceAllString(line, " "))
	if line == "" || looksLikeStructuralNoise(line) {
		return nil
	}
	return []string{line}
}

func cleanXMLLine(line string) []string {
	loc := xmlMarkerPattern.FindStringIndex(line)
	if loc == nil {
		return nil
	}
	prefix := strings.TrimSpace(line[:loc[0]])
	suffix := strings.TrimSpace(line[loc[0]:])
	result := make([]string, 0, 2)
	if prefix != "" && !looksLikeStructuralNoise(prefix) {
		result = append(result, prefix)
	}
	for _, value := range extractHumanXMLFields(suffix) {
		if value != "" && !looksLikeStructuralNoise(value) {
			result = append(result, value)
		}
	}
	return result
}

func extractHumanXMLFields(value string) []string {
	fields := make([]string, 0, 2)
	for _, tag := range []string{"title", "des", "description", "summary"} {
		for _, item := range extractTagValues(value, tag) {
			item = strings.TrimSpace(cdataBoundary.Replace(xmlTagPattern.ReplaceAllString(item, " ")))
			item = strings.TrimSpace(spacePattern.ReplaceAllString(item, " "))
			if item != "" && item != "null" {
				fields = append(fields, item)
			}
		}
	}
	return fields
}

func extractTagValues(value string, tag string) []string {
	open := "<" + tag + ">"
	close := "</" + tag + ">"
	result := make([]string, 0)
	rest := value
	for {
		start := strings.Index(strings.ToLower(rest), open)
		if start < 0 {
			return result
		}
		start += len(open)
		end := strings.Index(strings.ToLower(rest[start:]), close)
		if end < 0 {
			return result
		}
		result = append(result, rest[start:start+end])
		rest = rest[start+end+len(close):]
	}
}

func looksLikeStructuralNoise(value string) bool {
	lower := strings.ToLower(value)
	hits := 0
	for _, keyword := range wechatNoiseKeywords {
		if strings.Contains(lower, keyword) {
			hits++
		}
	}
	if hits >= 2 {
		return true
	}
	var punctuation int
	var natural int
	for _, r := range value {
		switch {
		case unicode.Is(unicode.Han, r), unicode.IsLetter(r), unicode.IsDigit(r):
			natural++
		case !unicode.IsSpace(r):
			punctuation++
		}
	}
	return natural > 0 && punctuation > natural*2
}
