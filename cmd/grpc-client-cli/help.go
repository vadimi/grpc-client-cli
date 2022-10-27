package main

import (
	"runtime/debug"

	"github.com/urfave/cli/v2"
)

var helpTemplate = cli.AppHelpTemplate + `
BUILD INFO:
   go version: {{ExtraInfo.go_version}}{{if ExtraInfo.vcs_revision}}
   revision: {{ExtraInfo.vcs_revision}}{{end}}
`

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
