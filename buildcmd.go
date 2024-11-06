package main

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/rs/zerolog/log"
)

type BuildCommand struct {
	BuildTags          string `long:"buildtags" description:"Go build tags for the build process"`
	KeepTemporaryFiles bool   `long:"keep-files" description:"Keep temporary generated files after compilation"`
}

var buildCommand BuildCommand

func (x *BuildCommand) Execute(rargs []string) error {
	args.Process()
	ctx, err := DoPyExports(args, rargs)
	if err != nil {
		return err
	}

	goargs := []string{"build", "-buildmode=c-shared"}
	tags := args.GoTags
	if x.BuildTags != "" {
		tags = append(tags, x.BuildTags)
	}
	if len(tags) > 0 {
		goargs = append(goargs, fmt.Sprintf("-tags=%s", strings.Join(tags, ",")))
	}

	soFilename := fmt.Sprintf("%s.so", args.PyModuleName)
	if args.OutputDir != "" {
		soFilename = path.Join(args.OutputDir, soFilename)
	}
	goargs = append(goargs, "-o", soFilename)

	gocmd := exec.Command("go", goargs...)
	gocmd.Stdout = os.Stdout
	gocmd.Stderr = os.Stderr

	if ctx.WithNumpy {
		// Get the import path for numpy
		cmd := exec.Command("python3", "-c", "import numpy; print(numpy.get_include())")
		out, err := cmd.CombinedOutput()
		if err != nil {
			log.Error().Msg("Failed to find numpy's import path")

		} else {
			numpypath := strings.TrimSpace(string(out))
			if _, err := os.Stat(numpypath); err == nil {
				log.Trace().Msgf("Using numpy include path: %v", numpypath)
				gocmd.Env = append(os.Environ(), fmt.Sprintf("CGO_CFLAGS=-I%s", numpypath))

			} else {
				log.Error().Err(err).Msgf("Invalid path: %s", numpypath)
			}
		}
	}

	log.Debug().Msgf("%v", gocmd)
	err = gocmd.Run()
	if err != nil {
		return err
	}
	log.Debug().Msgf("Built %s", soFilename)

	if !x.KeepTemporaryFiles {
		for _, fname := range []string{
			args.OutputCCode,
			args.OutputChdrCode,
			args.OutputGoCode,
		} {
			log.Debug().Msgf("Removing %s", fname)
			err := os.Remove(fname)
			if err != nil {
				log.Error().Err(err).Msgf("Failed to remove %s", fname)
			}
		}
	}
	return nil
}

func init() {
	flagparser.AddCommand("build",
		"Build the library",
		"The build command builds the final python library file",
		&buildCommand)
}
