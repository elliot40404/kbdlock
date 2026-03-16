package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"

	"github.com/elliot40404/kbdlock/internal/appconsole"
	"github.com/elliot40404/kbdlock/internal/cli"
	"github.com/elliot40404/kbdlock/internal/config"
	"github.com/elliot40404/kbdlock/internal/control"
	"github.com/elliot40404/kbdlock/internal/controller"
	"github.com/elliot40404/kbdlock/internal/instance"
	"github.com/elliot40404/kbdlock/internal/ipc"
	"github.com/elliot40404/kbdlock/internal/launch"
	"github.com/elliot40404/kbdlock/internal/logger"
	"github.com/elliot40404/kbdlock/internal/notify"
	"github.com/elliot40404/kbdlock/internal/sentinel"
	"github.com/elliot40404/kbdlock/internal/tray"
)

var version = "dev"

type controlHandler struct {
	pid    int
	ctrl   *controller.Controller
	onStop func()
}

func (h *controlHandler) LockCommand() error   { return h.ctrl.LockCommand() }
func (h *controlHandler) UnlockCommand() error { return h.ctrl.UnlockCommand() }
func (h *controlHandler) StopCommand()         { h.onStop() }
func (h *controlHandler) PID() int             { return h.pid }

func main() {
	os.Exit(run())
}

func run() int {
	args := os.Args[1:]

	if startupStatusPath, ok := launch.ParseInternalStartupStatusPath(args); ok {
		return runTrayApp(startupStatusPath)
	}

	if len(args) > 0 {
		code := cli.Run(args, version)
		switch code {
		case cli.StartDetached:
			return runDetachedLaunch()
		case cli.StopBackground:
			return runControlStop()
		case cli.LockKeyboard:
			return runControlLock()
		case cli.UnlockKeyboard:
			return runControlUnlock()
		case cli.ContinueWithGUI:
			// No-op; only possible for zero args, handled below.
		default:
			return code
		}
	}

	if appconsole.HasAttachedTerminal() {
		return runDetachedLaunch()
	}

	return runTrayApp("")
}

func runDetachedLaunch() int {
	if err := launch.StartDetached(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	return 0
}

func runControlStop() int {
	client, err := control.Connect()
	if err != nil {
		return reportControlError(err)
	}

	pid, err := client.PID()
	_ = client.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	client, err = control.Connect()
	if err != nil {
		return reportControlError(err)
	}
	defer func() {
		_ = client.Close()
	}()

	if err := client.Stop(); err != nil {
		return reportControlError(err)
	}

	if err := control.WaitForExitOrKill(pid, control.StopTimeout()); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	return 0
}

func runControlLock() int {
	client, err := control.Connect()
	if err != nil {
		return reportControlError(err)
	}
	defer func() {
		_ = client.Close()
	}()

	if err := client.Lock(); err != nil {
		return reportControlError(err)
	}
	return 0
}

func runControlUnlock() int {
	client, err := control.Connect()
	if err != nil {
		return reportControlError(err)
	}
	defer func() {
		_ = client.Close()
	}()

	if err := client.Unlock(); err != nil {
		return reportControlError(err)
	}
	return 0
}

func reportControlError(err error) int {
	if errors.Is(err, control.ErrNotRunning) {
		fmt.Fprintln(os.Stderr, "kbdlock is not running")
		return 1
	}
	if errors.Is(err, control.ErrControlUnavailable) {
		fmt.Fprintln(os.Stderr, "kbdlock is already running, but the running instance is from an older build and does not support lock/unlock/stop. Quit it once from the tray or Task Manager, then start the new build.")
		return 1
	}

	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	return 1
}

func runTrayApp(startupStatusPath string) int {
	reporter := launch.NewReporter(startupStatusPath)

	if !appconsole.HasAttachedTerminal() {
		appconsole.HideIfStandalone()
	}

	instanceGuard, err := instance.AcquireController()
	if errors.Is(err, instance.ErrAlreadyRunning) {
		reporter.ReportAlreadyRunning()
		return 0
	}
	if err != nil {
		reporter.ReportFailed(fmt.Sprintf("instance guard: %v", err))
		return 1
	}
	defer func() {
		_ = instanceGuard.Close()
	}()

	log, err := logger.New()
	if err != nil {
		reporter.ReportFailed(fmt.Sprintf("logger: %v", err))
		return 1
	}
	defer log.Close()

	log.Info("kbdlock %s starting", version)
	tray.Version = version

	cfg, err := config.Load()
	if err != nil {
		log.Error("config: %v", err)
		reporter.ReportFailed(fmt.Sprintf("config: %v", err))
		return 1
	}

	if cfg.Notifications {
		if err := notify.EnsureReady(); err != nil {
			log.Error("notification setup failed: %v", err)
		}
	}

	vkCodes, err := config.ComboToVKCodes(cfg.UnlockCombo)
	if err != nil {
		log.Error("combo: %v", err)
		reporter.ReportFailed(fmt.Sprintf("combo: %v", err))
		return 1
	}

	mgr := sentinel.New(log)
	if err := mgr.Start(); err != nil {
		log.Error("sentinel start: %v", err)
		reporter.ReportFailed(fmt.Sprintf("sentinel start: %v", err))
		return 1
	}

	client, err := mgr.Connect()
	if err != nil {
		log.Error("sentinel connect: %v", err)
		reporter.ReportFailed(fmt.Sprintf("sentinel connect: %v", err))
		mgr.Stop()
		return 1
	}

	if err := client.SetCombo(vkCodes); err != nil {
		log.Error("set combo: %v", err)
	}

	ctrl := controller.New(log, cfg, mgr, client)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mgr.StartWatchdog(ctx, func(newClient *ipc.Client) {
		ctrl.SetClient(newClient)
		if err := newClient.SetCombo(vkCodes); err != nil {
			log.Error("re-set combo after restart: %v", err)
		}
	})

	ctrl.StartMouseCorner(ctx)
	ctrl.StartHotkey(ctx)
	ctrl.StartStateSync(ctx)

	var (
		shutdownOnce sync.Once
		controlSrv   *control.Server
	)

	shutdown := func() {
		shutdownOnce.Do(func() {
			if controlSrv != nil {
				_ = controlSrv.Close()
			}
			ctrl.Stop()
			mgr.Stop()
			cancel()
		})
	}

	controlSrv, err = control.Listen(&controlHandler{
		pid:  os.Getpid(),
		ctrl: ctrl,
		onStop: func() {
			shutdown()
			tray.Quit()
		},
	})
	if err != nil {
		log.Error("control pipe listen: %v", err)
		reporter.ReportFailed(fmt.Sprintf("control pipe listen: %v", err))
		shutdown()
		return 1
	}
	go controlSrv.Run()

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt)
		defer signal.Stop(sigCh)

		<-sigCh
		shutdown()
		tray.Quit()
	}()

	tray.Run(tray.Actions{
		OnLock:   ctrl.Lock,
		OnUnlock: ctrl.Unlock,
		OnReady:  reporter.ReportStarted,
		OnQuit:   shutdown,
	})

	log.Info("kbdlock exiting")
	return 0
}
