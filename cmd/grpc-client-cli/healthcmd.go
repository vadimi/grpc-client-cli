package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/golang/protobuf/jsonpb"
	"github.com/urfave/cli/v2"
	"github.com/vadimi/grpc-client-cli/internal/rpc"
	"google.golang.org/grpc/health/grpc_health_v1"
)

func healthCmd(c *cli.Context) error {
	return checkHealth(c, os.Stdout)
}

func checkHealth(c *cli.Context, out io.Writer) error {
	target := ""
	if c.NArg() > 0 {
		target = c.Args().First()
	}

	if target == "" {
		err := errors.New("please provide service host:port")
		fmt.Printf("Error: %s\n", err)
		return err
	}

	service := c.String("service")
	cf := rpc.NewGrpcConnFactory()
	defer cf.Close()
	conn, err := cf.GetConn(target)
	if err != nil {
		return err
	}

	deadline := c.Int("deadline")
	client := grpc_health_v1.NewHealthClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(deadline)*time.Second)
	defer cancel()
	resp, err := client.Check(ctx, &grpc_health_v1.HealthCheckRequest{
		Service: service,
	})
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	m := jsonpb.Marshaler{
		EmitDefaults: true,
		Indent:       " ",
	}

	if err := m.Marshal(out, resp); err != nil {
		return err
	}

	if resp.Status != grpc_health_v1.HealthCheckResponse_SERVING {
		return cli.NewExitError("", 1)
	}

	return nil
}
