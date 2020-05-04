/*
 *
 * Copyright 2018 gRPC authors.
 * Copyright 2020 Justin Haygood
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package eureka

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	ec "github.com/ArthurHlt/go-eureka-client/eureka"
	"google.golang.org/grpc/resolver"
)

// NewEurekaBuilder creates a eurekaBuilder which is used to factory DNS resolvers.
func NewEurekaBuilder() resolver.Builder {
	return &eurekaBuilder{}
}

type eurekaBuilder struct{}

// Build creates and starts a DNS resolver that watches the name resolution of the target.
func (b *eurekaBuilder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {

	eurekaServer := target.Authority
	serviceName := target.Endpoint
	eurekaPath := ""

	if len(serviceName) == 0 {
		serviceName = eurekaServer
		eurekaServer = "localhost:8761"
		eurekaPath = "eureka"
	}

	serviceNameIndex := strings.LastIndex(serviceName, "/")

	if serviceNameIndex != -1 {
		eurekaPath = serviceName[0:serviceNameIndex]
		serviceName = serviceName[serviceNameIndex+1:]
	}

	portSeparatorIndex := strings.LastIndex(eurekaServer, ":")

	if portSeparatorIndex == -1 {
		eurekaServer = eurekaServer + ":8761"
	}

	d := &eurekaResolver{EurekaServer: eurekaServer, EurekaPath: eurekaPath, ServiceName: serviceName, ClientConn: cc}

	d.ResolveNow(resolver.ResolveNowOptions{})

	return d, nil
}

// Scheme returns the naming scheme of this resolver builder, which is "eureka".
func (b *eurekaBuilder) Scheme() string {
	return "eureka"
}

type eurekaResolver struct {
	EurekaServer string
	EurekaPath   string
	ServiceName  string
	ClientConn   resolver.ClientConn
}

// ResolveNow invoke an immediate resolution of the target that this dnsResolver watches.
func (d *eurekaResolver) ResolveNow(resolver.ResolveNowOptions) {

	eurekaURL := url.URL{Scheme: "http", Host: d.EurekaServer, Path: d.EurekaPath}

	eurekaClient := ec.NewClient([]string{
		eurekaURL.String(),
	})

	application, err := eurekaClient.GetApplication(d.ServiceName)

	if err == nil {

		var newAddrs []resolver.Address = make([]resolver.Address, 0, len(application.Instances))

		for _, instance := range application.Instances {

			port := strconv.Itoa(instance.Port.Port)

			if val, ok := instance.Metadata.Map["grpc"]; ok {
				port = val
			}

			if val, ok := instance.Metadata.Map["grpc.port"]; ok {
				port = val
			}

			addr := instance.IpAddr + ":" + port
			newAddrs = append(newAddrs, resolver.Address{Addr: addr})
		}

		if len(newAddrs) == 0 {
			err = fmt.Errorf("No address for application %v", d.ServiceName)
			d.ClientConn.ReportError(err)
		} else {
			state := &resolver.State{
				Addresses: newAddrs,
			}

			d.ClientConn.UpdateState(*state)
		}
	}

	if err != nil {
		d.ClientConn.ReportError(err)
	}

}

func (d *eurekaResolver) Close() {}
