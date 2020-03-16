package main

import (
	"errors"
	"io/ioutil"
	"os"

	"github.com/urfave/cli"
	"gopkg.in/AlecAivazis/survey.v1/terminal"
)

const (
	appVersion = "1.0.0"
)

func main() {
	app := cli.NewApp()
	app.Usage = "generic gRPC client"
	app.Version = appVersion

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "service, s",
			Value: "",
			Usage: "grpc full or partial service name",
		},
		cli.StringFlag{
			Name:  "method, m",
			Value: "",
			Usage: "grpc service method name",
		},
		cli.StringFlag{
			Name:  "input, i",
			Value: "",
			Usage: "file that contains message json, it will be ignored if used in conjunction with stdin pipes",
		},
		cli.IntFlag{
			Name:  "deadline, d",
			Value: 15,
			Usage: "grpc call deadline in seconds",
		},
		cli.BoolFlag{
			Name:  "verbose, V",
			Usage: "output some additional information like request time and message size",
		},
	}

	app.Action = baseCmd
	app.Commands = []cli.Command{
		cli.Command{
			Name:   "discover",
			Usage:  "print service protobuf",
			Action: discoverCmd,
		},
		cli.Command{
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

	opts.Service = c.GlobalString("service")
	opts.Method = c.GlobalString("method")
	opts.Deadline = c.GlobalInt("deadline")
	opts.Verbose = c.GlobalBool("verbose")
	opts.Target = target

	input := c.GlobalString("input")

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
