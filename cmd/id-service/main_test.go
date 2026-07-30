package main

import (
	"bytes"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestGracefulShutdown(t *testing.T) {
	bin := buildBinary(t)

	for _, tc := range []struct {
		name string
		sig  os.Signal
	}{
		{"SIGTERM", syscall.SIGTERM},
		{"SIGINT", syscall.SIGINT},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runShutdownTest(t, bin, tc.sig)
		})
	}
}

func buildBinary(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "id-service")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	return bin
}

func runShutdownTest(t *testing.T, bin string, sig os.Signal) {
	t.Helper()

	port := freePort(t)

	cmd := exec.Command(bin)
	cmd.Env = []string{"PORT=" + strconv.Itoa(port), "LOG_LEVEL=debug"}
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = io.Discard

	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() {
		if cmd.ProcessState == nil || !cmd.ProcessState.Exited() {
			_ = cmd.Process.Kill()
			_, _ = cmd.Process.Wait()
		}
	})

	base := "http://127.0.0.1:" + strconv.Itoa(port)
	if err := waitForReady(t, base+"/readyz", 5*time.Second); err != nil {
		t.Fatalf("waitForReady: %v", err)
	}

	if code := probe(t, base+"/readyz"); code != 200 {
		t.Fatalf("readyz baseline: got %d, want 200", code)
	}
	if code := probe(t, base+"/healthz"); code != 200 {
		t.Fatalf("healthz baseline: got %d, want 200", code)
	}

	if err := cmd.Process.Signal(sig); err != nil {
		t.Fatalf("signal: %v", err)
	}

	waitCh := make(chan error, 1)
	go func() { waitCh <- cmd.Wait() }()

	select {
	case err := <-waitCh:
		if err != nil {
			t.Fatalf("exit: got error %v, want clean exit (code 0)", err)
		}
	case <-time.After(15 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("timeout waiting for process exit (15s)")
	}

	logs := stdout.String()
	if !strings.Contains(logs, "shutting down") {
		t.Errorf("log missing %q, got:\n%s", "shutting down", logs)
	}
	if !strings.Contains(logs, "service stopped") {
		t.Errorf("log missing %q, got:\n%s", "service stopped", logs)
	}
}

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("freePort: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	return port
}

var errTimeout = errors.New("timeout waiting for ready")

func waitForReady(t *testing.T, url string, timeout time.Duration) error {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == 200 {
				return nil
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	return errTimeout
}

func probe(t *testing.T, url string) int {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("probe %s: %v", url, err)
	}
	defer resp.Body.Close()
	return resp.StatusCode
}
