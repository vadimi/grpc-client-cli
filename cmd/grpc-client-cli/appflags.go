package main

import (
	"github.com/urfave/cli/v2"
	"github.com/urfave/cli/v2/altsrc"
	"github.com/vadimi/grpc-client-cli/internal/cliext"
)

const flagConfigFile = "config"

func appFlags() []cli.Flag {
	return []cli.Flag{
		altsrc.NewStringFlag(
			&cli.StringFlag{
				Name:    "service",
				Aliases: []string{"s"},
				Value:   "",
				Usage:   "grpc full or partial service name",
			}),
		altsrc.NewStringFlag(
			&cli.StringFlag{
				Name:    "method",
				Aliases: []string{"m"},
				Value:   "",
				Usage:   "grpc service method name",
			}),
		altsrc.NewStringFlag(
			&cli.StringFlag{
				Name:    "input",
				Aliases: []string{"i"},
				Value:   "",
				Usage:   "file that contains message json, it will be ignored if used in conjunction with stdin pipes",
			}),
		altsrc.NewStringFlag(
			&cli.StringFlag{
				Name:    "deadline",
				Aliases: []string{"d"},
				Value:   "15s",
				Usage:   "grpc call deadline in go duration format, e.g. 15s, 3m, 1h, etc. If no format is specified, defaults to seconds",
			}),
		altsrc.NewBoolFlag(
			&cli.BoolFlag{
				Name:    "verbose",
				Aliases: []string{"V"},
				Usage:   "output some additional information like request time and message size",
			}),
		altsrc.NewBoolFlag(
			&cli.BoolFlag{
				Name:  "tls",
				Value: false,
				Usage: "use TLS when connecting to grpc server",
			}),
		altsrc.NewBoolFlag(
			&cli.BoolFlag{
				Name:  "insecure",
				Value: false,
				Usage: "skip server's certificate chain and host name verification, this option should only be used for testing",
			}),
		altsrc.NewStringFlag(
			&cli.StringFlag{
				Name:  "cacert",
				Value: "",
				Usage: "the CA certificate file for verifying the server, this certificate is ignored if --insecure option is true",
			}),
		altsrc.NewStringFlag(
			&cli.StringFlag{
				Name:  "cert",
				Value: "",
				Usage: "client certificate to present to the server, only valid with -certkey option",
			}),
		altsrc.NewStringFlag(
			&cli.StringFlag{
				Name:  "certkey",
				Value: "",
				Usage: "client private key, only valid with -cert option",
			}),
		altsrc.NewStringSliceFlag(
			&cli.StringSliceFlag{
				Name:     "proto",
				Required: false,
				Usage: "proto files or directories to search for proto files, " +
					"if this option is provided service reflection would be ignored. " +
					"In order to provide multiple paths, separate them with comma",
			}),
		altsrc.NewStringSliceFlag(
			&cli.StringSliceFlag{
				Name:     "protoimports",
				Required: false,
				Usage:    "additional directories to search for dependencies, should be used with --proto option",
			}),
		altsrc.NewGenericFlag(
			&cli.GenericFlag{
				Name:        "header",
				Aliases:     []string{"H"},
				Required:    false,
				Value:       cliext.NewMapValue(),
				Usage:       "extra header(s) to include in the request",
				DefaultText: "no extra headers",
			}),
		&cli.StringFlag{
			Name:  "authority",
			Value: "",
			Usage: "override :authority header",
		},
		&cli.GenericFlag{
			Name:    "informat",
			Aliases: []string{"if"},
			Value: &cliext.EnumValue{
				Enum:    []string{"json", "text"},
				Default: "json",
			},
			Usage: "input proto message format, supported values are json and text",
		},
		&cli.GenericFlag{
			Name:    "outformat",
			Aliases: []string{"of"},
			Value: &cliext.EnumValue{
				Enum:    []string{"json", "text"},
				Default: "json",
			},
			Usage: "output proto message format, supported values are json and text",
		},
		altsrc.NewBoolFlag(
			&cli.BoolFlag{
				Name:  "keepalive",
				Value: false,
				Usage: "If true, send keepalive pings even with no active RPCs. If false, default grpc settings are used",
			}),
		altsrc.NewDurationFlag(
			&cli.DurationFlag{
				Name:        "keepalive-time",
				Usage:       `If set, send keepalive pings every "keepalive-time" timeout. If not set, default grpc settings are used`,
				DefaultText: "not set",
			}),
		altsrc.NewIntFlag(
			&cli.IntFlag{
				Name:    "max-receive-message-size",
				Aliases: []string{"mrms", "max-recv-msg-size"},
				Value:   0,
				Usage:   "If greater than 0, sets the max receive message size to bytes, else uses grpc defaults (currently 4 MB)",
			}),
		&cli.StringFlag{
			Name:  flagConfigFile,
			Usage: "config file in yaml format to configure the cli parameters, the default location is ~/.grpc-client-cli.yaml",
		},
	}
}
