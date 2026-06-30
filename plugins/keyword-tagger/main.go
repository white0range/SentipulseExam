package main

import (
	"fmt"
	"strings"

	"sentipulseexam/pkg/sdk"
)

const (
	pluginName    = "keyword-tagger"
	pluginVersion = "1.0.0"
)

func main() {
	sdk.Run(pluginName, pluginVersion, func(data map[string]interface{}) (map[string]interface{}, error) {
		text, ok := data["text"].(string)
		if !ok || strings.TrimSpace(text) == "" {
			return nil, fmt.Errorf("field %q must be a non-empty string", "text")
		}

		rawKeywords, ok := data["keywords"]
		if !ok {
			return nil, fmt.Errorf("field %q must be provided", "keywords")
		}

		keywords, err := normalizeKeywords(rawKeywords)
		if err != nil {
			return nil, err
		}

		matches := make([]string, 0)
		lowerText := strings.ToLower(text)
		for _, keyword := range keywords {
			if strings.Contains(lowerText, strings.ToLower(keyword)) {
				matches = append(matches, keyword)
			}
		}

		return map[string]interface{}{
			"matches":     matches,
			"match_count": len(matches),
		}, nil
	})
}

func normalizeKeywords(value interface{}) ([]string, error) {
	items, ok := value.([]interface{})
	if !ok {
		return nil, fmt.Errorf("field %q must be an array of strings", "keywords")
	}

	keywords := make([]string, 0, len(items))
	for _, item := range items {
		keyword, ok := item.(string)
		if !ok || strings.TrimSpace(keyword) == "" {
			return nil, fmt.Errorf("field %q must contain non-empty strings", "keywords")
		}
		keywords = append(keywords, keyword)
	}
	return keywords, nil
}
