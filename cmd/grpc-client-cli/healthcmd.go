package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/urfave/cli/v2"
	"github.com/vadimi/grpc-client-cli/internal/cliext"
	"github.com/vadimi/grpc-client-cli/internal/rpc"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/protobuf/encoding/protojson"
)

func healthCmd(c *cli.Context) error {
	return checkHealth(c, os.Stdout)
}

func checkHealth(c *cli.Context, out io.Writer) error {
	target := c.String("address")
	if target == "" {
		if c.NArg() > 0 {
			target = c.Args().First()
		}
	}

	if target == "" {
		err := errors.New("please provide service host:port")
		fmt.Printf("Error: %s\n", err)
		return err
	}

	service := c.String("service")
	tls := c.Bool("tls")
	var cf *rpc.GrpcConnFactory
	if tls {
		insecure := c.Bool("insecure")
		cACert := c.String("cacert")
		cert := c.String("cert")
		certKey := c.String("certkey")
		cf = rpc.NewGrpcConnFactory(rpc.WithConnCred(insecure, cACert, cert, certKey))
	} else {
		cf = rpc.NewGrpcConnFactory()
	}
	defer cf.Close()
	conn, err := cf.GetConn(target)
	if err != nil {
		return err
	}

	deadline, err := cliext.ParseDuration(c.String("deadline"))
	if err != nil {
		return cli.Exit(err, 1)
	}

	client := grpc_health_v1.NewHealthClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), deadline)
	defer cancel()
	resp, err := client.Check(ctx, &grpc_health_v1.HealthCheckRequest{
		Service: service,
	})
	if err != nil {
		return cli.Exit(err, 1)
	}

	m := protojson.MarshalOptions{
		EmitUnpopulated: true,
		Multiline:       true,
	}

	b, err := m.Marshal(resp)
	if err != nil {
		return err
	}

	fmt.Fprintln(out, string(b))

	if resp.Status != grpc_health_v1.HealthCheckResponse_SERVING {
		return cli.Exit("", 1)
	}

	return nil
}
