package main

import (
	"errors"
	"io"
	"os"

	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/urfave/cli/v2"
	"github.com/vadimi/grpc-client-cli/internal/caller"
	"github.com/vadimi/grpc-client-cli/internal/cliext"
)

const (
	appVersion = "1.12.0"
)

func main() {
	app := cli.NewApp()
	app.Usage = "generic gRPC client"
	app.Version = appVersion
	app.EnableBashCompletion = true

	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:    "service",
			Aliases: []string{"s"},
			Value:   "",
			Usage:   "grpc full or partial service name",
		},
		&cli.StringFlag{
			Name:    "method",
			Aliases: []string{"m"},
			Value:   "",
			Usage:   "grpc service method name",
		},
		&cli.StringFlag{
			Name:    "input",
			Aliases: []string{"i"},
			Value:   "",
			Usage:   "file that contains message json, it will be ignored if used in conjunction with stdin pipes",
		},
		&cli.StringFlag{
			Name:    "deadline",
			Aliases: []string{"d"},
			Value:   "15s",
			Usage:   "grpc call deadline in go duration format, e.g. 15s, 3m, 1h, etc. If no format is specified, defaults to seconds",
		},
		&cli.BoolFlag{
			Name:    "verbose",
			Aliases: []string{"V"},
			Usage:   "output some additional information like request time and message size",
		},
		&cli.BoolFlag{
			Name:  "tls",
			Value: false,
			Usage: "use TLS when connecting to grpc server",
		},
		&cli.BoolFlag{
			Name:  "insecure",
			Value: false,
			Usage: "skip server's certificate chain and host name verification, this option should only be used for testing",
		},
		&cli.StringFlag{
			Name:  "cacert",
			Value: "",
			Usage: "the CA certificate file for verifying the server, this certificate is ignored if --insecure option is true",
		},
		&cli.StringFlag{
			Name:  "cert",
			Value: "",
			Usage: "client certificate to present to the server, only valid with -certkey option",
		},
		&cli.StringFlag{
			Name:  "certkey",
			Value: "",
			Usage: "client private key, only valid with -cert option",
		},
		&cli.StringSliceFlag{
			Name:     "proto",
			Required: false,
			Usage: "proto files or directories to search for proto files, " +
				"if this option is provided service reflection would be ignored. " +
				"In order to provide multiple paths, separate them with comma",
		},
		&cli.StringSliceFlag{
			Name:     "protoimports",
			Required: false,
			Usage:    "additional directories to search for dependencies, should be used with --proto option",
		},
		&cli.GenericFlag{
			Name:        "header",
			Aliases:     []string{"H"},
			Required:    false,
			Value:       cliext.NewMapValue(),
			Usage:       "extra header(s) to include in the request",
			DefaultText: "no extra headers",
		},
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
		&cli.BoolFlag{
			Name:  "keepalive",
			Value: false,
			Usage: "If true, send keepalive pings even with no active RPCs. If false, default grpc settings are used",
		},
		&cli.DurationFlag{
			Name:        "keepalive-time",
			Usage:       `If set, send keepalive pings every "keepalive-time" timeout. If not set, default grpc settings are used`,
			DefaultText: "not set",
		},
		&cli.IntFlag{
			Name:    "max-receive-message-size",
			Aliases: []string{"mrms", "max-recv-msg-size"},
			Value:   0,
			Usage:   "If greater than 0, sets the max receive message size to bytes, else uses grpc defaults (currently 4 MB)",
		},
		&cli.StringFlag{
			Name:     "address",
			Aliases:  []string{"a", "addr"},
			Required: false,
			Usage:    "host:port of the service",
		},
	}

	app.Action = baseCmd
	app.Commands = []*cli.Command{
		{
			Name:   "discover",
			Usage:  "print service protobuf",
			Action: discoverCmd,
		},
		{
			Name:   "health",
			Usage:  "grpc health check",
			Action: healthCmd,
		},
	}
	app.Run(os.Args)
}

func discoverCmd(c *cli.Context) (e error) {
	opts := &startOpts{
		Discover: true,
	}
	err := runApp(c, opts)
	if err != nil {
		return cli.Exit(err, 1)
	}
	return nil
}

func baseCmd(c *cli.Context) (e error) {
	err := runApp(c, &startOpts{})
	if err != nil {
		return cli.Exit(err, 1)
	}
	return nil
}

func runApp(c *cli.Context, opts *startOpts) (e error) {
	target := c.String("address")
	if target == "" {
		if c.NArg() > 0 {
			target = c.Args().First()
		}
	}

	if target == "" {
		err := errors.New("please provide service host:port")
		return err
	}

	deadline, err := cliext.ParseDuration(c.String("deadline"))
	if err != nil {
		return err
	}

	opts.Service = c.String("service")
	opts.Method = c.String("method")
	opts.Deadline = int(deadline.Seconds())
	opts.Verbose = c.Bool("verbose")
	opts.Target = target
	opts.Authority = c.String("authority")
	opts.TLS = c.Bool("tls")
	opts.Insecure = c.Bool("insecure")
	opts.CACert = c.String("cacert")
	opts.Cert = c.String("cert")
	opts.CertKey = c.String("certkey")
	opts.Protos = c.StringSlice("proto")
	opts.ProtoImports = c.StringSlice("protoimports")
	opts.InFormat = parseMsgFormat(c.Generic("informat"))
	opts.OutFormat = parseMsgFormat(c.Generic("outformat"))
	opts.Headers = cliext.ParseMapValue(c.Generic("header"))
	opts.KeepaliveTime = c.Duration("keepalive-time")
	opts.Keepalive = c.Bool("keepalive")
	opts.MaxRecvMsgSize = c.Int("max-receive-message-size")

	input := c.String("input")

	message, err := getMessage(input)
	if err != nil {
		return err
	}

	// if message is not empty we are not in interactive mode
	opts.IsInteractive = len(message) == 0

	a, err := newApp(opts)
	defer func() {
		if a == nil {
			return
		}

		if err := a.Close(); err != nil {
			e = err
		}
	}()

	if err != nil {
		return err
	}

	err = a.Start(message)

	if err != nil && err != terminal.InterruptErr && err != ErrInterruptTerm {
		return err
	}

	return nil
}

func getMessage(input string) ([]byte, error) {
	message, err := readMessageFromstdin()
	if err != nil {
		return nil, err
	}

	if len(message) == 0 {
		message, err = readMessageFile(input)
		if err != nil {
			return nil, err
		}
	}

	return message, err
}

func readMessageFromstdin() ([]byte, error) {
	var message []byte
	s, err := os.Stdin.Stat()
	if err != nil {
		return nil, err
	}

	// only read from stdin if there is something to read
	if s.Mode()&os.ModeNamedPipe != 0 {
		bytes, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, err
		}

		message = bytes
	}

	return message, nil
}

func readMessageFile(file string) ([]byte, error) {
	if file == "" {
		return nil, nil
	}

	f, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}

	if len(f) == 0 {
		return nil, errors.New("message file is empty")
	}

	return f, nil
}

func parseMsgFormat(val interface{}) caller.MsgFormat {
	if enum, ok := val.(*cliext.EnumValue); ok {
		return caller.ParseMsgFormat(enum.String())
	}

	return caller.JSON
}
