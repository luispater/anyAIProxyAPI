package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/luispater/anyAIProxyAPI/internal/runner"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/luispater/anyAIProxyAPI/internal/api"
	"github.com/luispater/anyAIProxyAPI/internal/browser"
	"github.com/luispater/anyAIProxyAPI/internal/config"
	pc "github.com/luispater/anyAIProxyAPI/internal/proxy/config"
	"github.com/luispater/anyAIProxyAPI/internal/proxy/proxy"
	"github.com/playwright-community/playwright-go"
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

	err = playwright.Install(&playwright.RunOptions{
		Verbose: cfg.Debug,
	})
	if err != nil {
		log.Fatalf("Install playwright failed: %v", err)
		return
	}

	log.Info("Starting Any AI Proxy API application...")

	browserManagers := make([]*browser.Manager, 0)

	defer func() {
		for i := 0; i < len(browserManagers); i++ {
			log.Debugf("Closing browser manager...")
			if err = browserManagers[i].Close(); err != nil {
				log.Debugf("Error closing browser manager: %v", err)
			}
			log.Debugf("Browser manager closed.")
		}
	}()

	pages := make(map[string]playwright.Page)
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
		browserManager, errNewManager := browser.NewManager(cfg, i)
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

		page, errNewPage := browserManager.NewPage()
		if errNewPage != nil {
			log.Fatalf("could not create page: %v", errNewPage)
			return
		}

		// Navigate to the default URL
		log.Debugf("Navigating to: %s", cfg.Instance[i].URL)
		// Wait for the page to load completely using "networkidle"
		_, err = page.Goto(cfg.Instance[i].URL, playwright.PageGotoOptions{
			WaitUntil: playwright.WaitUntilStateNetworkidle,
			Timeout:   playwright.Float(90000),
		})
		if err != nil {
			log.Fatalf("could not goto %s: %v", cfg.Instance[i].URL, err)
			return
		}

		log.Debugf("Successfully navigated to: %s. Page fully loaded.", page.URL())

		pages[cfg.Instance[i].Name] = page

		r, errNewRunnerManager := runner.NewRunnerManager(cfg.Instance[i].Name, cfg.Instance[i].Runner, &page, cfg.Debug)
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
			for instanceName, p := range pages {
				if mapCfg[instanceName].Auth.Check != "" {
					checkLocator := pages[instanceName].Locator(mapCfg[instanceName].Auth.Check)
					count, errCount := checkLocator.Count()
					if errCount != nil || count == 0 {
						continue
					}
				}

				saveState := false
				if fileInfo, errStat := os.Stat(mapCfg[instanceName].Auth.File); os.IsNotExist(errStat) {
					saveState = true
				} else {
					lastModified := fileInfo.ModTime()
					now := time.Now()
					duration := now.Sub(lastModified)
					if duration > 5*time.Minute {
						saveState = true
					}
				}

				if saveState {
					storageState, errStorageState := p.Context().StorageState()
					if errStorageState != nil {
						log.Debugf("Error getting storage state: %v", err)
						continue
					}
					jsonData, errMarshalIndent := json.MarshalIndent(storageState, "", "  ")
					if errMarshalIndent != nil {
						log.Debugf("Error marshalling storage state to JSON: %v", err)
						continue
					}

					authAbsPath, errAbs := filepath.Abs(mapCfg[instanceName].Auth.File)
					if errAbs != nil {
						continue
					}
					authDirName := filepath.Dir(authAbsPath)
					_, errStat := os.Stat(filepath.Dir(authAbsPath))
					if os.IsNotExist(errStat) {
						err = os.MkdirAll(authDirName, 0755)
						if err != nil {
							continue
						}
					}

					err = os.WriteFile(mapCfg[instanceName].Auth.File, jsonData, 0644)
					if err != nil {
						log.Debugf("Error writing storage state to file %s: %v", mapCfg[instanceName].Auth.File, err)
					} else {
						log.Debugf("Successfully writing storage state to file %s", mapCfg[instanceName].Auth.File)
					}
				}
			}
		}
	}
}
