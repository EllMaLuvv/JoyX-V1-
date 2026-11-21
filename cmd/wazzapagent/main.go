package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	whatsapp "github.com/Chomosuke9/WazzapAgent-Go/internal/adapters/whatsapp/hypermeow"
	"github.com/Chomosuke9/WazzapAgent-Go/internal/app"
	"github.com/Chomosuke9/WazzapAgent-Go/internal/backup"
	"github.com/Chomosuke9/WazzapAgent-Go/internal/config"
	"github.com/Chomosuke9/WazzapAgent-Go/internal/observability"
	"github.com/Chomosuke9/WazzapAgent-Go/internal/platform"
)

// defaultPairingWebAddress is where the pairing page listens unless
// WAZZAP_PAIRING_WEB says otherwise. Loopback only: the QR links an account.
const defaultPairingWebAddress = "127.0.0.1:3001"

func main() {
	os.Exit(run())
}

func run() int {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "backup", "verify-backup", "restore-backup":
			return runOfflineCommand(os.Args[1:])
		}
	}

	cfg, err := config.LoadRuntimeBootstrap(os.LookupEnv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid configuration: %v\n", err)
		return 2
	}
	logger, _, err := observability.NewLogger(os.Stdout, cfg.LogLevel(), cfg.LogFormat())
	if err != nil {
		fmt.Fprintf(os.Stderr, "initialize logger: %v\n", err)
		return 2
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	lease, err := platform.AcquireDataRootLease(ctx, cfg.DataDir())
	if err != nil {
		fmt.Fprintf(os.Stderr, "acquire data root: %v\n", err)
		return 1
	}
	defer func() {
		if closeErr := lease.Close(); closeErr != nil {
			fmt.Fprintf(os.Stderr, "release data root: %v\n", closeErr)
		}
	}()
	cfg, err = cfg.ResolveRuntimeIdentity()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load runtime identity: %v\n", err)
		return 2
	}

	var pairing whatsapp.PairingSink
	// The QR is also served as a page, so it can be scanned from a browser on
	// this machine instead of read out of the terminal. Set
	// WAZZAP_PAIRING_WEB=off to skip it; any other value is the listen address.
	sinks := whatsapp.MultiPairingSink{}
	if cfg.PairingOutput() == "terminal" {
		sinks = append(sinks, &whatsapp.TerminalPairingSink{Writer: os.Stdout})
	}
	address := strings.TrimSpace(os.Getenv("WAZZAP_PAIRING_WEB"))
	if address == "" {
		address = defaultPairingWebAddress
	}
	if !strings.EqualFold(address, "off") {
		web := &whatsapp.WebPairingSink{Address: address}
		// Serve the page even while the account is already linked, so the URL
		// always answers instead of only during a pairing attempt.
		web.Start()
		sinks = append(sinks, web)
	}
	if len(sinks) > 0 {
		pairing = sinks
	}
	application := app.New(cfg, logger, app.Options{
		SystemPolicy: app.RenderSystemPolicy(cfg.AssistantName()),
		Pairing:      pairing,
	})
	if err := application.RunCLI(ctx); err != nil {
		logger.Error("application stopped with error", "error", err)
		return 1
	}
	return 0
}

func runOfflineCommand(arguments []string) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	switch arguments[0] {
	case "backup":
		if len(arguments) != 2 {
			fmt.Fprintln(os.Stderr, "usage: wazzapagent backup <destination-parent>")
			return 2
		}
		dataDir, err := config.LoadDataDirRuntime(os.LookupEnv)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid data directory configuration: %v\n", err)
			return 2
		}
		lease, err := platform.AcquireDataRootLease(ctx, dataDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "acquire data root: %v\n", err)
			return 1
		}
		defer lease.Close()
		path, err := backup.Create(ctx, dataDir, arguments[1], time.Now().UTC())
		if err != nil {
			fmt.Fprintf(os.Stderr, "backup failed: %v\n", err)
			return 1
		}
		fmt.Fprintf(os.Stdout, "backup created: %s\n", path)
		return 0
	case "verify-backup":
		if len(arguments) != 2 {
			fmt.Fprintln(os.Stderr, "usage: wazzapagent verify-backup <backup-directory>")
			return 2
		}
		manifest, err := backup.Verify(ctx, arguments[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "backup verification failed: %v\n", err)
			return 1
		}
		fmt.Fprintf(os.Stdout, "backup verified: %d files\n", len(manifest.Files))
		return 0
	case "restore-backup":
		if len(arguments) != 3 {
			fmt.Fprintln(os.Stderr, "usage: wazzapagent restore-backup <backup-directory> <new-data-directory>")
			return 2
		}
		dataDir, err := config.LoadDataDirRuntime(os.LookupEnv)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid data directory configuration: %v\n", err)
			return 2
		}
		lease, err := platform.AcquireDataRootLease(ctx, dataDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "acquire data root: %v\n", err)
			return 1
		}
		defer lease.Close()
		if err := backup.Restore(ctx, arguments[1], arguments[2]); err != nil {
			fmt.Fprintf(os.Stderr, "restore failed: %v\n", err)
			return 1
		}
		fmt.Fprintf(os.Stdout, "backup restored into: %s\n", arguments[2])
		return 0
	default:
		return 2
	}
}
