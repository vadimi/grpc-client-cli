package main

import (
	"runtime/debug"

	"github.com/urfave/cli/v3"
)

var helpTemplate = cli.RootCommandHelpTemplate + `
{{with call .ExtraInfo}}BUILD INFO:
   go version: {{index . "go_version"}}{{if index . "vcs_revision"}}
   revision: {{index . "vcs_revision"}}{{end}}
{{end}}`

func getExtraInfo() map[string]string {
	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		return nil
	}

	info := map[string]string{
		"go_version": buildInfo.GoVersion,
	}

	for _, setting := range buildInfo.Settings {
		if setting.Key == "vcs.revision" {
			info["vcs_revision"] = setting.Value
		}
	}

	return info
}
