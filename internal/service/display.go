package service

import (
	"strings"

	"github.com/kevinmatthe/record_analysis/internal/model"
)

func denormalizeAnalysis(result model.AnalysisResult) model.AnalysisResult {
	aliases := participantAliases(result.Messages)
	for i := range result.Messages {
		result.Messages[i].Sender = result.Messages[i].DisplaySender()
	}
	for i := range result.Actions {
		result.Actions[i].Sender = aliasOrSelf(result.Actions[i].Sender, aliases)
		result.Actions[i].Target = aliasOrSelf(result.Actions[i].Target, aliases)
		result.Actions[i].EvidenceText = replaceAliases(result.Actions[i].EvidenceText, aliases)
	}
	for i := range result.Events {
		result.Events[i].Topic = replaceAliases(result.Events[i].Topic, aliases)
		result.Events[i].Trigger = replaceAliases(result.Events[i].Trigger, aliases)
		result.Events[i].Result = replaceAliases(result.Events[i].Result, aliases)
		result.Events[i].RepairInitiator = aliasOrSelf(result.Events[i].RepairInitiator, aliases)
		for j := range result.Events[i].Process {
			result.Events[i].Process[j] = replaceAliases(result.Events[i].Process[j], aliases)
		}
	}
	result.Report.Markdown = replaceAliases(result.Report.Markdown, aliases)
	return result
}

func participantAliases(messages []model.Message) map[string]string {
	aliases := map[string]string{}
	for _, message := range messages {
		if message.Sender == "" {
			continue
		}
		aliases[message.Sender] = message.DisplaySender()
	}
	if _, ok := aliases["PERSON_A"]; !ok {
		aliases["PERSON_A"] = "我"
	}
	return aliases
}

func aliasOrSelf(value string, aliases map[string]string) string {
	if alias, ok := aliases[value]; ok && alias != "" {
		return alias
	}
	return value
}

func replaceAliases(text string, aliases map[string]string) string {
	if text == "" || len(aliases) == 0 {
		return text
	}
	pairs := make([]string, 0, len(aliases)*2)
	for _, key := range []string{"PERSON_A", "PERSON_B", "PERSON_2", "PERSON_3", "PERSON_4"} {
		if value, ok := aliases[key]; ok && value != "" {
			pairs = append(pairs, key, value)
		}
	}
	for key, value := range aliases {
		if !strings.HasPrefix(key, "PERSON_") || value == "" {
			continue
		}
		pairs = append(pairs, key, value)
	}
	if len(pairs) == 0 {
		return text
	}
	return strings.NewReplacer(pairs...).Replace(text)
}
