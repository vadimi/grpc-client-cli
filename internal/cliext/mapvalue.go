package cliext

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	slPfx = fmt.Sprintf("sl:::%d:::", time.Now().UTC().UnixNano())
)

// MapValue allows passing multiple key/value pairs from the command line args.
// This is an extension for github.com/urfave/cli
type MapValue struct {
	m     map[string][]string
	isSet bool
}

func NewMapValue() *MapValue {
	return &MapValue{}
}

// Set parses and stored key/value to the map in "a: b" format
func (mv *MapValue) Set(param string) error {
	if strings.HasPrefix(param, slPfx) {
		// Deserializing assumes overwrite
		_ = json.Unmarshal([]byte(strings.Replace(param, slPfx, "", 1)), &mv.m)
		mv.isSet = true
		return nil
	}

	if !mv.isSet {
		mv.m = map[string][]string{}
		mv.isSet = true
	}

	tokens := strings.SplitN(param, ":", 2)

	if len(tokens) != 2 {
		return errors.New("please use \"key: value\" format")
	}

	key := strings.TrimSpace(tokens[0])
	if key == "" {
		return errors.New("key cannot be empty")
	}

	value := strings.TrimSpace(tokens[1])
	if value == "" {
		return errors.New("value cannot be empty")
	}

	values, ok := mv.m[key]
	if !ok {
		values = []string{}
	}

	values = append(values, value)
	mv.m[key] = values

	return nil
}

// Serialize method is used internally by github.com/urfave/cli
func (mv *MapValue) Serialize() string {
	jsonBytes, _ := json.Marshal(mv.m)
	return fmt.Sprintf("%s%s", slPfx, string(jsonBytes))
}

func (mv MapValue) String() string {
	return fmt.Sprint(mv.m)
}

// ParseMapValue returns map from the interface object
func ParseMapValue(val interface{}) map[string][]string {
	if mval, ok := val.(*MapValue); ok {
		return mval.m
	}

	return nil
}
