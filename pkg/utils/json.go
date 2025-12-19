package utils

import (
	"encoding/json"
	"errors"
)

func Bytes2Data[T any](bytes []byte) (T, error) {
	var result T
	err := json.Unmarshal(bytes, &result)
	if err != nil {
		return result, errors.New("failed to unmarshal JSON: " + err.Error())
	}
	return result, nil
}
