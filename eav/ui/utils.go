package ui

import (
	"encoding/json"
)

func DecodeOptions(meta string, target any) error {
	if meta == "" {
		return nil
	}

	err := json.Unmarshal([]byte(meta), target)
	if err != nil {
		return err
	}
	return nil
}

func EncodeOptions(opts any) (json.RawMessage, error) {
	raw, err := json.Marshal(opts)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(raw), nil
}
