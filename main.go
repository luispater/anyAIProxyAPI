package main

import (
	"bytes"
	"context" // Will be needed for marshalling cookies
	"fmt"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"
	"time"

	// For cdp.Node
	"github.com/chromedp/chromedp" // For chromedp actions
	"github.com/luispater/anyAIProxyAPI/internal/api"
	chromedpmanager "github.com/luispater/anyAIProxyAPI/internal/browser/chrome"
	"github.com/luispater/anyAIProxyAPI/internal/config"
	pc "github.com/luispater/anyAIProxyAPI/internal/proxy/config"
	"github.com/luispater/anyAIProxyAPI/internal/proxy/proxy"
	"github.com/luispater/anyAIProxyAPI/internal/runner"

	// "github.com/playwright-community/playwright-go" // Playwright no longer used
	log "github.com/sirupsen/logrus"
)

type LogFormatter struct {
}

func (m *LogFormatter) Format(entry *log.Entry) ([]byte, error) {
	var b *bytes.Buffer
	if entry.Buffer != nil {
		b = entry.Buffer
	} else {
		b = &bytes.Buffer{}
	}

	timestamp := entry.Time.Format("2006-01-02 15:04:05")
	var newLog string
	newLog = fmt.Sprintf("[%s] [%s] [%s:%d] %s\n", timestamp, entry.Level, path.Base(entry.Caller.File), entry.Caller.Line, entry.Message)

	b.WriteString(newLog)
	return b.Bytes(), nil
}

func init() {
	log.SetOutput(os.Stdout)
	log.SetLevel(log.DebugLevel)
	log.SetReportCaller(true)
	log.SetFormatter(&LogFormatter{})
}

func main() {
	// Load application configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Load configuare error: %v", err)
		return
	}
	if !cfg.Debug {
		log.SetLevel(log.InfoLevel)
	}

	// err = playwright.Install(&playwright.RunOptions{ // Playwright no longer used
	// 	Verbose: cfg.Debug,
	// })
	// if err != nil {
	// 	log.Fatalf("Install playwright failed: %v", err)
	// 	return
	// }

	log.Info("Starting Any AI Proxy API application...")

	browserManagers := make([]*chromedpmanager.Manager, 0)

	defer func() {
		for i := 0; i < len(browserManagers); i++ {
			log.Debugf("Closing browser manager...")
			if err = browserManagers[i].Close(); err != nil {
				log.Debugf("Error closing browser manager: %v", err)
			}
			log.Debugf("Browser manager closed.")
		}
	}()

	pages := make(map[string]context.Context) // Changed from playwright.Page to context.Context
	proxies := make(map[string]*proxy.Proxy)

	for i := 0; i < len(cfg.Instance); i++ {
		// Get configuration
		proxyCfg := pc.GetConfig()
		if cfg.Instance[i].ProxyURL != "" {
			proxyCfg.SetProxyServerURL(cfg.Instance[i].ProxyURL)
		}
		proxyCfg.Adapter = cfg.Instance[i].Adapter
		proxyCfg.SetProxyPort(cfg.Instance[i].SniffPort)

		// Add domains to sniff
		domains := strings.Split(cfg.Instance[i].SniffDomain, ",")
		for j := 0; j < len(domains); j++ {
			proxyCfg.AddSniffDomain(strings.TrimSpace(domains[j]))
		}

		// Create a proxy server
		p := proxy.NewProxy(proxyCfg)
		proxies[cfg.Instance[i].Name] = p

		// Start a proxy server in the main goroutine
		go func() {
			if err = p.Start(); err != nil {
				log.Fatalf("Proxy server failed to start: %v", err)
				return
			}
		}()

		// Create a new browser manager
		browserManager, errNewManager := chromedpmanager.NewManager(cfg, i)
		if errNewManager != nil {
			log.Fatalf("could not create browser manager: %v", errNewManager)
			return
		}
		browserManagers = append(browserManagers, browserManager)

		// Launch the browser and create a context
		if err = browserManager.LaunchBrowserAndContext(); err != nil {
			log.Fatalf("could not launch browser and context: %v", err)
		}

		log.Debugf("Browser and context launched successfully. Creating a new page...")

		pageCtx, cancelPage, errNewPage := browserManager.NewPage() // Modified to use pageCtx and cancelPage
		if errNewPage != nil {
			log.Fatalf("could not create page: %v", errNewPage)
			return
		}
		// Note: cancelPage() is not called here immediately as pageCtx is stored and used later.
		// It's assumed that chromedpmanager.Manager.Close() will handle cleanup of pages.
		// If explicit cancellation is needed per page, a mechanism to store and call cancelPage funcs would be required.
		_ = cancelPage // Avoid unused variable error if not used, for now.

		// Navigate to the default URL
		log.Debugf("Navigating to: %s", cfg.Instance[i].URL)
		// TODO: Playwright's WaitUntilStateNetworkidle is not directly available.
		// chromedpmanager.Navigate uses a timeout. For more complex wait, custom actions would be needed.
		// Using a 90-second timeout, similar to the original Playwright config.
		navigationTimeout := 90 * time.Second
		err = chromedpmanager.Navigate(pageCtx, cfg.Instance[i].URL, navigationTimeout)
		if err != nil {
			log.Fatalf("could not navigate to %s: %v", cfg.Instance[i].URL, err)
			return
		}

		var currentURL string
		err = chromedp.Run(pageCtx, chromedp.Location(&currentURL))
		if err != nil {
			log.Warnf("could not get current URL for instance %s: %v", cfg.Instance[i].Name, err)
			// Continue execution even if URL fetch fails, as navigation might have succeeded.
		} else {
			log.Debugf("Successfully navigated instance %s to: %s. Page loaded.", cfg.Instance[i].Name, currentURL)
		}

		pages[cfg.Instance[i].Name] = pageCtx // Store pageCtx

		r, errNewRunnerManager := runner.NewRunnerManager(cfg.Instance[i].Name, cfg.Instance[i].Runner, pageCtx, cfg.Debug) // Pass pageCtx
		if errNewRunnerManager != nil {
			log.Error(errNewRunnerManager)
		}
		err = r.Run("init")
		if err != nil {
			log.Debug(err)
		}
		log.Debugf("all of the init system rules are executed.")
	}

	// Create API server configuration
	apiConfig := &api.ServerConfig{
		Port:   cfg.ApiPort,
		Debug:  cfg.Debug,
		Pages:  pages,
		Proxys: proxies,
	}

	// Create API server
	apiServer := api.NewServer(apiConfig, cfg)

	// Start API server
	go func() {
		log.Infof("Starting API server on port %s", apiConfig.Port)
		if err = apiServer.Start(); err != nil {
			log.Fatalf("API server failed to start: %v", err)
			return
		}
	}()

	mapCfg := make(map[string]config.AppConfigInstance)
	for i := 0; i < len(cfg.Instance); i++ {
		mapCfg[cfg.Instance[i].Name] = cfg.Instance[i]
	}

	// Set up graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-sigChan:
			log.Debugf("Received shutdown signal. Cleaning up...")

			// Create shutdown context
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			_ = ctx // Mark ctx as used to avoid error, as apiServer.Stop(ctx) is commented out

			// Stop API server
			if err = apiServer.Stop(ctx); err != nil {
				log.Debugf("Error stopping API server: %v", err)
			}

			// Stop proxy server
			for _, p := range proxies {
				if err = p.Close(); err != nil {
					log.Debugf("Stop proxy server failed.")
				}
			}

			log.Debugf("Cleanup completed. Exiting...")
			os.Exit(0)
		case <-time.After(5 * time.Second):
			// for instanceName, pageCtxInstance := range pages { // p is pageCtxInstance
			// 	if mapCfg[instanceName].Auth.Check != "" {
			// 		var nodes []*cdp.Node
			// 		err := chromedp.Run(pageCtxInstance,
			// 			chromedp.Nodes(mapCfg[instanceName].Auth.Check, &nodes, chromedp.ByQueryAll),
			// 		)
			// 		if err != nil {
			// 			log.Errorf("Error checking auth selector '%s' for instance %s: %v", mapCfg[instanceName].Auth.Check, instanceName, err)
			// 			continue
			// 		}
			// 		if len(nodes) == 0 {
			// 			log.Debugf("Auth.Check selector '%s' not found for instance %s. Skipping state save.", mapCfg[instanceName].Auth.Check, instanceName)
			// 			continue
			// 		}
			// 		log.Debugf("Auth.Check selector '%s' found %d elements for instance %s.", mapCfg[instanceName].Auth.Check, len(nodes), instanceName)
			// 	}

			// 	saveState := false
			// 	if fileInfo, errStat := os.Stat(mapCfg[instanceName].Auth.File); os.IsNotExist(errStat) {
			// 		saveState = true
			// 	} else {
			// 		lastModified := fileInfo.ModTime()
			// 		now := time.Now()
			// 		duration := now.Sub(lastModified)
			// 		if duration > 5*time.Minute {
			// 			saveState = true
			// 		}
			// 	}

			// 	if saveState {
			// 		api.RequestMutex.Lock()
			// 		// For Chromedp, "StorageState" primarily means cookies for this use case.
			// 		// If localStorage/sessionStorage were needed, chromedp.Evaluate would be used.
			// 		cookies, errGetCookies := chromedpmanager.GetCookies(pageCtxInstance)
			// 		api.RequestMutex.Unlock()

			// 		if errGetCookies != nil {
			// 			log.Debugf("Error getting cookies for instance %s: %v", instanceName, errGetCookies)
			// 			continue
			// 		}

			// 		// The original Playwright storageState likely included more than just cookies.
			// 		// For Chromedp, we are focusing on cookies as the primary state to save.
			// 		// We will marshal the array of cookies directly, wrapped in a "cookies" key
			// 		// to somewhat mimic the structure that might be expected.
			// 		// []*network.Cookie from cdproto should be marshallable.
			// 		jsonData, errMarshalIndent := json.MarshalIndent(map[string]interface{}{"cookies": cookies}, "", "  ")
			// 		if errMarshalIndent != nil {
			// 			log.Debugf("Error marshalling cookies to JSON for instance %s: %v", instanceName, errMarshalIndent)
			// 			continue
			// 		}

			// 		// Ensure the directory exists
			// 		authAbsPath, errAbs := filepath.Abs(mapCfg[instanceName].Auth.File)
			// 		if errAbs != nil {
			// 			log.Debugf("Error getting absolute path for auth file for instance %s: %v", instanceName, errAbs)
			// 			continue
			// 		}
			// 		authDirName := filepath.Dir(authAbsPath)
			// 		if _, errStat := os.Stat(authDirName); os.IsNotExist(errStat) {
			// 			errMkdir := os.MkdirAll(authDirName, 0755)
			// 			if errMkdir != nil {
			// 				log.Debugf("Error creating directory %s for instance %s: %v", authDirName, instanceName, errMkdir)
			// 				continue
			// 			}
			// 		}

			// 		errWriteFile := os.WriteFile(mapCfg[instanceName].Auth.File, jsonData, 0644)
			// 		if errWriteFile != nil {
			// 			log.Debugf("Error writing cookies to file %s for instance %s: %v", mapCfg[instanceName].Auth.File, instanceName, errWriteFile)
			// 		} else {
			// 			log.Debugf("Successfully wrote cookies to file %s for instance %s", mapCfg[instanceName].Auth.File, instanceName)
			// 		}
			// 	}
			// }
		}
	}
}
