package cliext

import (
	"flag"
	"os"
	"path"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
	"github.com/urfave/cli/v2/altsrc"
)

var testFlags = []cli.Flag{
	altsrc.NewIntFlag(&cli.IntFlag{Name: "id", Required: false}),
	&cli.StringFlag{Name: "config"},
}

func TestNoConfigFileFound(t *testing.T) {
	app, set := newTestApp([]string{"test-cmd"})

	fsys := fstest.MapFS{}

	c := cli.NewContext(app, set, nil)
	command := &cli.Command{
		Name:  "test-cmd",
		Flags: testFlags,
		Action: func(c *cli.Context) error {
			return nil
		},
		Before: InitWithYamlSource(testFlags, WithFlagFileName("config"), WithFS(fsys)),
	}

	err := command.Run(c)
	assert.NoError(t, err)
}

func TestConfigFileRequired(t *testing.T) {
	app, set := newTestApp([]string{"test-cmd", "--config", "test.yaml"})

	fsys := fstest.MapFS{}

	c := cli.NewContext(app, set, nil)
	command := &cli.Command{
		Name:  "test-cmd",
		Flags: testFlags,
		Action: func(c *cli.Context) error {
			return nil
		},
		Before: InitWithYamlSource(testFlags, WithFlagFileName("config"), WithFS(fsys)),
	}

	err := command.Run(c)
	assert.Error(t, err)
}

func TestConfigFileLoaded(t *testing.T) {
	app, set := newTestApp([]string{"test-cmd", "--config", "test.yaml"})

	fsys := fstest.MapFS{
		"test.yaml": &fstest.MapFile{
			Data: []byte(`id: 2`),
		},
	}

	c := cli.NewContext(app, set, nil)
	command := &cli.Command{
		Name:  "test-cmd",
		Flags: testFlags,
		Action: func(c *cli.Context) error {
			require.Equal(t, 2, c.Int("id"))
			return nil
		},
		Before: InitWithYamlSource(testFlags, WithFlagFileName("config"), WithFS(fsys)),
	}

	err := command.Run(c)
	assert.NoError(t, err)
}

func TestDefaultConfigFileLoaded(t *testing.T) {
	app, set := newTestApp([]string{"test-cmd"})

	homeDir, err := os.UserHomeDir()
	require.NoError(t, err)
	homeDir = strings.Trim(homeDir, "/")

	fsys := fstest.MapFS{
		path.Join(homeDir, ".grpc-client-cli.yaml"): &fstest.MapFile{
			Data: []byte(`id: 1`),
		},
	}

	c := cli.NewContext(app, set, nil)
	command := &cli.Command{
		Name:  "test-cmd",
		Flags: testFlags,
		Action: func(c *cli.Context) error {
			require.Equal(t, 1, c.Int("id"))
			return nil
		},
		Before: InitWithYamlSource(testFlags, WithFlagFileName("config"), WithFS(fsys)),
	}

	err = command.Run(c)
	assert.NoError(t, err)
}

func newTestApp(commandArgs []string) (*cli.App, *flag.FlagSet) {
	app := &cli.App{}
	set := flag.NewFlagSet("test", 0)
	_ = set.Parse(commandArgs)
	return app, set
}
