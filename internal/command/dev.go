package command

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mugon-net/cli/internal/model"
	"github.com/mugon-net/cli/internal/relay"
	"github.com/mugon-net/cli/internal/service"
	"github.com/mugon-net/cli/internal/templates"
	"github.com/urfave/cli/v3"
)

func withinRoot(rootDir, path string) bool {
	rel, err := filepath.Rel(rootDir, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

func collectModTimes(rootDir string, paths []string) map[string]time.Time {
	modTimes := make(map[string]time.Time)

	for _, path := range paths {
		resolved := path
		if !filepath.IsAbs(resolved) {
			resolved = filepath.Join(rootDir, resolved)
		}
		if !withinRoot(rootDir, resolved) {
			continue
		}

		info, err := os.Stat(resolved)
		if err != nil {
			continue
		}

		if info.IsDir() {
			filepath.Walk(resolved, func(p string, i os.FileInfo, err error) error { //nolint:errcheck
				if err != nil || i.IsDir() || !withinRoot(rootDir, p) {
					return nil
				}
				modTimes[p] = i.ModTime()
				return nil
			})
			continue
		}

		modTimes[resolved] = info.ModTime()
	}

	return modTimes
}

func modTimesChanged(oldTimes, newTimes map[string]time.Time) bool {
	for path, mod := range newTimes {
		if prev, ok := oldTimes[path]; !ok || mod.After(prev) {
			return true
		}
	}
	for path := range oldTimes {
		if _, ok := newTimes[path]; !ok {
			return true
		}
	}
	return false
}

func watchPaths(ctx context.Context, rootDir string, paths []string, onChange func()) {
	modTimes := make(map[string]time.Time)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	initialScanDone := false

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			newModTimes := collectModTimes(rootDir, paths)

			if initialScanDone && modTimesChanged(modTimes, newModTimes) {
				fmt.Printf("Files have changed, triggering rebuild...\n")
				onChange()
			}

			modTimes = newModTimes
			initialScanDone = true
		}
	}
}

// requestHostname returns the hostname the browser used to reach this
// request, formatted so it can be followed by ":<port>" (IPv6 literals keep
// their brackets, e.g. "[::1]").
func requestHostname(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.Host)
	if err != nil {
		return r.Host
	}
	if strings.Contains(host, ":") {
		return "[" + host + "]"
	}
	return host
}

func findFreePort(startPort int) (net.Listener, int, error) {
	for port := startPort; port < startPort+100; port++ {
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err == nil {
			return ln, port, nil
		}
	}
	return nil, 0, fmt.Errorf("no free port found in range %d-%d", startPort, startPort+99)
}

func ExecuteDevCommand(ctx context.Context, c *cli.Command) error {
	globalConfig := model.GetGlobalConfig(ctx)
	projectConfig := model.GetProjectConfig(ctx)

	mainLn, mainPort, err := findFreePort(3000)
	if err != nil {
		return err
	}

	childframeLn, gameFramePort, err := findFreePort(mainPort + 1)
	if err != nil {
		return err
	}

	webrtcLn, webrtcPort, err := findFreePort(gameFramePort + 1)
	if err != nil {
		return err
	}

	parentframeFS, err := templates.ParentframeFileSystem(*projectConfig, mainPort, gameFramePort, webrtcPort)
	if err != nil {
		return fmt.Errorf("failed to load parentframe: %w", err)
	}

	mainServer := &http.Server{Handler: http.FileServer(parentframeFS)}
	// TODO Add Middleware that checks the path of the served file and overwrites the csp meta tag for the index.html
	childframeHandler := http.FileServer(http.Dir(projectConfig.DistributionDir))
	childframeServer := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			host := requestHostname(r)
			w.Header().Set(
				"Content-Security-Policy",
				"default-src 'none'; "+
					fmt.Sprintf("script-src   http://%v:%v/ 'unsafe-inline' 'wasm-unsafe-eval'; ", host, gameFramePort)+
					fmt.Sprintf("connect-src  http://%v:%v/; ", host, gameFramePort)+
					fmt.Sprintf("img-src      http://%v:%v/; ", host, gameFramePort)+
					fmt.Sprintf("style-src    http://%v:%v/ 'unsafe-inline'; ", host, gameFramePort)+
					fmt.Sprintf("font-src     http://%v:%v/; ", host, gameFramePort)+
					fmt.Sprintf("media-src    http://%v:%v/; ", host, gameFramePort)+
					fmt.Sprintf("worker-src   http://%v:%v/ blob:; ", host, gameFramePort)+
					"base-uri     'none';"+
					"form-action  'none';"+
					fmt.Sprintf("frame-ancestors http://%v:%v/; ", host, mainPort)+
					"sandbox allow-scripts allow-pointer-lock;")
			w.Header().Set("Access-Control-Allow-Origin", "*")
			childframeHandler.ServeHTTP(w, r)
		}),
	}

	webrtcRelay := relay.New()
	webrtcServer := &http.Server{Handler: webrtcRelay.Handler()}

	ctx, stop := context.WithCancel(ctx)
	defer stop()

	var mu sync.Mutex
	var debounceTimer *time.Timer

	triggerBuild := func() {
		commandConfig, err := projectConfig.GetCommand("build", "dev")
		if err != nil {
			commandConfig, err = projectConfig.GetCommand("build", model.DefaultCommandScope)
			if err != nil {
				fmt.Fprintf(os.Stderr, "build command not found: %v\n", err)
				return
			}
		}
		if err := service.ExecuteProjectCommand(commandConfig, *globalConfig, *projectConfig); err != nil {
			fmt.Fprintf(os.Stderr, "build failed: %v\n", err)
		}
	}

	triggerBuildDebounced := func() {
		mu.Lock()
		defer mu.Unlock()
		if debounceTimer != nil {
			debounceTimer.Stop()
		}
		debounceTimer = time.AfterFunc(200*time.Millisecond, triggerBuild)
	}
	triggerBuild()

	go watchPaths(ctx, projectConfig.RootDir, projectConfig.WatchPaths, triggerBuildDebounced)

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = mainServer.Shutdown(shutdownCtx)
		_ = childframeServer.Shutdown(shutdownCtx)
		_ = webrtcServer.Shutdown(shutdownCtx)
	}()

	errCh := make(chan error, 3)
	go func() {
		if err := mainServer.Serve(mainLn); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()
	go func() {
		if err := childframeServer.Serve(childframeLn); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()
	go func() {
		if err := webrtcServer.Serve(webrtcLn); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	fmt.Printf("Dev server listening on http://localhost:%d\n", mainPort)

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return nil
	}
}
