package config

import (
	"flag"
	"fmt"
	"io"
	"time"
)

const (
	ModeRun     = "run"
	ModeList    = "list"
	ModeEnable  = "enable"
	ModeDisable = "disable"
	ModeWatch   = "watch"
)

type Config struct {
	Mode          string
	PluginsDir    string
	InputFile     string
	PluginName    string
	RequestID     string
	Parallel      bool
	RunOnChange   bool
	WatchInterval time.Duration
}

func Parse(args []string) (Config, error) {
	cfg := Config{}

	flagSet := flag.NewFlagSet("sentipulseexam", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	flagSet.StringVar(&cfg.Mode, "mode", ModeRun, "run, list, enable, disable")
	flagSet.StringVar(&cfg.PluginsDir, "plugins-dir", "plugins", "directory containing plugin manifests")
	flagSet.StringVar(&cfg.InputFile, "input", "", "path to a JSON input file; if empty, read from stdin")
	flagSet.StringVar(&cfg.PluginName, "plugin", "", "plugin name used by enable/disable modes")
	flagSet.StringVar(&cfg.RequestID, "request-id", "", "optional request identifier")
	flagSet.BoolVar(&cfg.Parallel, "parallel", false, "execute enabled plugins concurrently")
	flagSet.BoolVar(&cfg.RunOnChange, "run-on-change", false, "in watch mode, execute enabled plugins whenever the plugin directory changes")
	flagSet.DurationVar(&cfg.WatchInterval, "watch-interval", 2*time.Second, "poll interval used by watch mode")

	if err := flagSet.Parse(args); err != nil {
		return Config{}, err
	}

	switch cfg.Mode {
	case ModeRun, ModeList, ModeWatch:
	case ModeEnable, ModeDisable:
		if cfg.PluginName == "" {
			return Config{}, fmt.Errorf("mode %q requires -plugin", cfg.Mode)
		}
	default:
		return Config{}, fmt.Errorf("invalid mode %q", cfg.Mode)
	}

	return cfg, nil
}

func Usage() string {
	return `Usage:
  go run . -mode list
  go run . -mode run -input examples/input.json
  go run . -mode watch -input examples/input.json -run-on-change
  go run . -mode enable -plugin keyword-tagger
  go run . -mode disable -plugin keyword-tagger

Flags:
  -mode         run, list, watch, enable, disable
  -plugins-dir  plugin directory (default "plugins")
  -input        JSON input file for run mode; if omitted, stdin is used
  -plugin       plugin name for enable/disable
  -request-id   optional request identifier for run mode
  -parallel     execute enabled plugins concurrently (default false)
  -run-on-change  in watch mode, execute enabled plugins on every plugin change
  -watch-interval poll interval for watch mode (default 2s)`
}
