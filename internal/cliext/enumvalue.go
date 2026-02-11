package cliext

import (
	"fmt"
	"slices"
	"strings"
)

type EnumValue struct {
	Enum     []string
	Default  string
	selected string
}

func (e *EnumValue) Set(value string) error {
	if slices.Contains(e.Enum, value) {
		e.selected = value
		return nil
	}

	return fmt.Errorf("allowed values are %s", strings.Join(e.Enum, ", "))
}

func (e EnumValue) String() string {
	if e.selected == "" {
		return e.Default
	}
	return e.selected
}

func (e *EnumValue) Get() any {
	return e.selected
}
