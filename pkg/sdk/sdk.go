package sdk

import (
	"encoding/json"
	"fmt"
	"os"

	"sentipulseexam/pkg/protocol"
)

func Run(name, version string, handler func(map[string]interface{}) (map[string]interface{}, error)) {
	RunWithRequest(name, version, func(request protocol.ExecuteRequest) (map[string]interface{}, error) {
		return handler(request.Data)
	})
}

func RunWithRequest(name, version string, handler func(protocol.ExecuteRequest) (map[string]interface{}, error)) {
	decoder := json.NewDecoder(os.Stdin)
	decoder.UseNumber()

	var request protocol.ExecuteRequest
	if err := decoder.Decode(&request); err != nil {
		writeAndExit(protocol.ExecuteResponse{
			PluginName: name,
			Version:    version,
			Success:    false,
			Error:      fmt.Sprintf("decode request: %v", err),
		}, 1)
	}

	result, err := handler(request)
	response := protocol.ExecuteResponse{
		PluginName: name,
		Version:    version,
		Success:    err == nil,
		Result:     result,
	}
	if err != nil {
		response.Error = err.Error()
		writeAndExit(response, 1)
	}

	writeAndExit(response, 0)
}

func writeAndExit(response protocol.ExecuteResponse, code int) {
	encoder := json.NewEncoder(os.Stdout)
	if err := encoder.Encode(response); err != nil {
		fmt.Fprintf(os.Stderr, "encode response: %v\n", err)
		code = 1
	}
	os.Exit(code)
}
