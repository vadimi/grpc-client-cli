package cliext

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"github.com/urfave/cli/v2"
	"github.com/urfave/cli/v2/altsrc"
	"gopkg.in/yaml.v2"
)

type yamlSourceOptions struct {
	flagFileName string
	fsys         fs.FS
}

type SourceOption func(*yamlSourceOptions)

func WithFlagFileName(flagFileName string) SourceOption {
	return func(o *yamlSourceOptions) {
		o.flagFileName = flagFileName
	}
}

// WithFS is useful in unit testing
func WithFS(fsys fs.FS) SourceOption {
	return func(o *yamlSourceOptions) {
		o.fsys = fsys
	}
}

func InitWithYamlSource(flags []cli.Flag, opts ...SourceOption) cli.BeforeFunc {
	return func(context *cli.Context) error {
		sourceOptions := &yamlSourceOptions{
			fsys: os.DirFS("/"),
		}

		for _, o := range opts {
			o(sourceOptions)
		}

		inputSource, err := newYamlSourceFromFlagFunc(sourceOptions)(context)
		if err != nil {
			return err
		}

		return altsrc.ApplyInputSourceValues(context, inputSource, flags)
	}
}

func newYamlSourceFromFlagFunc(opts *yamlSourceOptions) func(context *cli.Context) (altsrc.InputSourceContext, error) {
	return func(context *cli.Context) (altsrc.InputSourceContext, error) {
		filePath := ""
		fileRequired := false
		if context.IsSet(opts.flagFileName) {
			filePath = context.String(opts.flagFileName)
			// if config file is passed via parameters, then it's required
			fileRequired = true
		}

		return newYamlSourceFromFile(opts.fsys, filePath, fileRequired)
	}
}

func newYamlSourceFromFile(fsys fs.FS, file string, fileRequired bool) (altsrc.InputSourceContext, error) {
	results := map[interface{}]interface{}{}
	err := readConfigYaml(fsys, file, fileRequired, &results)
	if err != nil {
		return nil, err
	}
	return altsrc.NewMapInputSource(file, results), nil
}

func readConfigYaml(fsys fs.FS, filePath string, fileRequired bool, container interface{}) error {
	configPath, found, err := findConfigFile(fsys, filePath)
	if err != nil {
		return fmt.Errorf("cannot find config file: %w", err)
	}
	if !found {
		if fileRequired {
			return fmt.Errorf("config file %s not found", filePath)
		}
		return nil
	}

	b, err := fs.ReadFile(fsys, configPath)
	if err != nil {
		return fmt.Errorf("cannot read config file: %w", err)
	}

	err = yaml.Unmarshal(b, container)
	if err != nil {
		return fmt.Errorf("invalid config file format: %w", err)
	}

	return nil
}

func findConfigFile(fsys fs.FS, filePath string) (string, bool, error) {
	if filePath == "" {
		return findDefaultConfigFile(fsys)
	}

	if _, err := fs.Stat(fsys, filePath); err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, err
	}
	return filePath, true, nil
}

// homeDir config is optional
func findDefaultConfigFile(fsys fs.FS) (string, bool, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", false, err
	}
	homeDir, err = filepath.Rel("/", homeDir)
	if err != nil {
		return "", false, err
	}

	defaultLocations := []string{
		path.Join(homeDir, ".grpc-client-cli.yaml"),
		path.Join(homeDir, ".grpc-client-cli.yml"),
	}

	for _, loc := range defaultLocations {
		if _, err := fs.Stat(fsys, loc); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", false, err
		}
		return loc, true, nil
	}

	return "", false, nil
}
