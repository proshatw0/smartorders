package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"reflect"
	"runtime"
	"smartorders/user-svc/internal/api"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type stringFlag struct {
	set bool
	val string
}

func (s *stringFlag) String() string       { return s.val }
func (s *stringFlag) Set(v string) error   { s.val, s.set = v, true; return nil }
func newStringFlag(def string) *stringFlag { return &stringFlag{val: def} }

type boolFlag struct {
	set bool
	val bool
}

func (b *boolFlag) String() string {
	if b.val {
		return "true"
	}
	return "false"
}
func (b *boolFlag) Set(v string) error {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", "1", "t", "true", "yes", "y", "on":
		b.val, b.set = true, true
	case "0", "f", "false", "no", "n", "off":
		b.val, b.set = false, true
	default:
		return fmt.Errorf("invalid boolean: %q", v)
	}
	return nil
}
func newBoolFlag(def bool) *boolFlag { return &boolFlag{val: def} }

func Run(ctx context.Context, args []string) error {
	cfg := DefaultServerConfig()

	fs := flag.NewFlagSet("server", flag.ContinueOnError)

	hostF := newStringFlag(cfg.Host)
	portF := newStringFlag(cfg.Port)
	addrF := newStringFlag("")
	bgF := newBoolFlag(false)
	stopF := newBoolFlag(false)
	logF := newStringFlag(cfg.LogFile)
	pidF := newStringFlag(cfg.PidFile)

	fs.Var(hostF, "host", "Host to bind (e.g. 0.0.0.0)")
	fs.Var(portF, "port", "Port to bind (e.g. 8001)")
	fs.Var(addrF, "addr", "Address in one arg (host:port or :port)")
	fs.Var(bgF, "bg", "Run in background (detach child process)")
	fs.Var(stopF, "stop", "Stop a running service via pidfile")
	fs.Var(logF, "logfile", "Log file path when -bg is set (default from config)")
	fs.Var(pidF, "pidfile", "PID file path (default from config)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	rest := fs.Args()
	if len(rest) > 0 {
		applyPositional(&cfg, rest[0])
	}

	if addrF.set && addrF.val != "" {
		h, p, err := splitAddr(addrF.val)
		if err != nil {
			return fmt.Errorf("-addr: %w", err)
		}
		if h != "" {
			cfg.Host = h
		}
		if p != "" {
			cfg.Port = p
		}
	}
	if hostF.set {
		cfg.Host = hostF.val
	}
	if portF.set {
		if _, err := normalizePort(portF.val); err != nil {
			return fmt.Errorf("-port: %w", err)
		}
		cfg.Port = portF.val
	}
	if logF.set {
		cfg.LogFile = logF.val
	}
	if pidF.set {
		cfg.PidFile = pidF.val
	}

	if stopF.set && stopF.val {
		pidfile := strings.TrimSpace(cfg.PidFile)
		if pidfile == "" {
			return fmt.Errorf("pidfile is empty (check config or pass -pidfile)")
		}
		return stopByPidfile(pidfile, 8*time.Second)
	}

	host, port, err := finalizeHostPort(cfg.Host, cfg.Port)
	if err != nil {
		return err
	}
	addr := net.JoinHostPort(host, port)

	if bgF.set && bgF.val {
		logfile := strings.TrimSpace(cfg.LogFile)
		if logfile == "" {
			return fmt.Errorf("logfile is empty (check config or pass -logfile)")
		}
		pidfile := strings.TrimSpace(cfg.PidFile)
		if pidfile == "" {
			return fmt.Errorf("pidfile is empty (check config or pass -pidfile)")
		}

		if err := relaunchInBackground(logfile, pidfile); err != nil {
			return fmt.Errorf("background start failed: %w", err)
		}
		fmt.Printf("started in background, pidfile=%s, logfile=%s\n", pidfile, logfile)
		return nil
	}

	srv := &http.Server{
		Addr:         addr,
		Handler:      api.NewRouter(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 20 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		fmt.Printf(
			"listening on http://%s (log=%s, pid=%s)\n",
			addr, cfg.LogFile, cfg.PidFile,
		)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	select {
	case <-ctx.Done():
	case err := <-errCh:
		return err
	case <-sigCh:
	}

	shCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shCtx)
}

func applyPositional(cfg *Config, arg string) {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return
	}
	if isJustPort(arg) {
		cfg.Port = arg
		return
	}
	if h, p, err := splitAddr(arg); err == nil {
		if h != "" {
			cfg.Host = h
		}
		if p != "" {
			cfg.Port = p
		}
	}
}

func isJustPort(s string) bool {
	if strings.Contains(s, ":") {
		return false
	}
	_, err := normalizePort(s)
	return err == nil
}

func splitAddr(s string) (host, port string, err error) {
	if strings.HasPrefix(s, ":") {
		p := strings.TrimPrefix(s, ":")
		if _, err := normalizePort(p); err != nil {
			return "", "", fmt.Errorf("invalid port %q", p)
		}
		return "", p, nil
	}
	h, p, e := net.SplitHostPort(s)
	if e != nil {
		if !strings.Contains(s, ":") {
			return s, "", nil
		}
		return "", "", fmt.Errorf("invalid address %q: %w", s, e)
	}
	if p != "" {
		if _, err := normalizePort(p); err != nil {
			return "", "", fmt.Errorf("invalid port %q", p)
		}
	}
	return h, p, nil
}

func normalizePort(p string) (string, error) {
	n, err := strconv.Atoi(strings.TrimSpace(p))
	if err != nil || n < 1 || n > 65535 {
		return "", fmt.Errorf("port must be 1..65535, got %q", p)
	}
	return strconv.Itoa(n), nil
}

func finalizeHostPort(host, port string) (string, string, error) {
	if host == "" {
		host = "0.0.0.0"
	}
	if port == "" {
		port = "8001"
	}
	np, err := normalizePort(port)
	if err != nil {
		return "", "", err
	}
	return host, np, nil
}

func relaunchInBackground(logfile, pidfile string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}

	filtered := filterOutFlags(os.Args[1:], map[string]bool{
		"-bg": true, "--bg": true, "-logfile": true, "--logfile": true, "-pidfile": true, "--pidfile": true,
	})

	if dir := filepath.Dir(logfile); dir != "." && dir != "" {
		_ = os.MkdirAll(dir, 0o755)
	}
	f, err := os.OpenFile(logfile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open logfile: %w", err)
	}

	cmd := exec.Command(exe, filtered...)
	cmd.Stdout = f
	cmd.Stderr = f
	cmd.Stdin = nil

	attr := &syscall.SysProcAttr{}
	if runtime.GOOS == "windows" {
		setUintField(attr, "CreationFlags", 0x00000200)
	} else {
		if !setBoolField(attr, "Setpgid", true) {
			setBoolField(attr, "Setsid", true)
		}
	}
	// если хоть что-то установили — присвоим
	if anyFieldSet(attr) {
		cmd.SysProcAttr = attr
	}

	if err := cmd.Start(); err != nil {
		_ = f.Close()
		return err
	}

	if dir := filepath.Dir(pidfile); dir != "." && dir != "" {
		_ = os.MkdirAll(dir, 0o755)
	}
	if err := os.WriteFile(pidfile, []byte(strconv.Itoa(cmd.Process.Pid)), 0o644); err != nil {
		fmt.Fprintf(f, "warn: cannot write pidfile %s: %v\n", pidfile, err)
	}

	return nil
}

func setBoolField(attr *syscall.SysProcAttr, name string, val bool) bool {
	v := reflect.ValueOf(attr).Elem()
	f := v.FieldByName(name)
	if f.IsValid() && f.CanSet() && f.Kind() == reflect.Bool {
		f.SetBool(val)
		return true
	}
	return false
}

func setUintField(attr *syscall.SysProcAttr, name string, val uint32) bool {
	v := reflect.ValueOf(attr).Elem()
	f := v.FieldByName(name)
	if f.IsValid() && f.CanSet() && (f.Kind() == reflect.Uint32 || f.Kind() == reflect.Uint) {
		f.SetUint(uint64(val))
		return true
	}
	return false
}

func anyFieldSet(attr *syscall.SysProcAttr) bool {
	v := reflect.ValueOf(attr).Elem()
	for _, n := range []string{"Setpgid", "Setsid", "CreationFlags"} {
		f := v.FieldByName(n)
		if f.IsValid() {
			switch f.Kind() {
			case reflect.Bool:
				if f.Bool() {
					return true
				}
			case reflect.Uint32, reflect.Uint:
				if f.Uint() != 0 {
					return true
				}
			}
		}
	}
	return false
}

func filterOutFlags(args []string, names map[string]bool) []string {
	var out []string
	skipNext := false
	for i := 0; i < len(args); i++ {
		if skipNext {
			skipNext = false
			continue
		}
		a := args[i]
		key, val, hasEq := strings.Cut(a, "=")
		if names[key] {
			if !hasEq && (i+1) < len(args) && !strings.HasPrefix(args[i+1], "-") {
				skipNext = true
			}
			continue
		}
		if hasEq {
			out = append(out, key+"="+val)
		} else {
			out = append(out, a)
		}
	}
	return out
}

func stopByPidfile(pidfile string, timeout time.Duration) error {
	b, err := os.ReadFile(pidfile)
	if err != nil {
		return fmt.Errorf("read pidfile: %w", err)
	}
	pidStr := strings.TrimSpace(string(b))
	pid, err := strconv.Atoi(pidStr)
	if err != nil || pid <= 0 {
		return fmt.Errorf("bad pid in %s: %q", pidfile, pidStr)
	}

	if runtime.GOOS == "windows" {
		cmd := exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/T", "/F")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("taskkill failed: %w", err)
		}
		_ = os.Remove(pidfile)
		return nil
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find process: %w", err)
	}

	_ = proc.Signal(syscall.SIGTERM)

	dead := make(chan struct{})
	go func() {
		for i := 0; i < int(timeout/time.Millisecond); i++ {
			if !processExists(pid) {
				close(dead)
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		close(dead)
	}()

	select {
	case <-dead:
		if processExists(pid) {
			_ = proc.Signal(syscall.SIGKILL)
		}
	case <-time.After(timeout):
		_ = proc.Signal(syscall.SIGKILL)
	}

	_ = os.Remove(pidfile)
	return nil
}

func processExists(pid int) bool {
	if runtime.GOOS == "windows" {
		_, err := os.FindProcess(pid)
		return err == nil
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}
