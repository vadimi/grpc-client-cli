package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/urfave/cli/v2"
	"github.com/vadimi/grpc-client-cli/internal/caller"
	"github.com/vadimi/grpc-client-cli/internal/cliext"
)

const (
	appVersion = "1.12.0-pre1"
)

func main() {
	app := cli.NewApp()
	app.Usage = "generic gRPC client"
	app.Version = appVersion
	app.EnableBashCompletion = true
	app.Flags = appFlags()
	app.Before = cliext.InitWithYamlSource(app.Flags, cliext.WithFlagFileName(flagConfigFile))
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
	err := app.Run(os.Args)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
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
	target := ""
	if c.NArg() > 0 {
		target = c.Args().First()
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
