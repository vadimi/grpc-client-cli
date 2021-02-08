package cliext

import (
	"errors"
	"strconv"
	"time"
)

func ParseDuration(val string) (time.Duration, error) {
	if val != "" {
		valSec, err := strconv.ParseInt(val, 10, 64)
		if err == nil {
			return time.Duration(valSec) * time.Second, nil
		}

		d, err := time.ParseDuration(val)
		if err != nil {
			return 0, err
		}

		return d, nil
	}

	return 0, errors.New("invalid duration")
}
