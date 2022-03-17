package rpc

import (
	"errors"
	"strings"
)

const (
	hostOpt      = "host"
	authorityOpt = "authority"
	metadataOpt  = "metadata"
)

type ConnectionOptions struct {
	Host      string
	Authority string
	Metadata  map[string][]string
}

func (co *ConnectionOptions) addMetadata(key, value string) {
	co.Metadata[key] = append(co.Metadata[key], value)
}

func NewConnectionOpts(target string) (*ConnectionOptions, error) {
	if target == "" {
		return nil, errors.New("target cannot be empty")
	}

	opts := &ConnectionOptions{
		Metadata: map[string][]string{},
	}

	tokens := strings.Split(target, ",")
	for _, token := range tokens {
		opt := strings.TrimSpace(token)
		elements := strings.SplitN(opt, "=", 2)
		if len(elements) > 1 {
			key := strings.TrimSpace(elements[0])
			value := strings.TrimSpace(elements[1])

			switch key {
			case hostOpt:
				opts.Host = value
			case authorityOpt:
				opts.Authority = value
			case metadataOpt:
				k, v := parseMetadata(value)
				opts.addMetadata(k, v)
			}
		} else {
			opts.Host = opt
		}
	}

	return opts, nil
}

func parseMetadata(val string) (string, string) {
	key, value, _ := strings.Cut(val, ":")
	return key, value
}
