package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	pidDirName      = ".pids"
	backendPIDFile  = "backend.pid"
	frontendPIDFile = "frontend.pid"
	backendLogFile  = "backend.log"
	frontendLogFile = "frontend.log"
	serverBinName   = "mcplens-server"
)

// cmdStart builds and starts the backend and/or frontend.
func cmdStart() {
	fs := flag.NewFlagSet("start", flag.ExitOnError)
	backendOnly := fs.Bool("backend", false, "Start only the backend server")
	frontendOnly := fs.Bool("frontend", false, "Start only the frontend dev server")
	fs.Parse(os.Args[2:])

	doBackend, doFrontend := resolveTargets(*backendOnly, *frontendOnly)

	root := mustFindProjectRoot()
	pd := ensurePIDDir(root)

	if doBackend {
		startBackend(root, pd)
	}
	if doFrontend {
		startFrontend(root, pd)
	}
}

// cmdStop stops running backend and/or frontend processes.
func cmdStop() {
	fs := flag.NewFlagSet("stop", flag.ExitOnError)
	backendOnly := fs.Bool("backend", false, "Stop only the backend server")
	frontendOnly := fs.Bool("frontend", false, "Stop only the frontend dev server")
	fs.Parse(os.Args[2:])

	doBackend, doFrontend := resolveTargets(*backendOnly, *frontendOnly)

	root := mustFindProjectRoot()
	pd := filepath.Join(root, pidDirName)

	// Stop frontend first (it proxies to backend)
	if doFrontend {
		stopService("frontend", pd)
	}
	if doBackend {
		stopService("backend", pd)
	}
}

// cmdRestart stops then starts the backend and/or frontend.
func cmdRestart() {
	fs := flag.NewFlagSet("restart", flag.ExitOnError)
	backendOnly := fs.Bool("backend", false, "Restart only the backend server")
	frontendOnly := fs.Bool("frontend", false, "Restart only the frontend dev server")
	fs.Parse(os.Args[2:])

	doBackend, doFrontend := resolveTargets(*backendOnly, *frontendOnly)

	root := mustFindProjectRoot()
	pd := ensurePIDDir(root)

	// Stop
	if doFrontend {
		stopService("frontend", pd)
	}
	if doBackend {
		stopService("backend", pd)
	}

	time.Sleep(1 * time.Second)

	// Start
	if doBackend {
		startBackend(root, pd)
	}
	if doFrontend {
		startFrontend(root, pd)
	}
}

func startBackend(root, pd string) {
	pidFile := filepath.Join(pd, backendPIDFile)

	if pid := readPID(pidFile); pid > 0 && processAlive(pid) {
		fmt.Printf("Backend is already running (PID %d)\n", pid)
		return
	}

	backendDir := filepath.Join(root, "backend")
	binPath := filepath.Join(pd, serverBinName)
	logPath := filepath.Join(pd, backendLogFile)

	// Build the server binary
	fmt.Print("Building backend... ")
	buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/server")
	buildCmd.Dir = backendDir
	if out, err := buildCmd.CombinedOutput(); err != nil {
		fmt.Printf("FAILED\n%s\n", string(out))
		os.Exit(1)
	}
	fmt.Println("OK")

	// Open log file
	lf, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create log file: %v\n", err)
		os.Exit(1)
	}

	// Start the server as a new session (detached from terminal)
	cmd := exec.Command(binPath)
	cmd.Dir = backendDir
	cmd.Stdout = lf
	cmd.Stderr = lf
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := cmd.Start(); err != nil {
		lf.Close()
		fmt.Fprintf(os.Stderr, "Failed to start backend: %v\n", err)
		os.Exit(1)
	}

	pid := cmd.Process.Pid
	os.WriteFile(pidFile, []byte(strconv.Itoa(pid)), 0644)
	cmd.Process.Release()
	lf.Close()

	// Brief check for immediate failure
	time.Sleep(500 * time.Millisecond)
	if !processAlive(pid) {
		fmt.Fprintf(os.Stderr, "Backend failed to start. Check log: %s\n", logPath)
		os.Remove(pidFile)
		os.Exit(1)
	}

	fmt.Printf("Backend started (PID %d)\n", pid)
	fmt.Printf("  Log: %s\n", logPath)
}

func startFrontend(root, pd string) {
	pidFile := filepath.Join(pd, frontendPIDFile)

	if pid := readPID(pidFile); pid > 0 && processAlive(pid) {
		fmt.Printf("Frontend is already running (PID %d)\n", pid)
		return
	}

	frontendDir := filepath.Join(root, "frontend")
	logPath := filepath.Join(pd, frontendLogFile)

	// Use the local vite binary directly to avoid npx overhead
	viteBin := filepath.Join(frontendDir, "node_modules", ".bin", "vite")
	if _, err := os.Stat(viteBin); err != nil {
		fmt.Fprintf(os.Stderr, "Vite not found. Run 'npm install' in the frontend directory first.\n")
		os.Exit(1)
	}

	lf, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create log file: %v\n", err)
		os.Exit(1)
	}

	cmd := exec.Command(viteBin)
	cmd.Dir = frontendDir
	cmd.Stdout = lf
	cmd.Stderr = lf
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := cmd.Start(); err != nil {
		lf.Close()
		fmt.Fprintf(os.Stderr, "Failed to start frontend: %v\n", err)
		os.Exit(1)
	}

	pid := cmd.Process.Pid
	os.WriteFile(pidFile, []byte(strconv.Itoa(pid)), 0644)
	cmd.Process.Release()
	lf.Close()

	// Brief check for immediate failure
	time.Sleep(500 * time.Millisecond)
	if !processAlive(pid) {
		fmt.Fprintf(os.Stderr, "Frontend failed to start. Check log: %s\n", logPath)
		os.Remove(pidFile)
		os.Exit(1)
	}

	fmt.Printf("Frontend started (PID %d)\n", pid)
	fmt.Printf("  Log: %s\n", logPath)
}

func stopService(name, pd string) {
	pidFile := filepath.Join(pd, name+".pid")
	pid := readPID(pidFile)

	if pid <= 0 || !processAlive(pid) {
		fmt.Printf("%s is not running\n", capitalizeStr(name))
		os.Remove(pidFile)
		return
	}

	fmt.Printf("Stopping %s (PID %d)... ", name, pid)

	// Send SIGTERM to the process group (negative PID targets the group)
	syscall.Kill(-pid, syscall.SIGTERM)

	// Wait up to 5 seconds for graceful shutdown
	for i := 0; i < 50; i++ {
		if !processAlive(pid) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Force kill if still alive
	if processAlive(pid) {
		syscall.Kill(-pid, syscall.SIGKILL)
		time.Sleep(200 * time.Millisecond)
	}

	os.Remove(pidFile)
	fmt.Println("stopped")
}

// --- Helpers ---

// resolveTargets determines which services to act on based on flags.
// No flags = both; one flag = only that one; both flags = both.
func resolveTargets(backendOnly, frontendOnly bool) (doBackend, doFrontend bool) {
	doBackend = true
	doFrontend = true
	if backendOnly || frontendOnly {
		doBackend = backendOnly
		doFrontend = frontendOnly
	}
	return
}

// findProjectRoot walks up from cwd looking for a directory with both backend/ and frontend/.
func findProjectRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	dir := cwd
	for {
		if isDirAt(filepath.Join(dir, "backend")) && isDirAt(filepath.Join(dir, "frontend")) {
			return dir, nil
		}
		// Check if we're inside the backend directory
		if isDirAt(filepath.Join(dir, "cmd")) && isDirAt(filepath.Join(dir, "internal")) {
			parent := filepath.Dir(dir)
			if isDirAt(filepath.Join(parent, "frontend")) {
				return parent, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find project root (looking for backend/ and frontend/ directories)")
		}
		dir = parent
	}
}

func mustFindProjectRoot() string {
	root, err := findProjectRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	return root
}

func ensurePIDDir(root string) string {
	pd := filepath.Join(root, pidDirName)
	os.MkdirAll(pd, 0755)
	return pd
}

func isDirAt(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func readPID(file string) int {
	data, err := os.ReadFile(file)
	if err != nil {
		return 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0
	}
	return pid
}

func processAlive(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}

func capitalizeStr(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
