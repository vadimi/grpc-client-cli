package rpc

import (
	"log"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"golang.org/x/net/context"

	"github.com/vadimi/grpc-client-cli/internal/resolver/eureka"
	"google.golang.org/grpc"
	"google.golang.org/grpc/balancer/roundrobin"
	_ "google.golang.org/grpc/encoding/gzip" // register gzip compressor
	"google.golang.org/grpc/resolver"
)

func init() {
	// TODO: remove that line when dns is default resolver
	resolver.SetDefaultScheme("dns")
	resolver.Register(eureka.NewEurekaBuilder())
}

type connMeta struct {
	sync.Once
	conn    *grpc.ClientConn
	dialErr error
}

type GrpcConnFactory struct {
	conns struct {
		sync.Mutex
		cache map[string]*connMeta
	}
}

type dialFunc func(target string, opts ...grpc.DialOption) (*grpc.ClientConn, error)

func NewGrpcConnFactory() *GrpcConnFactory {
	f := &GrpcConnFactory{}
	f.conns.cache = map[string]*connMeta{}
	return f
}

func (f *GrpcConnFactory) GetConnContext(ctx context.Context, target string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	return f.getConn(target, func(target string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
		return grpc.DialContext(ctx, target, opts...)
	}, opts...)
}

func (f *GrpcConnFactory) GetConn(target string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	return f.getConn(target, func(target string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
		return grpc.Dial(target, opts...)
	})
}

func (f *GrpcConnFactory) getConn(target string, dial dialFunc, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	connOpts, err := NewConnectionOpts(target)
	if err != nil {
		return nil, err
	}
	f.conns.Lock()
	conn, ok := f.conns.cache[connOpts.Host]
	if !ok {
		conn = &connMeta{}
		f.conns.cache[connOpts.Host] = conn
	}
	f.conns.Unlock()

	conn.Do(func() {
		opts := append(opts,
			grpc.WithDisableServiceConfig(),
			grpc.WithInsecure(),
			grpc.WithBalancerName(roundrobin.Name),
		)

		svcTarget := connOpts.Host

		// if we have a proxy use it as our service target and pass original target to :authority header
		if connOpts.Authority != "" {
			svcTarget = connOpts.Authority
			opts = append(opts, grpc.WithAuthority(connOpts.Host))
		}

		unaryInterceptors := []grpc.UnaryClientInterceptor{
			DiagUnaryClientInterceptor(),
		}

		streamInterceptors := []grpc.StreamClientInterceptor{
			DiagStreamClientInterceptor(),
		}

		if len(connOpts.Metadata) > 0 {
			unaryInterceptors = append(unaryInterceptors,
				MetadataUnaryInterceptor(connOpts.Metadata),
			)

			streamInterceptors = append(streamInterceptors,
				MetadataStreamInterceptor(connOpts.Metadata),
			)
		}

		opts = append(opts,
			grpc.WithChainUnaryInterceptor(unaryInterceptors...),
			grpc.WithChainStreamInterceptor(streamInterceptors...))

		conn.conn, conn.dialErr = dial(svcTarget, opts...)
		if conn.dialErr != nil {
			log.Println(conn.dialErr)
		}
	})

	return conn.conn, conn.dialErr
}

func (f *GrpcConnFactory) Close() error {
	f.conns.Lock()
	defer f.conns.Unlock()

	resultErr := []string{}
	for _, connMeta := range f.conns.cache {
		err := connMeta.conn.Close()
		if err != nil {
			resultErr = append(resultErr, err.Error())
		}
	}

	if len(resultErr) > 0 {
		msg := strings.Join(resultErr, ": ")
		return errors.New(msg)
	}

	return nil
}

func (f *GrpcConnFactory) CloseConn(target string) error {
	f.conns.Lock()
	defer f.conns.Unlock()

	connOpts, err := NewConnectionOpts(target)
	if err != nil {
		return err
	}
	conn, ok := f.conns.cache[connOpts.Host]
	if ok {
		err = conn.conn.Close()
		delete(f.conns.cache, connOpts.Host)
	}
	return err
}
