package chrome

import (
	"context"
	"fmt"
	"github.com/luispater/anyAIProxyAPI/internal/config"
	"os"
	"strings"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	log "github.com/sirupsen/logrus"
)

// Manager manages a Chrome browser instance and its contexts.
type Manager struct {
	appConfig     *config.AppConfig
	allocator     context.Context
	allocCancel   context.CancelFunc
	browserCtx    context.Context
	browserCancel context.CancelFunc
	execPath      string
}

// NewManager creates a new Chromedp Manager instance.
// It initializes the allocator context but does not launch the browser yet.
func NewManager(appConfig *config.AppConfig) (*Manager, error) {
	if appConfig == nil {
		return nil, fmt.Errorf("appConfig cannot be nil")
	}

	execPath := appConfig.Browser.FingerprintChromiumPath
	if execPath == "" {
		execPath = os.Getenv("CHROME_BIN")
		if execPath == "" {
			log.Warn("Chromedp browser path not specified in config or CHROME_BIN env, will attempt auto-detection.")
		}
	}

	opts := []chromedp.ExecAllocatorOption{
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.Flag("fingerprint", "1000"),
		// chromedp.Flag("ignore-certificate-errors", true),
		// chromedp.Flag("proxy-pac-url", "http://127.0.0.1:2048/proxy.pac"),

		// chromedp.Flag("disable-background-timer-throttling", true),
		// chromedp.Flag("disable-backgrounding-occluded-windows", true),
		// chromedp.Flag("disable-renderer-backgrounding", true),
		// chromedp.Flag("disable-infobars", true),      // 禁用 "Chrome is being controlled by automated test software"
		// chromedp.Flag("disable-breakpad", true),      // 禁用崩溃报告
		// chromedp.Flag("disable-dev-shm-usage", true), // 在 Docker 或 CI 环境中通常需要
		// chromedp.Flag("disable-extensions", true),
		// chromedp.Flag("mute-audio", true),
		// chromedp.Flag("headless", true), // 默认非无头，可以根据配置添加
		// chromedp.Flag("remote-debugging-port", "9222"), // 如果需要远程调试
	}

	if execPath != "" {
		opts = append(opts, chromedp.ExecPath(execPath))
	}

	if appConfig.Headless {
		opts = append(opts, chromedp.Flag("headless", true))
		opts = append(opts, chromedp.Flag("disable-gpu", true))
		opts = append(opts, chromedp.WindowSize(1920, 1080))
	}

	if appConfig.Browser.UserDataDir != "" {
		opts = append(opts, chromedp.UserDataDir(appConfig.Browser.UserDataDir))
	}

	opts = append(opts, chromedp.UserAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36"))

	for _, arg := range appConfig.Browser.Args {
		if arg != "" {
			parts := strings.SplitN(arg, "=", 2)
			if len(parts) == 2 {
				opts = append(opts, chromedp.Flag(strings.TrimPrefix(parts[0], "--"), parts[1]))
			} else {
				opts = append(opts, chromedp.Flag(strings.TrimPrefix(parts[0], "--"), true))
			}
		}
	}

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)

	return &Manager{
		appConfig:   appConfig,
		allocator:   allocCtx,
		allocCancel: allocCancel,
		execPath:    execPath,
	}, nil
}

// LaunchBrowserAndContext launches the browser and creates a new browser context.
func (m *Manager) LaunchBrowserAndContext() error {
	if m.allocator == nil {
		return fmt.Errorf("manager not properly initialized, allocator is nil")
	}

	browserCtx, browserCancel := chromedp.NewContext(
		m.allocator,
		chromedp.WithLogf(log.Infof),
		// chromedp.WithDebugf(log.Debugf),
		// chromedp.WithErrorf(log.Errorf),
	)
	m.browserCtx = browserCtx
	m.browserCancel = browserCancel

	if err := chromedp.Run(m.browserCtx); err != nil {
		_ = m.Close()
		return fmt.Errorf("failed to launch browser: %w", err)
	}

	log.Infof("Chromedp browser launched successfully with path: %s", m.execPath)
	return nil
}

func (m *Manager) NewPage(url, adapterName, authFile string) (*Page, error) {
	if m.browserCtx == nil {
		return nil, fmt.Errorf("browser context not initialized. Call LaunchBrowserAndContext first")
	}

	sniffURL := make([]string, 0)
	for i := 0; i < len(m.appConfig.Instance); i++ {
		sniffURL = append(sniffURL, m.appConfig.Instance[i].SniffURL...)
	}

	return NewPage(m.browserCtx, adapterName, url, authFile, sniffURL)
}

func (m *Manager) Close() error {
	var firstErr error

	if m.browserCancel != nil {
		log.Debug("Cancelling Chromedp browser context...")
		m.browserCancel()
		m.browserCancel = nil
		m.browserCtx = nil
		log.Info("Chromedp browser context cancelled.")
	}

	if m.allocCancel != nil {
		log.Debug("Cancelling Chromedp allocator context...")
		m.allocCancel()
		m.allocCancel = nil
		m.allocator = nil
		log.Info("Chromedp allocator context cancelled and browser process shut down.")
	}

	log.Info("Chromedp Manager closed.")
	return firstErr
}

func GetLocalStorages(pageCtx context.Context) (map[string]string, error) {
	var localStorage map[string]string
	err := chromedp.Run(pageCtx, chromedp.ActionFunc(func(ctx context.Context) error {
		return chromedp.Evaluate(`Object.entries(localStorage).reduce((acc, [key, value]) => ({ ...acc, [key]: value }), {})`, &localStorage).Do(ctx)
	}))
	if err != nil {
		return nil, fmt.Errorf("failed to get localStorage: %w", err)
	}
	return localStorage, nil
}

// Helper function to set cookies, if needed later
func SetCookies(pageCtx context.Context, cookies []*network.CookieParam) error {
	if len(cookies) == 0 {
		return nil
	}

	err := chromedp.Run(pageCtx, chromedp.ActionFunc(func(ctx context.Context) error {
		return network.SetCookies(cookies).Do(ctx)
	}))

	if err != nil {
		return fmt.Errorf("failed to get cookies: %w", err)
	}
	return nil
}

// Helper function to get cookies, if needed later
func GetCookies(pageCtx context.Context) ([]*network.Cookie, error) {
	var cookies []*network.Cookie
	var err error
	err = chromedp.Run(pageCtx, chromedp.ActionFunc(func(ctx context.Context) error {
		cookies, err = network.GetCookies().Do(ctx)
		if err != nil {
			return err
		}
		return nil
	}))

	if err != nil {
		return nil, fmt.Errorf("failed to get cookies: %w", err)
	}
	return cookies, nil
}

// ClearBrowserCache clears the browser cache.
func (m *Manager) ClearBrowserCache() error {
	if m.browserCtx == nil {
		return fmt.Errorf("browser context not initialized")
	}
	log.Info("Clearing browser cache...")
	if err := chromedp.Run(m.browserCtx, network.ClearBrowserCache()); err != nil {
		return fmt.Errorf("failed to clear browser cache: %w", err)
	}
	log.Info("Browser cache cleared.")
	return nil
}

// ClearBrowserCookies clears all browser cookies.
func (m *Manager) ClearBrowserCookies() error {
	if m.browserCtx == nil {
		return fmt.Errorf("browser context not initialized")
	}
	log.Info("Clearing browser cookies...")
	if err := chromedp.Run(m.browserCtx, network.ClearBrowserCookies()); err != nil {
		return fmt.Errorf("failed to clear browser cookies: %w", err)
	}
	log.Info("Browser cookies cleared.")
	return nil
}
