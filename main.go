package main

import (
	"os"
	"path"

	flags "github.com/jessevdk/go-flags"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func init() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	if _, ok := os.LookupEnv("TRACE"); ok {
		zerolog.SetGlobalLevel(zerolog.TraceLevel)
	} else if _, ok := os.LookupEnv("DEBUG"); ok {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}
}

type Args struct {
	Debug        []bool `long:"debug" short:"d" description:"Enable debug messages"`
	OutputDir    string `long:"output-dir" description:"Output directory"`
	OutputCCode  string `long:"output-c-code" description:"Output C code file" default:"pyexports.c" required:"true"`
	OutputGoCode string `long:"output-go-code" description:"Output Go code file" default:"pyexports.go" required:"true"`
	PyModuleName string `long:"pymodule" description:"Name of the python module" default:"gomodule" required:"true"`
}

func main() {
	var args Args
	rargs, err := flags.Parse(&args)
	if err != nil {
		os.Exit(1)
	}

	if len(args.Debug) == 1 {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else if len(args.Debug) > 1 {
		zerolog.SetGlobalLevel(zerolog.TraceLevel)
	}

	if args.OutputDir != "" {
		args.OutputCCode = path.Join(args.OutputDir, args.OutputCCode)
		args.OutputGoCode = path.Join(args.OutputDir, args.OutputGoCode)
	}
	DoPyExports(args, rargs)
}
