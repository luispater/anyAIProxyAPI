package browser

import (
	"errors"
	"fmt"
	log "github.com/sirupsen/logrus"
	"os" // Added for os.WriteFile and os.Stat

	"github.com/luispater/anyAIProxyAPI/internal/config"
	"github.com/playwright-community/playwright-go"
)

// Manager holds the Playwright instance, browser instance, and browser context.
type Manager struct {
	pw       *playwright.Playwright
	Browser  playwright.Browser
	Context  playwright.BrowserContext
	Cfg      *config.AppConfig
	cfgIndex int
}

// NewManager creates a new browser manager.
func NewManager(cfg *config.AppConfig, cfgIndex int) (*Manager, error) {
	pw, err := playwright.Run()
	if err != nil {
		return nil, err
	}
	return &Manager{
		pw:       pw,
		Cfg:      cfg,
		cfgIndex: cfgIndex,
	}, nil
}

// LaunchBrowserAndContext launches the configured browser and creates a new context with storage state.
func (m *Manager) LaunchBrowserAndContext() error {
	launchOptions := playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(m.Cfg.Headless),
	}
	if m.Cfg.CamoufoxPath != "" {
		launchOptions.ExecutablePath = playwright.String(m.Cfg.CamoufoxPath)
		log.Debugf("Attempting to launch Camoufox from: %s", m.Cfg.CamoufoxPath)
	} else {
		log.Debug("Camoufox path not specified, launching default Playwright Firefox.")
	}

	browser, err := m.pw.Firefox.Launch(launchOptions)
	if err != nil {
		return err
	}
	m.Browser = browser
	log.Debug("Browser launched successfully.")

	contextOptions := playwright.BrowserNewContextOptions{}
	if m.Cfg.Instance[m.cfgIndex].Auth.File != "" {
		if _, err = os.Stat(m.Cfg.Instance[m.cfgIndex].Auth.File); err == nil {
			log.Infof("Loading authentication state from: %s", m.Cfg.Instance[m.cfgIndex].Auth.File)
			contextOptions.StorageStatePath = playwright.String(m.Cfg.Instance[m.cfgIndex].Auth.File)
		} else if os.IsNotExist(err) {
			log.Infof("Authentication state file not found at %s. Proceeding without loading state. It will be created if you save state.", m.Cfg.Instance[m.cfgIndex].Auth.File)
		} else {
			log.Infof("Error checking auth state file %s: %v. Proceeding without loading state.", m.Cfg.Instance[m.cfgIndex].Auth.File, err)
		}
	}
	contextOptions.Proxy = &playwright.Proxy{
		Server: fmt.Sprintf("http://127.0.0.1:%s/", m.Cfg.Instance[m.cfgIndex].SniffPort),
	}
	contextOptions.IgnoreHttpsErrors = playwright.Bool(true)

	context, err := m.Browser.NewContext(contextOptions)
	if err != nil {
		if bErr := m.Browser.Close(); bErr != nil {
			log.Debugf("Error closing browser after context creation failed: %v", bErr)
		}
		return err
	}
	m.Context = context
	log.Debugf("Browser context created successfully.")
	return nil
}

// NewPage creates a new browser page from the existing context.
func (m *Manager) NewPage() (playwright.Page, error) {
	if m.Context == nil {
		return nil, errors.New("browser context is not initialized. Call LaunchBrowserAndContext first")
	}
	page, err := m.Context.NewPage()
	if err != nil {
		return nil, err
	}
	return page, nil
}

// Close stops the Playwright instance and closes the browser and context.
func (m *Manager) Close() error {
	var firstErr error
	if m.Browser != nil {
		if err := m.Browser.Close(); err != nil {
			log.Debugf("Error closing browser: %v", err)
			firstErr = err
		}
	}
	if m.pw != nil {
		if err := m.pw.Stop(); err != nil {
			log.Debugf("Error stopping playwright: %v", err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}
