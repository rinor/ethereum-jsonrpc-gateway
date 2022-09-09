package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/HydroProtocol/ethereum-jsonrpc-gateway/core"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use: "start",
	Run: func(cmd *cobra.Command, args []string) {
		os.Exit(Run())
	},
}

func waitExitSignal(ctxStop context.CancelFunc) {
	var exitSignal = make(chan os.Signal)
	signal.Notify(exitSignal, syscall.SIGTERM)
	signal.Notify(exitSignal, syscall.SIGINT)

	<-exitSignal

	logrus.Info("Stopping...")
	ctxStop()
}

func setLogLevel() {
	lvl, ok := os.LookupEnv("LOG_LEVEL")
	// LOG_LEVEL not set, let's default to debug
	if !ok {
		lvl = "debug"
	}
	// parse string, this is built-in feature of logrus
	ll, err := logrus.ParseLevel(lvl)
	if err != nil {
		ll = logrus.DebugLevel
	}
	// set global log level
	logrus.SetLevel(ll)
	logrus.SetFormatter(&logrus.TextFormatter{TimestampFormat: time.RFC3339, FullTimestamp: true})
}

// ulimits - increase resources
func ulimits() error {
	var rLimit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit); err != nil {
		return fmt.Errorf("error getting rlimit: %s", err)
	}
	rLimit.Cur = rLimit.Max
	if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit); err != nil {
		return fmt.Errorf("error setting ulimit: %s", err)
	}
	return nil
}

func Run() int {
	setLogLevel()

	if err := ulimits(); err != nil {
		logrus.Warnf("Could not change ulimits : %v\n", err)
	}

	ctx, stop := context.WithCancel(context.Background())
	go waitExitSignal(stop)

	quitLoopConfig := make(chan bool)
	core.LoadConfig(ctx, quitLoopConfig)

	go core.StartMonitorHttpServer(ctx)
	httpServer := &http.Server{Addr: ":3005", Handler: &core.Server{}}

	// http server graceful shutdown
	go func() {
		<-ctx.Done()
		quitLoopConfig <- true

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			logrus.Fatalf("Could not gracefully shutdown the server: %v\n", err)
		}
	}()

	logrus.Infof("Listening on http://0.0.0.0%s\n", httpServer.Addr)

	if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
		logrus.Fatal(err)
	}

	logrus.Info("Stopped")
	return 0
}
