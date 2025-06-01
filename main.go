package main

import (
	"bytes"
	"context" // Will be needed for marshalling cookies
	"encoding/json"
	"fmt"
	"github.com/chromedp/cdproto/cdp"
	"github.com/luispater/anyAIProxyAPI/internal/runner"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"syscall"
	"time"

	// For cdp.Node
	"github.com/chromedp/chromedp" // For chromedp actions
	"github.com/luispater/anyAIProxyAPI/internal/api"
	"github.com/luispater/anyAIProxyAPI/internal/browser/chrome"
	chromedpmanager "github.com/luispater/anyAIProxyAPI/internal/browser/chrome"
	"github.com/luispater/anyAIProxyAPI/internal/config"
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

	pages := make(map[string]*chrome.Page) // Changed from playwright.Page to context.Context

	// Create API server configuration
	apiConfig := &api.ServerConfig{
		Port:  cfg.ApiPort,
		Debug: cfg.Debug,
		Pages: &pages,
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

	log.Info("Starting Any AI Proxy API application...")

	// Create a new browser manager
	browserManager, errNewManager := chromedpmanager.NewManager(cfg)
	if errNewManager != nil {
		log.Fatalf("could not create browser manager: %v", errNewManager)
		return
	}
	defer func() {
		log.Debugf("Closing browser manager...")
		if err = browserManager.Close(); err != nil {
			log.Debugf("Error closing browser manager: %v", err)
		}
		log.Debugf("Browser manager closed.")
	}()

	// Launch the browser and create a context
	if err = browserManager.LaunchBrowserAndContext(); err != nil {
		log.Fatalf("could not launch browser and context: %v", err)
	}

	log.Debugf("Browser and context launched successfully.")

	for i := 0; i < len(cfg.Instance); i++ {
		log.Debugf("Creating a new page...")
		page, errNewPage := browserManager.NewPage(cfg.Instance[i].URL, cfg.Instance[i].Adapter, cfg.Instance[i].Auth.File) // Modified to use pageCtx and cancelPage
		if errNewPage != nil {
			log.Fatalf("could not create page: %v", errNewPage)
			return
		}

		var currentURL string
		err = chromedp.Run(page.GetContext(), chromedp.Location(&currentURL))
		if err != nil {
			log.Warnf("could not get current URL for instance %s: %v", cfg.Instance[i].Name, err)
			// Continue execution even if URL fetch fails, as navigation might have succeeded.
		} else {
			log.Debugf("Successfully navigated instance %s to: %s. Page loaded.", cfg.Instance[i].Name, currentURL)
		}

		pages[cfg.Instance[i].Name] = page // Store pageCtx

		r, errNewRunnerManager := runner.NewRunnerManager(cfg.Instance[i].Name, cfg.Instance[i].Runner, page, cfg.Debug) // Pass pageCtx
		if errNewRunnerManager != nil {
			log.Error(errNewRunnerManager)
		}
		err = r.Run("init")
		if err != nil {
			log.Debug(err)
		}
		log.Debugf("all of the init system rules are executed.")
	}

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

			log.Debugf("Cleanup completed. Exiting...")
			os.Exit(0)
		case <-time.After(5 * time.Second):
			for instanceName, pageInstance := range pages { // p is pageCtxInstance
				if mapCfg[instanceName].Auth.Check != "" {
					hasCheckFlag := false

					var nodes []*cdp.Node

					timeoutCtx, cancel := context.WithTimeout(pageInstance.GetContext(), 1*time.Second)
					err = chromedp.Run(timeoutCtx,
						chromedp.Nodes(mapCfg[instanceName].Auth.Check, &nodes, chromedp.ByQueryAll),
					)
					cancel()
					if err != nil {
						if err.Error() != "context deadline exceeded" {
							log.Errorf("Error checking auth selector '%s' for instance %s: %v", mapCfg[instanceName].Auth.Check, instanceName, err)
						}
					} else if len(nodes) == 0 {
						log.Debugf("Auth.Check selector '%s' not found for instance %s. Skipping state save.", mapCfg[instanceName].Auth.Check, instanceName)
					} else {
						hasCheckFlag = true
						log.Debugf("Auth.Check selector '%s' found %d elements for instance %s.", mapCfg[instanceName].Auth.Check, len(nodes), instanceName)
					}

					if hasCheckFlag {
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
							cookies, errGetCookies := chromedpmanager.GetCookies(pageInstance.GetContext())
							localStorages, errGetLocalStorages := chromedpmanager.GetLocalStorages(pageInstance.GetContext())
							// localStorages, errGetLocalStorages := pageInstance.GetLocalStorages()
							if errGetCookies != nil {
								log.Debugf("Error getting cookies for instance %s: %v", instanceName, errGetCookies)
								continue
							}
							if errGetLocalStorages != nil {
								log.Debugf("Error getting local storages for instance %s: %v", instanceName, errGetLocalStorages)
								continue
							}

							jsonData, errMarshalIndent := json.MarshalIndent(map[string]interface{}{"cookies": cookies, "local_storage": localStorages}, "", "  ")
							if errMarshalIndent != nil {
								log.Debugf("Error marshalling cookies to JSON for instance %s: %v", instanceName, errMarshalIndent)
								continue
							}

							// Ensure the directory exists
							authAbsPath, errAbs := filepath.Abs(mapCfg[instanceName].Auth.File)
							if errAbs != nil {
								log.Debugf("Error getting absolute path for auth file for instance %s: %v", instanceName, errAbs)
								continue
							}
							authDirName := filepath.Dir(authAbsPath)
							if _, errStat := os.Stat(authDirName); os.IsNotExist(errStat) {
								errMkdir := os.MkdirAll(authDirName, 0755)
								if errMkdir != nil {
									log.Debugf("Error creating directory %s for instance %s: %v", authDirName, instanceName, errMkdir)
									continue
								}
							}

							errWriteFile := os.WriteFile(mapCfg[instanceName].Auth.File, jsonData, 0644)
							if errWriteFile != nil {
								log.Debugf("Error writing auth info to file %s for instance %s: %v", mapCfg[instanceName].Auth.File, instanceName, errWriteFile)
							} else {
								log.Debugf("Successfully wrote auth info to file %s for instance %s", mapCfg[instanceName].Auth.File, instanceName)
							}
						}
					}
				}
			}
		}
	}
}
