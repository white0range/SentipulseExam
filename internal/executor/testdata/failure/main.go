package main

import (
	"errors"

	"sentipulseexam/pkg/sdk"
)

func main() {
	sdk.Run("failure", "1.0.0", func(data map[string]interface{}) (map[string]interface{}, error) {
		return nil, errors.New("forced failure")
	})
}
