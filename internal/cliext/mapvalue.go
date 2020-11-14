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

type MapValue struct {
	m     map[string][]string
	isSet bool
}

func NewMapValue() *MapValue {
	return &MapValue{}
}

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
	value := strings.TrimSpace(tokens[1])

	values, ok := mv.m[key]
	if !ok {
		values = []string{}
	}

	values = append(values, value)
	mv.m[key] = values

	return nil
}

func (mv *MapValue) Serialize() string {
	jsonBytes, _ := json.Marshal(mv.m)
	return fmt.Sprintf("%s%s", slPfx, string(jsonBytes))
}

func (mv MapValue) String() string {
	return fmt.Sprint(mv.m)
}

func ParseMapValue(val interface{}) map[string][]string {
	if mval, ok := val.(*MapValue); ok {
		return mval.m
	}

	return nil
}
