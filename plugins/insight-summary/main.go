package main

import (
	"fmt"
	"strings"

	"sentipulseexam/pkg/protocol"
	"sentipulseexam/pkg/sdk"
)

const (
	pluginName    = "insight-summary"
	pluginVersion = "1.0.0"
)

func main() {
	sdk.RunWithRequest(pluginName, pluginVersion, func(request protocol.ExecuteRequest) (map[string]interface{}, error) {
		wordCountResult, ok := request.Context.DependencyResults["wordcount"]
		if !ok {
			return nil, fmt.Errorf("missing dependency result for %q", "wordcount")
		}

		wordCount, ok := wordCountResult.Result["word_count"]
		if !ok {
			return nil, fmt.Errorf("dependency %q did not provide word_count", "wordcount")
		}

		summary := fmt.Sprintf(
			"Text processing completed successfully with word_count=%v.",
			wordCount,
		)

		text, _ := request.Data["text"].(string)
		if strings.Contains(strings.ToLower(text), "bugs") {
			summary += " The input mentions bugs, so follow-up review may be useful."
		}

		result := map[string]interface{}{
			"summary": summary,
		}

		if keywordResult, ok := request.Context.DependencyResults["keyword-tagger"]; ok {
			if matches, exists := keywordResult.Result["matches"]; exists {
				result["keyword_matches"] = matches
				result["summary"] = summary + " Optional keyword-tagging data was also included."
			}
		}

		return result, nil
	})
}
