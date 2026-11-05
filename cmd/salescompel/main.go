package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"salescompel/internal/platform"
)

func run() error { return dispatch(os.Args[1:], os.Stdout) }
func dispatch(args []string, out io.Writer) error {
	if len(args) < 1 {
		return errors.New("usage: salescompel serve|provision")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if args[0] == "migrate" {
		migrationCtx, stop := context.WithTimeout(context.Background(), 30*time.Minute)
		defer stop()
		return migrateCommand(migrationCtx, args, out)
	}
	if args[0] == "schema" {
		schemaCtx, stop := context.WithTimeout(context.Background(), 30*time.Second)
		defer stop()
		return schemaCommand(schemaCtx, args, out)
	}
	s, err := platform.OpenOperator(ctx, os.Getenv("MIGRATION_DATABASE_URL"))
	if err != nil {
		return err
	}
	defer s.Close()
	switch args[0] {
	case "user", "oauth-app":
		return adminCommand(ctx, s, args, out)
	case "provision":
		return s.Provision(ctx, os.Getenv("SALESCOMPEL_TOKEN"), platform.Principal{Tenant: os.Getenv("SALESCOMPEL_TENANT"), User: os.Getenv("SALESCOMPEL_USER"), Role: os.Getenv("SALESCOMPEL_ROLE")})
	case "serve":
		fs := flag.NewFlagSet("serve", flag.ContinueOnError)
		addr := fs.String("addr", "127.0.0.1:8080", "listen address")
		publicURL := fs.String("public-url", "", "public OAuth origin")
		accessTTL := fs.Duration("access-token-ttl", 2*time.Hour, "OAuth access token lifetime")
		assets := fs.String("assets", "web/dist", "built web assets; empty disables UI")
		cert := fs.String("tls-cert", "", "TLS certificate")
		key := fs.String("tls-key", "", "TLS private key")
		if err = fs.Parse(args[1:]); err != nil {
			return err
		}
		if (*cert == "") != (*key == "") {
			return errors.New("both TLS certificate and key required")
		}
		host, _, err := net.SplitHostPort(*addr)
		if err != nil {
			return errors.New("invalid listen address")
		}
		ip := net.ParseIP(host)
		if *cert == "" && host != "localhost" && (ip == nil || !ip.IsLoopback()) {
			return errors.New("non-loopback listeners require TLS")
		}
		resolvedURL, err := platform.ResolvePublicURL(*publicURL, *addr, *cert != "")
		if err != nil {
			return err
		}
		if *accessTTL <= 0 {
			return errors.New("access-token-ttl must be positive")
		}
		if err = platform.AssetDir(*assets); err != nil {
			return err
		}
		server := &http.Server{Addr: *addr, Handler: platform.NewHandlerWithOptions(s, *assets, platform.Options{PublicURL: resolvedURL, AccessTokenTTL: *accessTTL}), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 20 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 16}
		done, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		errCh := make(chan error, 1)
		go func() {
			if *cert != "" {
				errCh <- server.ListenAndServeTLS(*cert, *key)
			} else {
				errCh <- server.ListenAndServe()
			}
		}()
		log.Printf("SalesCompel candidate listening on %s", *addr)
		select {
		case err = <-errCh:
			if !errors.Is(err, http.ErrServerClosed) {
				return err
			}
		case <-done.Done():
			shutdownCtx, c := context.WithTimeout(context.Background(), 10*time.Second)
			defer c()
			return server.Shutdown(shutdownCtx)
		}
		return nil
	default:
		return errors.New("unknown command")
	}
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "SalesCompel:", err)
		os.Exit(1)
	}
}
