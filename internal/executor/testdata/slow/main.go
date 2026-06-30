package main

import (
	"time"

	"sentipulseexam/pkg/sdk"
)

func main() {
	sdk.Run("slow", "1.0.0", func(data map[string]interface{}) (map[string]interface{}, error) {
		time.Sleep(200 * time.Millisecond)
		return map[string]interface{}{
			"ok": true,
		}, nil
	})
}
