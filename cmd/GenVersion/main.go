package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/community-outpost/GenVersion/internal/gamecrc"
	"github.com/community-outpost/GenVersion/internal/version"
)

type stringList []string

func (values *stringList) String() string         { return strings.Join(*values, ",") }
func (values *stringList) Set(value string) error { *values = append(*values, value); return nil }

func main() {
	var installpath, exe, fallbackGeneralsRoot, mod string
	var sideloads stringList
	var verbose bool
	flag.StringVar(&installpath, "installpath", "", "game installpath")
	flag.StringVar(&exe, "exe", "Generals.exe", "game binary filename or absolute path")
	flag.StringVar(&fallbackGeneralsRoot, "fallback-generals-root", "", "original Generals install layered before sideloads")
	flag.Var(&sideloads, "sideload", "archive or directory layered as a game-folder source (repeatable)")
	flag.StringVar(&mod, "mod", "", "a .big file or folder of .big files")
	flag.BoolVar(&verbose, "v", false, "list archives and INI files to stderr")
	flag.Parse()
	exeProvided := false
	flag.Visit(func(current *flag.Flag) {
		if current.Name == "exe" {
			exeProvided = true
		}
	})
	if installpath == "" {
		fail("--installpath is required")
	}
	info, err := os.Stat(installpath)
	if err != nil || !info.IsDir() {
		fail("installpath must be an existing directory: " + installpath)
	}
	exePath := gamecrc.ResolveExe(installpath, exe)
	if !exeProvided && gamecrc.IsLauncher(exePath) {
		exePath = filepath.Join(installpath, "Game.dat")
	}
	if _, err := os.Stat(exePath); err != nil {
		fail("could not find the default game binary; pass --exe <name> for another binary")
	}
	game := gamecrc.DetectGameFromExe(exePath)
	if game == "" {
		game = gamecrc.DetectGame(installpath)
	}
	if game == "" {
		fail("could not detect game from the executable, INIZH.big, or INI.big")
	}
	major, minor, err := version.FromFile(exePath)
	if err != nil {
		fail(err.Error())
	}
	iniCRC, err := gamecrc.INICRC(installpath, fallbackGeneralsRoot, game, sideloads, mod, verbose)
	if err != nil {
		fail(err.Error())
	}
	exeCRC, err := gamecrc.ExeCRC(exePath, installpath, major, minor)
	if err != nil {
		fail(err.Error())
	}
	data, _ := os.ReadFile(exePath)
	result := map[string]string{"exe_crc": fmt.Sprintf("%08X", exeCRC), "ini_crc": fmt.Sprintf("%08X", iniCRC), "game": game, "exe": filepath.Base(exePath), "version": version.Format(major, minor)}
	if online := version.Online(data); online != "" {
		result["generalsonline_version"] = online
	}
	encoded, _ := json.Marshal(result)
	fmt.Println(string(encoded))
}

func fail(message string) { fmt.Fprintln(os.Stderr, "error:", message); os.Exit(2) }
