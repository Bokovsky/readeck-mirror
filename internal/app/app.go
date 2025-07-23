// SPDX-FileCopyrightText: © 2020 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package app is Readeck main application.
package app

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"hash/crc32"
	"io"
	"log/slog"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/cristalhq/acmd"
	"github.com/phsym/console-slog"

	"codeberg.org/readeck/readeck/configs"
	"codeberg.org/readeck/readeck/internal/acls"
	"codeberg.org/readeck/readeck/internal/auth/users"
	"codeberg.org/readeck/readeck/internal/bookmarks"
	"codeberg.org/readeck/readeck/internal/db"
	"codeberg.org/readeck/readeck/internal/email"
	"codeberg.org/readeck/readeck/locales"
	"codeberg.org/readeck/readeck/pkg/extract/contentscripts"
)

var commands = []acmd.Command{}

var (
	colorReset  = console.ResetMod
	colorGreen  = console.ToANSICode(console.Green)
	colorYellow = console.ToANSICode(console.Yellow)
	bold        = console.ToANSICode(console.Bold)
)

type appFlags struct {
	ConfigFile string
}

func (f *appFlags) Flags() *flag.FlagSet {
	fs := flag.NewFlagSet("", flag.ContinueOnError)
	fs.StringVar(&f.ConfigFile, "config", "config.toml", "configuration file path")

	return fs
}

func fatal(msg string, err error) {
	slog.Error(msg, slog.Any("err", err))
	os.Exit(1)
}

type stringsFlag []string

func (l *stringsFlag) String() string {
	return fmt.Sprintf("%v", *l)
}

func (l *stringsFlag) Set(value string) error {
	*l = append(*l, value)
	return nil
}

// Run starts the application CLI.
func Run() error {
	return acmd.RunnerOf(commands, acmd.Config{
		AppName:        "readeck",
		AppDescription: "Run Readeck commands",
		Version:        configs.Version(),
	}).Run()
}

// InitApp prepares the app for running the server or the tests.
func InitApp() {
	// Setup logger
	var handler slog.Handler
	switch f := configs.Config.Main.LogFormat; f {
	case "", "json":
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: configs.Config.Main.LogLevel,
		})
	case "text":
		handler = console.NewHandler(os.Stdout, &console.HandlerOptions{
			Level:      configs.Config.Main.LogLevel,
			Theme:      stdLogTheme,
			TimeFormat: time.RFC3339Nano,
		})
	case "dev":
		handler = console.NewHandler(os.Stdout, &console.HandlerOptions{
			Level:      configs.Config.Main.LogLevel,
			Theme:      devLogTheme,
			TimeFormat: "15:04:05.0000",
		})
	default:
		fatal("can't setup logger", fmt.Errorf("unknown format %s", f))
	}

	slog.SetDefault(slog.New(handler))

	// Load locales
	locales.Load()

	// Create required folders
	if err := createFolder(configs.Config.Main.DataDirectory); err != nil {
		fatal("can't create data directory", err)
	}

	// Create content-scripts folder
	if err := createFolder(filepath.Join(configs.Config.Main.DataDirectory, "content-scripts")); err != nil {
		fatal("can't create content-scripts directory", err)
	}
	if err := copyContentScriptHelpers(filepath.Join(configs.Config.Main.DataDirectory, "content-scripts")); err != nil {
		slog.Warn("could not copy content-scripts helpers", slog.Any("err", err))
	}

	// Preload content scripts
	_ = bookmarks.LoadContentScripts(slog.Default())

	// Database URL
	dsn, err := url.Parse(configs.Config.Database.Source)
	if err != nil {
		fatal("can't read database source value", err)
	}

	// SQLite data path
	if dsn.Scheme == "sqlite3" {
		if err := createFolder(path.Dir(dsn.Opaque)); err != nil {
			fatal("can't create database directory", err)
		}
	}

	// Connect to database
	if err := db.Open(configs.Config.Database.Source); err != nil {
		fatal("can't connect to database", err)
	}

	// Init db schema
	if err := db.Init(); err != nil {
		fatal("can't initialize database", err)
	}

	// Init email sending
	email.InitSender()
	if !email.CanSendEmail() {
		// If we can't send email, remnove the mail permission.
		if _, err = acls.DeleteRole("/email/send"); err != nil {
			panic(err)
		}
	}

	// Set the commissioned flag
	nbUser, err := users.Users.Count()
	if err != nil {
		panic(err)
	}
	configs.Config.Commissioned = nbUser > 0
}

func enforceChecks(flags *appFlags) error {
	if flags.ConfigFile == "" {
		flags.ConfigFile = "config.toml"
	}

	if err := configs.LoadConfiguration(flags.ConfigFile); err != nil {
		return fmt.Errorf("error loading configuration: %w", err)
	}

	if configs.Config.Main.SecretKey == "" {
		return errors.New("invalid configuration file")
	}

	s, err := os.Stat(configs.Config.Main.DataDirectory)
	if err != nil {
		return err
	}
	if !s.IsDir() {
		return fmt.Errorf("%s is not a directory", configs.Config.Main.DataDirectory)
	}

	return nil
}

func appPreRun(flags *appFlags) error {
	if flags.ConfigFile == "" {
		flags.ConfigFile = "config.toml"
	}
	if err := createConfigFile(flags.ConfigFile); err != nil {
		return err
	}

	if err := configs.LoadConfiguration(flags.ConfigFile); err != nil {
		return fmt.Errorf("error loading configuration: %w", err)
	}

	if err := initConfig(flags.ConfigFile); err != nil {
		return err
	}

	InitApp()

	return nil
}

func appPostRun() {
	if err := db.Close(); err != nil {
		slog.Error("closing database", slog.Any("err", err))
	} else {
		slog.Debug("database is closed")
	}
}

func createConfigFile(filename string) error {
	_, err := os.Stat(filename)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		fd, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
		if err != nil {
			return err
		}
		if err = fd.Close(); err != nil {
			return err
		}
	}
	return nil
}

func initConfig(filename string) error {
	// If secret key is empty, we're facing a new configuration file and
	// must write it to a file.
	if configs.Config.Main.SecretKey == "" {
		configs.Config.Main.SecretKey = configs.GenerateKey()
		return configs.WriteConfig(filename)
	}

	return nil
}

func createFolder(name string) error {
	stat, err := os.Stat(name)
	if err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(name, 0o750); err != nil {
				return err
			}
		} else {
			return err
		}
	} else if !stat.IsDir() {
		return fmt.Errorf("'%s' is not a directory", name)
	}

	return nil
}

func copyContentScriptHelpers(dest string) error {
	for _, name := range []string{"@types.d.ts", "jsconfig.json"} {
		if err := func(name string) error {
			dcs := crc32.NewIEEE()
			fd, err := os.Open(filepath.Join(dest, name))
			if err == nil {
				defer fd.Close() //nolint:errcheck
				if _, err := io.Copy(dcs, fd); err != nil {
					return err
				}
			} else if !errors.Is(err, os.ErrNotExist) {
				return err
			}

			f, err := contentscripts.ContentScriptFS.Open(name)
			if err != nil {
				return err
			}
			defer f.Close() //nolint:errcheck

			contents := new(bytes.Buffer)
			scs := crc32.NewIEEE()
			if _, err := io.Copy(scs, io.TeeReader(f, contents)); err != nil {
				return err
			}

			if dcs.Sum32() == scs.Sum32() {
				return nil
			}

			dst, err := os.Create(filepath.Join(dest, name))
			if err != nil {
				return err
			}
			defer dst.Close() //nolint:errcheck

			slog.Debug("copying helper",
				slog.String("src", name),
				slog.String("dst", filepath.Join(dest, name)),
			)
			_, err = io.Copy(dst, contents)
			return err
		}(name); err != nil {
			return err
		}
	}

	return nil
}

func confirmPrompt(label string, defaultV bool) bool {
	choices := "Y/n"
	if !defaultV {
		choices = "y/N"
	}

	r := bufio.NewReader(os.Stdin)
	var s string

	for {
		fmt.Fprintf(os.Stderr, "%s (%s) ", label, choices)
		s, _ = r.ReadString('\n')
		s = strings.TrimSpace(s)
		if s == "" {
			return defaultV
		}
		s = strings.ToLower(s)
		if s == "y" || s == "yes" {
			return true
		}
		if s == "n" || s == "no" {
			return false
		}
	}
}
