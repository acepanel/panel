package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/bddjr/hlfhr"
	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/libtnb/cron"

	"github.com/acepanel/panel/v3/pkg/config"
	"github.com/acepanel/panel/v3/pkg/tlscert"
	"github.com/acepanel/panel/v3/pkg/types"
)

type Ace struct {
	conf     *config.Config
	server   *hlfhr.Server
	reloader *tlscert.Reloader
	migrator *gormigrate.Gormigrate
	cron     *cron.Cron
	runner   types.TaskRunner
}

func NewAce(
	conf *config.Config,
	cron *cron.Cron,
	migrator *gormigrate.Gormigrate,
	server *hlfhr.Server,
	reloader *tlscert.Reloader,
	runner types.TaskRunner,
) *Ace {
	return &Ace{
		conf:     conf,
		server:   server,
		reloader: reloader,
		migrator: migrator,
		cron:     cron,
		runner:   runner,
	}
}

func (r *Ace) Run() error {
	// migrate database
	if err := r.migrator.Migrate(); err != nil {
		return err
	}
	fmt.Println("[DB] database migrated")

	// start cron scheduler
	if err := r.cron.Start(); err != nil {
		return err
	}
	fmt.Println("[CRON] cron scheduler started")

	// setup graceful shutdown
	sigCtx, stopSignal := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stopSignal()

	// create context for runner
	runnerCtx, runnerCancel := context.WithCancel(sigCtx)
	defer runnerCancel()

	// start task runner
	r.runner.Run(runnerCtx)

	// run http server in goroutine
	serverErr := make(chan error, 1)
	go func() {
		fmt.Println("[HTTP] listening and serving on port", r.conf.HTTP.Port)
		if r.conf.HTTP.IsHTTPS() {
			if err := r.server.ListenAndServeTLS("", ""); !errors.Is(err, http.ErrServerClosed) {
				serverErr <- err
			}
		} else {
			if err := r.server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
				serverErr <- err
			}
		}
		close(serverErr)
	}()

	// wait for shutdown signal or server error
	select {
	case err := <-serverErr:
		if err != nil {
			return err
		}
	case <-sigCtx.Done():
		fmt.Println("[APP] received shutdown signal")
	}

	// graceful shutdown
	fmt.Println("[APP] shutting down gracefully...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// shutdown http server
	shutdownErr := r.server.Shutdown(shutdownCtx)
	if shutdownErr != nil {
		fmt.Println("[HTTP] server shutdown error:", shutdownErr)
	} else {
		fmt.Println("[HTTP] server stopped")
	}

	// stop cron scheduler
	_ = r.cron.Stop(shutdownCtx)
	fmt.Println("[CRON] cron scheduler stopped")

	// wait for task runner
	runnerCancel()
	r.runner.Wait(shutdownCtx)
	fmt.Println("[QUEUE] task runner stopped")

	// close certificate reloader
	if r.reloader != nil {
		if err := r.reloader.Close(); err != nil {
			fmt.Println("[TLS] certificate reloader close error:", err)
		}
	}

	fmt.Println("[APP] shutdown complete")
	return shutdownErr
}
