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
	err := DoPyExports(args, rargs)
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

	cmd := exec.Command("go", goargs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.Debug().Msgf("%v", cmd)
	err = cmd.Run()
	if err != nil {
		return err
	}
	log.Info().Msgf("Built %s", soFilename)

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
