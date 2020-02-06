package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/videa-tv/grpc-client-cli/internal/services"

	"github.com/golang/protobuf/jsonpb"
	"github.com/urfave/cli"
	"google.golang.org/grpc/health/grpc_health_v1"
)

func healthCmd(c *cli.Context) error {
	target := ""
	if c.NArg() > 0 {
		target = c.Args().First()
	}

	if target == "" {
		err := errors.New("please provide service host:port")
		fmt.Printf("Error: %s\n", err)
		return err
	}

	service := c.GlobalString("service")
	cf := services.NewGrpcConnFactory()
	defer cf.Close()
	conn, err := cf.GetConn(target)
	if err != nil {
		return err
	}

	deadline := c.GlobalInt("deadline")
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

	return m.Marshal(os.Stdout, resp)
}
