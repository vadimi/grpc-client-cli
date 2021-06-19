package rpc

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/net/context"

	"github.com/vadimi/grpc-client-cli/internal/resolver/eureka"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	_ "google.golang.org/grpc/encoding/gzip" // register gzip compressor
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/resolver"
)

// round_robin loadbalaning policy
// see https://github.com/grpc/proposal/blob/master/A24-lb-policy-config.md
const loadBalancer = `{"loadBalancingConfig": [{"round_robin": {}}]}`

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

type GrpcConnFactorySettings struct {
	tls            bool
	insecure       bool
	caCert         string
	cert           string
	certKey        string
	authority      string
	headers        map[string][]string
	keepalive      bool
	keepaliveTime  time.Duration
	maxRecvMsgSize int
}

type GrpcConnFactory struct {
	settings *GrpcConnFactorySettings
	conns    struct {
		sync.Mutex
		cache map[string]*connMeta
	}
}

type dialFunc func(target string, opts ...grpc.DialOption) (*grpc.ClientConn, error)

type ConnFactoryOption func(*GrpcConnFactorySettings)

func WithConnCred(insecure bool, caCert string, cert string, certKey string) ConnFactoryOption {
	return func(s *GrpcConnFactorySettings) {
		s.tls = true
		s.caCert = caCert
		s.cert = cert
		s.certKey = certKey
		s.maxRecvMsgSize = 0
	}
}

func WithAuthority(authority string) ConnFactoryOption {
	return func(s *GrpcConnFactorySettings) {
		s.authority = authority
	}
}

func WithHeaders(h map[string][]string) ConnFactoryOption {
	return func(s *GrpcConnFactorySettings) {
		s.headers = h
	}
}

func WithKeepalive(keepalive bool, keepaliveTime time.Duration) ConnFactoryOption {
	return func(s *GrpcConnFactorySettings) {
		s.keepalive = keepalive
		s.keepaliveTime = keepaliveTime
	}
}

func WithMaxRecvMsgSize(messageSize int) ConnFactoryOption {
	return func(s *GrpcConnFactorySettings) {
		s.maxRecvMsgSize = messageSize
	}
}

func NewGrpcConnFactory(opts ...ConnFactoryOption) *GrpcConnFactory {
	settings := &GrpcConnFactorySettings{}

	for _, o := range opts {
		o(settings)
	}

	f := &GrpcConnFactory{
		settings: settings,
	}
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
			grpc.WithDefaultServiceConfig(loadBalancer),
			grpc.WithStatsHandler(newStatsHanler()),
		)

		authority := connOpts.Authority

		if f.settings.authority != "" {
			authority = f.settings.authority
		}

		if !f.settings.tls {
			opts = append(opts, grpc.WithInsecure())

			// if we have a proxy use it as our service target and pass original target to :authority header
			// override authority for non TLS connection only
			if authority != "" {
				opts = append(opts, grpc.WithAuthority(authority))
			}
		} else {
			creds, err := getCredentials(f.settings.insecure, f.settings.caCert, f.settings.cert, f.settings.certKey)
			if err != nil {
				conn.dialErr = err
				return
			}
			if authority != "" {
				if err := creds.OverrideServerName(authority); err != nil {
					conn.dialErr = err
					return
				}
			}
			opts = append(opts, grpc.WithTransportCredentials(creds))
		}

		if f.settings.keepalive {
			ka := keepalive.ClientParameters{
				PermitWithoutStream: true,
			}

			if f.settings.keepaliveTime > 0 {
				ka.Time = f.settings.keepaliveTime
			}

			opts = append(opts, grpc.WithKeepaliveParams(ka))
		}

		if f.settings.maxRecvMsgSize > 0 {
			opts = append(opts, grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(f.settings.maxRecvMsgSize)))
		}

		unaryInterceptors := []grpc.UnaryClientInterceptor{}
		streamInterceptors := []grpc.StreamClientInterceptor{}

		md := f.metadata(connOpts.Metadata)

		if len(md) > 0 {
			unaryInterceptors = append(unaryInterceptors,
				MetadataUnaryInterceptor(md),
			)

			streamInterceptors = append(streamInterceptors,
				MetadataStreamInterceptor(md),
			)
		}

		opts = append(opts,
			grpc.WithChainUnaryInterceptor(unaryInterceptors...),
			grpc.WithChainStreamInterceptor(streamInterceptors...))

		conn.conn, conn.dialErr = dial(connOpts.Host, opts...)
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

func (f *GrpcConnFactory) metadata(connOptsMd map[string][]string) map[string][]string {
	var mds []metadata.MD
	if f.settings.headers != nil {
		mds = append(mds, f.settings.headers)
	}

	if len(connOptsMd) > 0 {
		mds = append(mds, connOptsMd)
	}

	return metadata.Join(mds...)
}

func getCredentials(insecure bool, caCert, cert, certKey string) (credentials.TransportCredentials, error) {
	var tlsCfg tls.Config
	if insecure {
		tlsCfg.InsecureSkipVerify = true
	} else if caCert != "" {
		b, err := ioutil.ReadFile(caCert)
		if err != nil {
			return nil, errors.Wrap(err, "failed to read the CA certificate")
		}
		cp := x509.NewCertPool()
		if !cp.AppendCertsFromPEM(b) {
			return nil, errors.New("failed to append the client certificate")
		}
		tlsCfg.RootCAs = cp
	}

	if cert != "" && certKey != "" {
		certificate, err := tls.LoadX509KeyPair(cert, certKey)
		if err != nil {
			return nil, errors.Wrap(err, "failed to read the client certificate")
		}
		tlsCfg.Certificates = append(tlsCfg.Certificates, certificate)
	} else if cert != "" || certKey != "" {
		return nil, errors.New("both cert and certKey need to be specified")
	}

	return credentials.NewTLS(&tlsCfg), nil
}
