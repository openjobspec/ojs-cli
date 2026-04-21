package commands

import (
	"encoding/json"
	"fmt"

	"github.com/openjobspec/ojs-cli/internal/output"
)

func decodeResponse(data []byte, target any) error {
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}
	return nil
}

func printJSONResponse(data []byte) error {
	var result any
	if err := decodeResponse(data, &result); err != nil {
		return err
	}
	return output.JSON(result)
}

func formatJSONValue(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("format JSON value: %w", err)
	}
	return string(data), nil
}
