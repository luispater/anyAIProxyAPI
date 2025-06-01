package config

import (
	"strings"
	"sync"
)

// Config stores application configuration
type Config struct {
	// List of domains to sniff
	SniffDomains map[string]bool
	// Proxy server port
	ProxyPort string
	// Upstream proxy server URL (http://user:pass@host:port, socks5://user:pass@host:port, etc.)
	ProxyServerURL string
	// Mutex for protecting configuration
	mu sync.RWMutex
}

// Global configuration instance
var globalConfig *Config
var once sync.Once

// GetConfig returns the global configuration instance
func GetConfig() *Config {
	once.Do(func() {
		globalConfig = &Config{
			SniffDomains:   make(map[string]bool),
			ProxyPort:      "3120",
			ProxyServerURL: "",
		}
		// Initialize default configuration
		globalConfig.initDefaultConfig()
	})
	return globalConfig
}

// Initialize default configuration
func (c *Config) initDefaultConfig() {
}

// AddSniffDomain adds a domain to the sniff list
func (c *Config) AddSniffDomain(domain string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.SniffDomains[domain] = true
}

// RemoveSniffDomain removes a domain from the sniff list
func (c *Config) RemoveSniffDomain(domain string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.SniffDomains, domain)
}

// IsSniffDomain checks if a domain should be sniffed
func (c *Config) IsSniffDomain(domain string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Exact match
	if c.SniffDomains[domain] {
		return true
	}

	// Wildcard match (e.g. *.example.com)
	for d := range c.SniffDomains {
		if strings.HasPrefix(d, "*.") {
			suffix := d[1:] // Remove *
			if strings.HasSuffix(domain, suffix) {
				return true
			}
		}
	}

	return false
}

// SetProxyPort sets the proxy port
func (c *Config) SetProxyPort(port string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ProxyPort = port
}

// GetProxyPort gets the proxy port
func (c *Config) GetProxyPort() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ProxyPort
}

// GetSniffDomains returns all domains in the sniff list
func (c *Config) GetSniffDomains() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	domains := make([]string, 0, len(c.SniffDomains))
	for domain := range c.SniffDomains {
		domains = append(domains, domain)
	}
	return domains
}

// SetProxyServerURL sets the proxy server URL
func (c *Config) SetProxyServerURL(url string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ProxyServerURL = url
}

// GetProxyServerURL gets the proxy server URL
func (c *Config) GetProxyServerURL() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ProxyServerURL
}
