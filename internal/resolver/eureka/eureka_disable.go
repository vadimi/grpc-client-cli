//go:build !eureka

package eureka

import (
	"google.golang.org/grpc/resolver"
)

func NewEurekaBuilder() resolver.Builder {
	return nil
}
