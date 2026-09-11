package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/community-outpost/GenCRC/internal/retailsnapshot"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		if err != flag.ErrHelp {
			fmt.Fprintln(os.Stderr, "gencrc-snapshots:", err)
			os.Exit(1)
		}
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	var options retailsnapshot.Options
	flags := flag.NewFlagSet("gencrc-snapshots", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&options.Game, "game", "", "prepare or verify only generals or generalsmd (default: both)")
	flags.StringVar(&options.GeneralsRoot, "generals-root", "", "Generals installation directory; overrides registry discovery")
	flags.StringVar(&options.ZeroHourRoot, "zero-hour-root", "", "Zero Hour installation directory; overrides registry discovery")
	flags.StringVar(&options.Output, "output", "worker/retail", "prepared snapshot directory")
	flags.BoolVar(&options.Verify, "verify", false, "verify existing snapshots against the baseline lock without registry discovery or writes")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage: gencrc-snapshots [flags]")
		fmt.Fprintln(stderr, "Prepare raw INI/SCB deployment inputs matching the pinned baseline fingerprints.")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected argument %q", flags.Arg(0))
	}
	manifests, err := retailsnapshot.Run(options)
	if err != nil {
		return err
	}
	action := "Prepared"
	if options.Verify {
		action = "Verified"
	}
	for _, manifest := range manifests {
		fmt.Fprintf(stdout, "%s %s: %d files match the baseline lock in %s\n", action, manifest.Game, len(manifest.Files), options.Output)
	}
	return nil
}
