package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"go.uber.org/zap"
)

// Daemon manages an rsync daemon subprocess.
type Daemon struct {
	port       string
	configPath string
	cmd        *exec.Cmd
}

// NewDaemon creates a Daemon that will listen on port and serve files
// from modulePath under the "viking" module.
func NewDaemon(port, configPath string) *Daemon {
	return &Daemon{port: port, configPath: configPath}
}

// Start writes rsyncd.conf and launches the rsync daemon in foreground mode.
// It blocks until the daemon exits.
func (d *Daemon) Start() error {
	d.cmd = exec.Command("rsync",
		"--daemon", "--no-detach",
		"--config="+d.configPath,
		"--port="+d.port,
	)
	d.cmd.Stdout = os.Stdout
	d.cmd.Stderr = os.Stderr

	zap.L().Info("rsync daemon starting",
		zap.String("port", d.port),
		zap.String("config_path", d.configPath),
	)

	if err := d.cmd.Start(); err != nil {
		return fmt.Errorf("start rsync daemon: %w", err)
	}

	return d.cmd.Wait()
}

// Stop gracefully shuts down the rsync daemon with SIGINT, then waits for exit.
func (d *Daemon) Stop() {
	if d.cmd == nil || d.cmd.Process == nil {
		return
	}
	zap.L().Info("stopping rsync daemon")
	d.cmd.Process.Signal(syscall.SIGINT)
}
