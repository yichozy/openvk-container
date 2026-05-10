package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"go.uber.org/zap"
)

const rsyncdConfPath = "/etc/rsyncd.conf"

// Daemon manages an rsync daemon subprocess.
type Daemon struct {
	port       string
	modulePath string
	cmd        *exec.Cmd
}

// NewDaemon creates a Daemon that will listen on port and serve files
// from modulePath under the "viking" module.
func NewDaemon(port, modulePath string) *Daemon {
	return &Daemon{port: port, modulePath: modulePath}
}

// Start writes rsyncd.conf and launches the rsync daemon in foreground mode.
// It blocks until the daemon exits.
func (d *Daemon) Start() error {
	conf := fmt.Sprintf(`[viking]
path = %s
read only = false
use chroot = no
`, d.modulePath)
	if err := os.WriteFile(rsyncdConfPath, []byte(conf), 0644); err != nil {
		return fmt.Errorf("write rsyncd.conf: %w", err)
	}

	d.cmd = exec.Command("rsync",
		"--daemon", "--no-detach",
		"--config="+rsyncdConfPath,
		"--port="+d.port,
	)
	d.cmd.Stdout = os.Stdout
	d.cmd.Stderr = os.Stderr

	zap.L().Info("rsync daemon starting",
		zap.String("port", d.port),
		zap.String("module_path", d.modulePath),
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
