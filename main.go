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
	Debug          []bool   `long:"debug" short:"d" description:"Enable debug messages"`
	OutputDir      string   `long:"output-dir" description:"Output directory"`
	OutputCCode    string   `long:"output-c-code" description:"Output C code file" default:"pyexports.c" required:"true"`
	OutputChdrCode string   `long:"output-chdr-code" description:"Output C header file" default:"pyexports.h" required:"true"`
	OutputGoCode   string   `long:"output-go-code" description:"Output Go code file" default:"pyexports.go" required:"true"`
	PyModuleName   string   `long:"pymodule" description:"Name of the python module" default:"gomodule" required:"true"`
	GoTags         []string `long:"tags" description:"Go tags for the generated Go code file"`
}

func (a *Args) Process() {
	switch len(args.Debug) {
	case 0:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case 1:
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.TraceLevel)
	}

	if args.OutputDir != "" {
		args.OutputCCode = path.Join(args.OutputDir, args.OutputCCode)
		args.OutputChdrCode = path.Join(args.OutputDir, args.OutputChdrCode)
		args.OutputGoCode = path.Join(args.OutputDir, args.OutputGoCode)
	}

	if args.PyModuleName == "" {
		log.Fatal().Msg("Need to define a module name")
	}
}

var args Args

var flagparser = flags.NewParser(&args, flags.Default)

func main() {
	_, err := flagparser.Parse()
	if err != nil {
		switch flagsErr := err.(type) {
		case flags.ErrorType:
			if flagsErr == flags.ErrHelp {
				os.Exit(0)
			}
			os.Exit(1)
		default:
			os.Exit(1)
		}
	}
}
