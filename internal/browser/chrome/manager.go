package chrome

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/luispater/anyAIProxyAPI/internal/config"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
	log "github.com/sirupsen/logrus"
)

// Manager manages a Chrome browser instance and its contexts.
type Manager struct {
	appConfig     *config.AppConfig
	cfgIndex      int
	allocator     context.Context
	allocCancel   context.CancelFunc
	browserCtx    context.Context
	browserCancel context.CancelFunc
	execPath      string
}

// NewManager creates a new Chromedp Manager instance.
// It initializes the allocator context but does not launch the browser yet.
func NewManager(appConfig *config.AppConfig, cfgIndex int) (*Manager, error) {
	if appConfig == nil {
		return nil, fmt.Errorf("appConfig cannot be nil")
	}
	if cfgIndex < 0 || cfgIndex >= len(appConfig.Chromedp) {
		return nil, fmt.Errorf("cfgIndex %d out of bounds for Chromedp configurations", cfgIndex)
	}

	chromedpCfg := appConfig.Chromedp[cfgIndex]
	execPath := chromedpCfg.BrowserPath
	if execPath == "" {
		// 尝试从环境变量或默认路径查找
		execPath = os.Getenv("CHROME_BIN")
		if execPath == "" {
			// 这里可以根据操作系统添加更多默认路径
			// 例如 macOS: "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
			// Linux: "google-chrome", "chromium-browser"
			// Windows: "C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe"
			// 为了简单起见，我们暂时留空，让 chromedp 自动查找
			log.Warn("Chromedp browser path not specified in config or CHROME_BIN env, will attempt auto-detection.")
		}
	}

	opts := []chromedp.ExecAllocatorOption{
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.Flag("fingerprint", "1000"),
		chromedp.Flag("ignore-certificate-errors", true),
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

	if chromedpCfg.Headless {
		opts = append(opts, chromedp.Flag("headless", true))
		// 在无头模式下，有时需要禁用 GPU
		opts = append(opts, chromedp.Flag("disable-gpu", true))
		// 无头模式下，窗口大小可能需要显式设置
		opts = append(opts, chromedp.WindowSize(1920, 1080))
	}

	if chromedpCfg.UserDataDir != "" {
		opts = append(opts, chromedp.UserDataDir(chromedpCfg.UserDataDir))
	}

	// 处理代理
	if chromedpCfg.Proxy != "" {
		proxyURL, err := url.Parse(chromedpCfg.Proxy)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy URL: %w", err)
		}
		opts = append(opts, chromedp.ProxyServer(proxyURL.String()))
		log.Infof("Using proxy for Chromedp: %s", proxyURL.String())
	}

	// 添加用户代理（如果配置了）
	if chromedpCfg.UserAgent != "" {
		opts = append(opts, chromedp.UserAgent(chromedpCfg.UserAgent))
	} else {
		// 使用一个通用的 UserAgent
		opts = append(opts, chromedp.UserAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36"))
	}

	// 添加额外的启动参数
	for _, arg := range chromedpCfg.Args {
		if arg != "" { // 确保参数不为空
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
		cfgIndex:    cfgIndex,
		allocator:   allocCtx,
		allocCancel: allocCancel,
		execPath:    execPath, // 存储实际使用的路径，即使是自动检测的
	}, nil
}

// LaunchBrowserAndContext launches the browser and creates a new browser context.
func (m *Manager) LaunchBrowserAndContext() error {
	if m.allocator == nil {
		return fmt.Errorf("manager not properly initialized, allocator is nil")
	}

	// 创建浏览器上下文
	// 可以添加 chromedp.WithLogf(log.Printf) 或其他日志记录器
	browserCtx, browserCancel := chromedp.NewContext(
		m.allocator,
		chromedp.WithLogf(log.Infof), // 普通日志
		// chromedp.WithDebugf(log.Debugf), // CDP 命令和事件日志
		// chromedp.WithErrorf(log.Errorf),
	)
	m.browserCtx = browserCtx
	m.browserCancel = browserCancel

	// 运行一个空任务以确保浏览器已启动并准备就绪
	// 这也会实际启动浏览器进程
	if err := chromedp.Run(m.browserCtx); err != nil {
		// 尝试关闭已部分启动的资源
		m.Close()
		return fmt.Errorf("failed to launch browser: %w", err)
	}

	log.Infof("Chromedp browser launched successfully with path: %s", m.execPath)
	return nil
}

// NewPage creates a new "page" in Chromedp.
// In Chromedp, a "page" is typically a new target (tab).
// This function creates a new tab, gets its target ID, and returns a new context associated with that tab.
// The caller is responsible for cancelling the returned context when the page is no longer needed.
func (m *Manager) NewPage() (context.Context, context.CancelFunc, error) {
	if m.browserCtx == nil {
		return nil, nil, fmt.Errorf("browser context not initialized. Call LaunchBrowserAndContext first")
	}

	var newTargetID target.ID
	var newPageCtx context.Context
	var newPageCancel context.CancelFunc

	// 创建一个新的标签页
	err := chromedp.Run(m.browserCtx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			newTargetID, err = target.CreateTarget("about:blank").Do(ctx)
			return err
		}),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create new target (tab): %w", err)
	}

	// 为新的标签页创建一个新的上下文
	newPageCtx, newPageCancel = chromedp.NewContext(m.browserCtx, chromedp.WithTargetID(newTargetID))

	// 确保新页面已加载（至少 about:blank）
	if err := chromedp.Run(newPageCtx, chromedp.Navigate("about:blank")); err != nil {
		newPageCancel() // 清理新创建的上下文
		// 尝试关闭新创建的标签页
		if closeErr := chromedp.Run(m.browserCtx, chromedp.ActionFunc(func(ctx context.Context) error {
			if err := target.CloseTarget(newTargetID).Do(ctx); err != nil {
				return fmt.Errorf("could not close target %s: %w", newTargetID, err)
			}
			return nil
		})); closeErr != nil {
			log.Warnf("Failed to close newly created target %s after navigation error: %v", newTargetID, closeErr)
		}
		return nil, nil, fmt.Errorf("failed to navigate new page to about:blank: %w", err)
	}

	log.Infof("New Chromedp page (targetID: %s) created.", newTargetID)
	return newPageCtx, newPageCancel, nil
}

// Close closes the Chromedp browser instance and cleans up resources.
func (m *Manager) Close() error {
	var firstErr error

	// 首先取消浏览器上下文，这将尝试关闭所有标签页并释放与浏览器会话相关的资源
	if m.browserCancel != nil {
		log.Debug("Cancelling Chromedp browser context...")
		m.browserCancel()
		m.browserCancel = nil // 防止重复取消
		m.browserCtx = nil    // 清除引用
		log.Info("Chromedp browser context cancelled.")
	}

	// 然后取消分配器上下文，这将关闭浏览器进程
	if m.allocCancel != nil {
		log.Debug("Cancelling Chromedp allocator context...")
		m.allocCancel()
		m.allocCancel = nil // 防止重复取消
		m.allocator = nil   // 清除引用
		log.Info("Chromedp allocator context cancelled and browser process shut down.")
	}

	// chromedp.Cancel(m.browserCtx) // 这通常由 browserCancel() 处理
	// chromedp.Cancel(m.allocator)  // 这通常由 allocCancel() 处理

	// 等待浏览器进程完全关闭，可以根据需要添加超时
	// chromedp.Run(m.allocator, browser.Close()) // 这通常由 allocCancel 处理
	// 如果需要更明确的关闭，可以这样做，但这通常在取消分配器时发生。
	// 如果分配器上下文仍然存在（尽管不应该），可以尝试：
	// if m.allocator != nil {
	// 	 if err := chromedp.Run(m.allocator,
	// 	 	chromedp.ActionFunc(func(ctx context.Context) error {
	// 	 		log.Debug("Attempting to explicitly close browser via allocator context...")
	// 	 		return browser.Close().Do(ctx)
	// 	 	}),
	// 	 ); err != nil {
	// 	 	log.Warnf("Error during explicit browser close: %v", err)
	// 	 	if firstErr == nil {
	// 	 		firstErr = fmt.Errorf("error closing browser: %w", err)
	// 	 	}
	// 	 }
	// }

	// 等待一段时间确保进程退出，这在某些情况下有帮助
	// time.Sleep(1 * time.Second) // 可选

	log.Info("Chromedp Manager closed.")
	return firstErr
}

// Helper function to set cookies, if needed later
func SetCookies(ctx context.Context, cookies []*network.CookieParam) error {
	if len(cookies) == 0 {
		return nil
	}
	log.Debugf("Setting %d cookies", len(cookies))
	return network.SetCookies(cookies).Do(ctx)
}

// Helper function to get cookies, if needed later
func GetCookies(ctx context.Context) ([]*network.Cookie, error) {
	log.Debug("Getting cookies")
	cookies, err := network.GetCookies().Do(ctx)
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

// Navigate navigates the given page (context) to the URL.
func Navigate(pageCtx context.Context, url string, timeout time.Duration) error {
	if pageCtx == nil {
		return fmt.Errorf("page context cannot be nil")
	}
	log.Infof("Navigating page to URL: %s with timeout %v", url, timeout)

	navCtx, cancel := context.WithTimeout(pageCtx, timeout)
	defer cancel()

	err := chromedp.Run(navCtx,
		chromedp.Navigate(url),
		// chromedp.WaitVisible("body", chromedp.ByQuery), // 可选：等待页面加载完成的某个标志
	)
	if err != nil {
		return fmt.Errorf("failed to navigate to %s: %w", url, err)
	}
	log.Infof("Successfully navigated to %s", url)
	return nil
}
