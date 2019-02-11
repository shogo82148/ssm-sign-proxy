package proxy

import "encoding/json"

type lambdaError map[string]interface{}

func (e lambdaError) Error() string {
	if msg, ok := e["errorMessage"].(string); ok {
		return msg
	}
	data, _ := json.Marshal(e)
	return string(data)
}

func parseError(payload []byte) error {
	var e lambdaError
	if err := json.Unmarshal(payload, &e); err != nil {
		return err
	}
	return e
}
