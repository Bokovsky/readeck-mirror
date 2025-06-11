// SPDX-FileCopyrightText: © 2020 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/cristalhq/acmd"

	"codeberg.org/readeck/readeck/configs"
	"codeberg.org/readeck/readeck/docs"
	"codeberg.org/readeck/readeck/internal/admin"
	"codeberg.org/readeck/readeck/internal/assets"
	"codeberg.org/readeck/readeck/internal/auth/onboarding"
	"codeberg.org/readeck/readeck/internal/auth/signin"
	bookmark_routes "codeberg.org/readeck/readeck/internal/bookmarks/routes"
	"codeberg.org/readeck/readeck/internal/bus"
	"codeberg.org/readeck/readeck/internal/cookbook"
	"codeberg.org/readeck/readeck/internal/dashboard"
	"codeberg.org/readeck/readeck/internal/metrics"
	"codeberg.org/readeck/readeck/internal/opds"
	"codeberg.org/readeck/readeck/internal/profile"
	"codeberg.org/readeck/readeck/internal/server"
	"codeberg.org/readeck/readeck/internal/videoplayer"
)

type serveFlags struct {
	appFlags
	Host string
	Port uint
}

func (f *serveFlags) Flags() *flag.FlagSet {
	fs := f.appFlags.Flags()
	fs.StringVar(&f.Host, "host", "", "Listen to address")
	fs.UintVar(&f.Port, "port", 0, "Listen to port")

	return fs
}

func init() {
	commands = append(commands, acmd.Command{
		Name:        "serve",
		Description: "Start Readeck HTTP server",
		ExecFunc:    runServer,
	})
}

func runServer(_ context.Context, args []string) error {
	var flags serveFlags
	if err := flags.Flags().Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	// Init application
	if err := appPreRun(&flags.appFlags); err != nil {
		return err
	}
	defer appPostRun()

	// Command flags are the last override values
	if flags.Host != "" {
		configs.Config.Server.Host = flags.Host
	}
	if flags.Port > 0 {
		configs.Config.Server.Port = int(flags.Port)
	}

	// Prepare HTTP server
	s := server.New(configs.Config.Server.Prefix)
	if err := InitServer(s); err != nil {
		return err
	}

	if err := bus.Load(); err != nil {
		return err
	}

	ready := make(chan bool)
	stop := make(chan os.Signal, 2)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Start the metrics HTTP server
	startMetrics := configs.Config.Metrics.Port > 0
	if startMetrics {
		slog.Info("metrics endpoint",
			slog.String("host", configs.Config.Metrics.Host),
			slog.Int("port", configs.Config.Metrics.Port),
		)
		go func() {
			if err := metrics.ListenAndServe(
				configs.Config.Metrics.Host,
				configs.Config.Metrics.Port,
			); err != nil {
				fatal("cannot start the metrics server", err)
			}
		}()
	}

	// Start the embed standalone worker.
	startBus := configs.Config.Worker.StartWorker || bus.Protocol() == "memory"
	if startBus {
		go func() {
			bus.Tasks().Start()
			slog.Info("workers started")
		}()
	}

	srv := &http.Server{
		Handler:           s.Router,
		MaxHeaderBytes:    1 << 20,
		ReadHeaderTimeout: time.Second * 5,
	}
	var serveURL string

	// Start the HTTP server
	go func() {
		var ln net.Listener
		host := configs.Config.Server.Host
		if strings.HasPrefix(host, "unix:") {
			listenPath := strings.TrimPrefix(host, "unix:")
			unixAddr, err := net.ResolveUnixAddr("unix", listenPath)
			if err != nil {
				fatal("failed to parse listen address "+host, err)
			}
			srv.Addr = unixAddr.Name
			ln, err = net.ListenUnix("unix", unixAddr)
			if err != nil {
				fatal("failed to listen on "+host, err)
			}
			serveURL = unixAddr.String()
		} else {
			srv.Addr = net.JoinHostPort(
				configs.Config.Server.Host,
				strconv.Itoa(configs.Config.Server.Port),
			)
			var err error
			ln, err = net.Listen("tcp", srv.Addr)
			if err != nil {
				fatal("cannot start the server", err)
			}

			listenURL := url.URL{
				Scheme: "http",
				Host:   fmt.Sprintf("%s:%d", configs.Config.Server.Host, configs.Config.Server.Port),
				Path:   s.BasePath,
			}
			if listenURL.Hostname() == "0.0.0.0" || listenURL.Hostname() == "127.0.0.1" {
				listenURL.Host = fmt.Sprintf("localhost:%d", configs.Config.Server.Port)
			}
			serveURL = listenURL.String()
		}

		ready <- true
		if err := srv.Serve(ln); err != nil {
			if err == http.ErrServerClosed {
				slog.Info("stopping server...")
				return
			}
			slog.Error("server error", slog.Any("err", err))
		}
	}()

	// Server is ready to accept requests
	<-ready
	slog.Info("server started", slog.String("url", serveURL))

	// Server shutdown
	<-stop
	slog.Info("shutting down...")

	// Graceful http shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("shutdown error", slog.Any("err", err))
	}
	slog.Info("server stopped")

	if startBus {
		slog.Info("stopping workers...")
		bus.Tasks().Stop()
		slog.Info("workers stopped")
	}

	return nil
}

// InitServer setups all the routes.
func InitServer(s *server.Server) error {
	// Init session store
	if err := s.InitSession(); err != nil {
		return err
	}

	// Static asserts
	assets.SetupRoutes(s)

	// Auth routes
	signin.SetupRoutes(s)

	// Onboarding routes
	onboarding.SetupRoutes(s)

	// Dashboard routes
	dashboard.SetupRoutes(s)

	// Bookmark routes
	// - /bookmarks/*
	// - /bm/* (for bookmark media files)
	bookmark_routes.SetupRoutes(s)

	// OPDS routes
	opds.SetupRoutes(s)

	// User routes
	profile.SetupRoutes(s)

	// Admin routes
	admin.SetupRoutes(s)

	// Video player route
	videoplayer.SetupRoutes(s)

	// Help routes
	docs.SetupRoutes(s)

	// Cookbook routes
	cookbook.SetupRoutes(s)

	s.Init()
	return nil
}
