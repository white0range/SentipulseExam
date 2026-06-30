package main

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"sentipulseexam/pkg/sdk"
)

const (
	pluginName    = "wordcount"
	pluginVersion = "1.0.0"
)

func main() {
	sdk.Run(pluginName, pluginVersion, func(data map[string]interface{}) (map[string]interface{}, error) {
		text, err := readText(data)
		if err != nil {
			return nil, err
		}

		lines := 0
		if text != "" {
			lines = strings.Count(text, "\n") + 1
		}

		return map[string]interface{}{
			"word_count":      len(strings.Fields(text)),
			"line_count":      lines,
			"character_count": utf8.RuneCountInString(text),
		}, nil
	})
}

func readText(data map[string]interface{}) (string, error) {
	text, ok := data["text"].(string)
	if !ok || strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("field %q must be a non-empty string", "text")
	}
	return text, nil
}
