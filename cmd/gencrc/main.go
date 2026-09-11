package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/community-outpost/GenCRC/internal/gamecrc"
	"github.com/community-outpost/GenCRC/internal/installpath"
	"github.com/community-outpost/GenCRC/internal/version"
)

type stringList []string

func (values *stringList) String() string         { return strings.Join(*values, ",") }
func (values *stringList) Set(value string) error { *values = append(*values, value); return nil }

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr, installpath.Discover); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(2)
	}
}

func run(args []string, out, stderr io.Writer, discover func(string) (string, string, error)) error {
	var root, exe, fallback, mod, game string
	var sideloads stringList
	var verbose bool
	flags := flag.NewFlagSet("gencrc", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&root, "installpath", "", "game directory (otherwise discovered from Windows registry)")
	flags.StringVar(&exe, "exe", "Generals.exe", "game binary filename or absolute path")
	flags.StringVar(&game, "game", "", "game identity: generals or generalsmd")
	flags.StringVar(&fallback, "fallback-generals-root", "", "original Generals install layered before sideloads")
	flags.Var(&sideloads, "sideload", "archive or directory layered as a game-folder source (repeatable)")
	flags.StringVar(&mod, "mod", "", "a .big file or folder of .big files")
	flags.BoolVar(&verbose, "v", false, "list archives and INI files to stderr")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments: %s", strings.Join(flags.Args(), " "))
	}
	if game != "" && game != "generals" && game != "generalsmd" {
		return fmt.Errorf("--game must be generals or generalsmd")
	}
	exeProvided := false
	flags.Visit(func(f *flag.Flag) {
		if f.Name == "exe" {
			exeProvided = true
		}
	})
	mode := "both"
	hasINIInputs := len(sideloads) > 0 || mod != "" || fallback != ""
	if exeProvided && !hasINIInputs {
		mode = "exe"
	} else if !exeProvided && hasINIInputs {
		mode = "ini"
	}
	// INI-only inputs deliberately never inspect an executable.
	if mode != "ini" && filepath.IsAbs(exe) {
		if err := mergeGame(&game, gamecrc.DetectGameFromExe(exe), "executable"); err != nil {
			return err
		}
	}
	if root == "" {
		discoveredRoot, discoveredGame, err := discover(game)
		if err != nil {
			return err
		}
		root = discoveredRoot
		if err := mergeGame(&game, discoveredGame, "registry installation"); err != nil {
			return err
		}
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("installpath must be an existing directory: %s", root)
	}
	if err := mergeGame(&game, gamecrc.DetectGame(root), "installation INI archives"); err != nil {
		return err
	}
	result := map[string]string{}
	if mode != "ini" {
		exePath := gamecrc.ResolveExe(root, exe)
		if !exeProvided && gamecrc.IsLauncher(exePath) {
			exePath = gamecrc.ResolveExe(root, "Game.dat")
		}
		data, err := os.ReadFile(exePath)
		if err != nil {
			return fmt.Errorf("cannot read game binary %s; pass --exe <name>: %w", exePath, err)
		}
		if err := mergeGame(&game, gamecrc.DetectGameBytes(data), "executable"); err != nil {
			return err
		}
		major, minor, err := version.Extract(data)
		if err != nil {
			return err
		}
		// ExeCRC historically tolerated missing scripts; the CLI must not claim client parity in that case.
		for _, name := range []string{"SkirmishScripts.scb", "MultiplayerScripts.scb"} {
			if _, err := os.ReadFile(filepath.Join(root, "Data", "Scripts", name)); err != nil {
				return fmt.Errorf("EXE CRC requires Data/Scripts/%s in --installpath: %w", name, err)
			}
		}
		crc, err := gamecrc.ExeCRC(exePath, root, major, minor)
		if err != nil {
			return err
		}
		result["exe_crc"] = fmt.Sprintf("%08X", crc)
		result["exe"] = filepath.Base(exePath)
		result["version"] = version.Format(major, minor)
		if online := version.Online(data); online != "" {
			result["generalsonline_version"] = online
		}
	}
	if game == "" {
		return fmt.Errorf("could not determine game identity; pass --game generals|generalsmd")
	}
	if mode != "exe" {
		crc, err := gamecrc.INICRC(root, fallback, game, sideloads, mod, verbose)
		if err != nil {
			return err
		}
		result["ini_crc"] = fmt.Sprintf("%08X", crc)
	}
	result["game"] = game
	return json.NewEncoder(out).Encode(result)
}

func mergeGame(game *string, detected, source string) error {
	if detected == "" {
		return nil
	}
	if *game != "" && *game != detected {
		return fmt.Errorf("game mismatch: selected %s but %s identifies %s; check --game, --exe and --installpath", *game, source, detected)
	}
	*game = detected
	return nil
}
