package main

import "sentipulseexam/pkg/sdk"

func main() {
	sdk.Run("success", "1.0.0", func(data map[string]interface{}) (map[string]interface{}, error) {
		return map[string]interface{}{
			"echo": data["text"],
		}, nil
	})
}
