package main

import (
	"errors"
	"io/ioutil"
	"os"

	"github.com/urfave/cli/v2"
	"github.com/vadimi/grpc-client-cli/internal/caller"
	"gopkg.in/AlecAivazis/survey.v1/terminal"
)

const (
	appVersion = "1.4.0"
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
		&cli.IntFlag{
			Name:    "deadline",
			Aliases: []string{"d"},
			Value:   15,
			Usage:   "grpc call deadline in seconds",
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
			Usage: "the CA certificate file for verifying the server, this certificate is ignored if -insecure option is true",
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
			Usage:    "additional directories to search for dependencies, should be used with -proto option",
		},
		&cli.StringFlag{
			Name:  "authority",
			Value: "",
			Usage: "override :authority header",
		},
		&cli.GenericFlag{
			Name:    "format",
			Aliases: []string{"f"},
			Value: &EnumValue{
				Enum:    []string{"json", "text"},
				Default: "json",
			},
			Usage: "proto message format, supported values are json and text",
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
		return cli.NewExitError(err, 1)
	}
	return nil
}

func baseCmd(c *cli.Context) (e error) {
	err := runApp(c, &startOpts{})
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	return nil
}

func runApp(c *cli.Context, opts *startOpts) (e error) {
	target := ""
	if c.NArg() > 0 {
		target = c.Args().First()
	}

	if target == "" {
		err := errors.New("please provide service host:port")
		return err
	}

	opts.Service = c.String("service")
	opts.Method = c.String("method")
	opts.Deadline = c.Int("deadline")
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
	opts.Format = parseMsgFormat(c.Generic("format"))

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

	if err != nil && err != terminal.InterruptErr {
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
		bytes, err := ioutil.ReadAll(os.Stdin)
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

	f, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	if len(f) == 0 {
		return nil, errors.New("message file is empty")
	}

	return f, nil
}

func parseMsgFormat(val interface{}) caller.MsgFormat {
	if enum, ok := val.(*EnumValue); ok {
		if enum.String() == "text" {
			return caller.Text
		}
	}

	return caller.JSON
}
