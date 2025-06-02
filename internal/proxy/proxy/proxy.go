package proxy

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"github.com/luispater/anyAIProxyAPI/internal/proxy/adapter"
	"github.com/luispater/anyAIProxyAPI/internal/proxy/config"
	"github.com/luispater/anyAIProxyAPI/internal/proxy/model"
	"github.com/luispater/anyAIProxyAPI/internal/proxy/utils"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/proxy"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

// Proxy proxy struct
type Proxy struct {
	config   *config.Config
	server   *http.Server
	queue    *utils.Queue[*model.ProxyResponse]
	adapter  *adapter.Adapter
	sniffing bool
}

// createProxyDialer creates a dialer for the specified proxy URL
func createProxyDialer(proxyURL string) (proxy.Dialer, error) {
	if proxyURL == "" {
		return proxy.Direct, nil
	}

	proxyURLParsed, err := url.Parse(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("invalid proxy URL: %v", err)
	}

	var dialer proxy.Dialer
	switch proxyURLParsed.Scheme {
	case "http", "https":
		dialer = &httpProxyDialer{proxyURL: proxyURLParsed}
	case "socks5":
		auth := &proxy.Auth{}
		if proxyURLParsed.User != nil {
			auth.User = proxyURLParsed.User.Username()
			auth.Password, _ = proxyURLParsed.User.Password()
			dialer, err = proxy.SOCKS5("tcp", proxyURLParsed.Host, auth, proxy.Direct)
		} else {
			dialer, err = proxy.SOCKS5("tcp", proxyURLParsed.Host, nil, proxy.Direct)
		}
	default:
		return nil, fmt.Errorf("unsupported proxy scheme: %s", proxyURLParsed.Scheme)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create proxy dialer: %v", err)
	}

	return dialer, nil
}

// httpProxyDialer implements proxy.Dialer for HTTP proxies
type httpProxyDialer struct {
	proxyURL *url.URL
}

// Dial connects to the address through the HTTP proxy
func (d *httpProxyDialer) Dial(_, addr string) (net.Conn, error) {
	connectReq := &http.Request{
		Method: "CONNECT",
		URL:    &url.URL{Opaque: addr},
		Host:   addr,
		Header: make(http.Header),
	}

	if d.proxyURL.User != nil {
		password, _ := d.proxyURL.User.Password()
		auth := d.proxyURL.User.Username() + ":" + password
		basicAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(auth))
		connectReq.Header.Set("Proxy-Authorization", basicAuth)
	}

	conn, err := net.Dial("tcp", d.proxyURL.Host)
	if err != nil {
		return nil, err
	}

	if err = connectReq.Write(conn); err != nil {
		_ = conn.Close()
		return nil, err
	}

	respReader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(respReader, connectReq)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != 200 {
		_ = conn.Close()
		return nil, fmt.Errorf("proxy error: %s", resp.Status)
	}

	return conn, nil
}

// NewProxy create a new proxy object
func NewProxy(cfg *config.Config) *Proxy {
	return &Proxy{
		config: cfg,
		queue:  utils.NewQueue[*model.ProxyResponse](),
	}
}

func (p *Proxy) StartSniffing() {
	p.sniffing = true
}

func (p *Proxy) StopSniffing() {
	p.sniffing = false
	p.queue.Clear()
}

// Start Start proxy server
func (p *Proxy) Start() error {
	var adapterInstance adapter.Adapter
	var ok bool
	if adapterInstance, ok = adapter.Adapters[p.config.Adapter]; ok {
		p.adapter = &adapterInstance
	} else {
		log.Errorf("Adapter %s not found", p.config.Adapter)
	}

	addr := ":" + p.config.GetProxyPort()
	p.server = &http.Server{
		Addr: addr,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodConnect {
				p.handleHTTPS(w, r)
			} else {
				p.handleHTTP(w, r)
			}
		}),
	}

	log.Debugf("Proxy server started on %s", addr)
	return p.server.ListenAndServe()
}

func (p *Proxy) shouldRecord(buffer []byte) bool {
	if p.adapter != nil {
		adp := *p.adapter
		return adp.ShouldRecord(buffer)
	}
	return false
}

func (p *Proxy) handleResponse(responseBuffer chan []byte, disconnect chan bool, sniffing *bool, queue *utils.Queue[*model.ProxyResponse]) {
	if p.adapter != nil {
		adp := *p.adapter
		adp.HandleResponse(responseBuffer, disconnect, sniffing, queue)
	} else {
		go func() {
		outLoop:
			for {
				select {
				case <-responseBuffer:
				case <-disconnect:
					break outLoop
				}
			}
		}()
	}
}

func (p *Proxy) Close() error {
	if p.server != nil {
		return p.server.Close()
	}
	return nil
}

// handleHTTP handle HTTP request
func (p *Proxy) handleHTTP(w http.ResponseWriter, r *http.Request) {
	// check proxy config
	proxyURL := p.config.GetProxyServerURL()
	if proxyURL != "" {
		// create custom transport
		transport := &http.Transport{}

		// parse URL
		parsedURL, err := url.Parse(proxyURL)
		if err != nil {
			log.Debugf("Failed to parse proxy URL: %v", err)
			http.Error(w, "Proxy configuration error", http.StatusInternalServerError)
			return
		}

		// set transport type by schema
		switch parsedURL.Scheme {
		case "http", "https":
			transport.Proxy = http.ProxyURL(parsedURL)
		case "socks4", "socks5":
			// create proxy dialer
			dialer, errCreateProxyDialer := createProxyDialer(proxyURL)
			if errCreateProxyDialer != nil {
				log.Debugf("Failed to create proxy dialer: %v", errCreateProxyDialer)
				http.Error(w, "Proxy configuration error", http.StatusInternalServerError)
				return
			}

			// set custom dialer
			transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.Dial(network, addr)
			}
		default:
			log.Debugf("Unsupported proxy scheme: %s", parsedURL.Scheme)
			http.Error(w, "Unsupported proxy scheme", http.StatusInternalServerError)
			return
		}

		// use custom Transport send request
		resp, err := transport.RoundTrip(r)
		if err != nil {
			log.Debugf("Failed to send request through proxy: %v", err)
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		defer func() {
			_ = resp.Body.Close()
		}()

		// copy response header
		for k, vv := range resp.Header {
			for _, v := range vv {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(resp.StatusCode)

		// copy response body
		_, _ = io.Copy(w, resp.Body)
	} else {
		// Use default Transport
		resp, err := http.DefaultTransport.RoundTrip(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		defer func() {
			_ = resp.Body.Close()
		}()

		// copy response header
		for k, vv := range resp.Header {
			for _, v := range vv {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(resp.StatusCode)

		// copy response body
		_, _ = io.Copy(w, resp.Body)
	}
}

// handleHTTPS handle HTTPS request
func (p *Proxy) handleHTTPS(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	if !strings.Contains(host, ":") {
		host = host + ":443"
	}

	// Check if sniffing is needed
	// p.handleSniffHTTPS(w, r, host)
	if p.config.IsSniffDomain(strings.Split(r.Host, ":")[0]) {
		p.handleSniffHTTPS(w, r, host)
	} else {
		p.handleDirectHTTPS(w, r, host)
	}
}

// handleDirectHTTPS handle direct HTTPS request
func (p *Proxy) handleDirectHTTPS(w http.ResponseWriter, _ *http.Request, host string) {
	var targetConn net.Conn
	var err error

	// check proxy config
	proxyURL := p.config.GetProxyServerURL()
	if proxyURL != "" {
		// create proxy dialer
		dialer, errCreateProxyDialer := createProxyDialer(proxyURL)
		if errCreateProxyDialer != nil {
			log.Debugf("Failed to create proxy dialer: %v", errCreateProxyDialer)
			http.Error(w, "Proxy configuration error", http.StatusInternalServerError)
			return
		}

		// use proxy connect to target server
		targetConn, err = dialer.Dial("tcp", host)
		if err != nil {
			log.Debugf("Failed to connect to target server through proxy: %v", err)
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
	} else {
		// direct connect to target server
		targetConn, err = net.Dial("tcp", host)
		if err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
	}
	defer func() {
		_ = targetConn.Close()
	}()

	// Client connection established notification
	w.WriteHeader(http.StatusOK)

	// Get the raw connection
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacker is unsupported", http.StatusInternalServerError)
		return
	}

	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer func() {
		_ = clientConn.Close()
	}()

	var wg sync.WaitGroup
	wg.Add(2)

	// Read Client -> Write Server
	go func() {
		defer wg.Done()
		clientReader := bufio.NewReader(clientConn)
		clientBuf := make([]byte, 4096)

		for {
			n, errRead := clientReader.Read(clientBuf)
			if errRead != nil {
				if errRead != io.EOF {
					if !strings.Contains(errRead.Error(), "use of closed network connection") {
						// log.Errorf("Failed to read client data: %v", errRead)
					}
				}
				break
			}

			// forward to server
			_, err = targetConn.Write(clientBuf[:n])
			if err != nil {
				log.Debugf("Failed to write server data: %v", err)
				break
			}
		}

		_ = targetConn.(*net.TCPConn).CloseWrite()
	}()

	// Read Server -> Write Client
	go func() {
		defer wg.Done()
		serverReader := bufio.NewReader(targetConn)
		serverBuf := make([]byte, 4096)

		for {
			n, errRead := serverReader.Read(serverBuf)
			if errRead != nil {
				if errRead != io.EOF {
					log.Debugf("Failed to read server data: %v", errRead)
				}
				break
			}

			// forward to client
			_, errWrite := clientConn.Write(serverBuf[:n])
			if errWrite != nil {
				log.Debugf("Failed to write client data: %v", errWrite)
				break
			}
		}

		_ = clientConn.(*net.TCPConn).CloseWrite()
	}()

	wg.Wait()
}

// handleSniffHTTPS sniffs HTTPS requests
func (p *Proxy) handleSniffHTTPS(w http.ResponseWriter, r *http.Request, host string) {
	log.Debugf("Sniff HTTPS requests to %s", host)

	// get domain
	domain := strings.Split(r.Host, ":")[0]

	// create cert
	cert, err := utils.GenerateCertificate(domain)
	if err != nil {
		log.Debugf("Failed to generate certificate for %s: %v", domain, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Client connection established notification.
	w.WriteHeader(http.StatusOK)

	// Get the raw connection
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacker is unsupported", http.StatusInternalServerError)
		return
	}

	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		log.Debugf("Failed to Hijack: %v", err)
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer func() {
		_ = clientConn.Close()
	}()

	// connect to target server
	var targetConn *tls.Conn

	// check proxy config
	proxyURL := p.config.GetProxyServerURL()
	if proxyURL != "" {
		// 创建代理拨号器
		dialer, errCreateProxyDialer := createProxyDialer(proxyURL)
		if errCreateProxyDialer != nil {
			log.Debugf("Failed to create proxy dialer: %v", errCreateProxyDialer)
			return
		}

		// create proxy dialer
		conn, errDial := dialer.Dial("tcp", host)
		if errDial != nil {
			log.Debugf("Failed to connect to target server through proxy: %v", errDial)
			return
		}

		// create tls connection
		targetConn = tls.Client(conn, &tls.Config{
			InsecureSkipVerify: false,
			ServerName:         domain,
			MinVersion:         tls.VersionTLS12,
			MaxVersion:         tls.VersionTLS13,
		})
		if errHandshake := targetConn.Handshake(); errHandshake != nil {
			log.Debugf("TLS handshake with target server %s failed: %v", domain, errHandshake)
			_ = conn.Close()
			return
		}
	} else {
		// direct connect to target server
		targetConn, err = tls.Dial("tcp", host, &tls.Config{
			InsecureSkipVerify: true,
		})
		if err != nil {
			log.Debugf("Failed to connect to the target server: %v", err)
			return
		}
	}
	defer func() {
		_ = targetConn.Close()
	}()

	// Establish a TLS connection with the client.
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{*cert},
	}
	tlsConn := tls.Server(clientConn, tlsConfig)
	err = tlsConn.Handshake()
	if err != nil {
		log.Debugf("TLS handshake failed: %v", err)
		return
	}
	defer func() {
		_ = tlsConn.Close()
	}()

	// Variable storing whether to log requests
	var shouldRecord = false
	var requestData bytes.Buffer

	// Forward data in both directions and record it
	var wg sync.WaitGroup
	wg.Add(2)

	disconnect := make(chan bool)
	responseBuffer := make(chan []byte)
	// Read Client -> Write Server
	go func() {
		defer wg.Done()
		clientReader := bufio.NewReader(tlsConn)
		clientBuf := make([]byte, 4096)
		firstLoop := true
		for {
			n, errRead := clientReader.Read(clientBuf)
			if errRead != nil {
				if !strings.Contains(errRead.Error(), "use of closed network connection") {
					// log.Errorf("Failed to read client data: %v", errRead)
				}
				// Notification has been disconnected
				disconnect <- true
				break
			}

			if firstLoop {
				// urlRe := regexp.MustCompile(`(.*?)\s(/.*?)\sHTTP/1\.1`)
				// urlMatches := urlRe.FindAllStringSubmatch(string(clientBuf[:n]), -1)
				// if len(urlMatches) == 1 && urlMatches[0][1] == "GET" {
				// 	log.Info(urlMatches[0])
				// }

				firstLoop = false
			}

			if !shouldRecord && p.shouldRecord(clientBuf[:n]) {
				shouldRecord = true
			}

			if shouldRecord {
				requestData.Write(clientBuf[:n])
				// log.Info(requestData.String())
			}

			_, err = targetConn.Write(clientBuf[:n])
			if err != nil {
				log.Debugf("Failed to write server data: %v", err)
				// Notification has been disconnected
				disconnect <- true
				break
			}
		}

		_ = targetConn.CloseWrite()
	}()

	// Read Server -> Write Client
	go func() {
		defer wg.Done()
		serverReader := bufio.NewReader(targetConn)

		for {
			serverBuf := make([]byte, 4096)
			n, errRead := serverReader.Read(serverBuf)
			if errRead != nil {
				if errRead != io.EOF {
					log.Debugf("Failed to read server data: %v", errRead)
				}
				// Notification has been disconnected
				disconnect <- true
				break
			}

			if shouldRecord {
				data := serverBuf[:n]
				responseBuffer <- data
			}

			_, errWrite := tlsConn.Write(serverBuf[:n])
			if errWrite != nil {
				log.Debugf("Failed to write client data: %v", errWrite)
				// Notification has been disconnected
				disconnect <- true
				break
			}

			// Check for end of response (simple check: HTTP/1.1 chunked terminator)
			if n >= 5 && bytes.Equal(serverBuf[:5], []byte("0\r\n\r\n")) {
				disconnect <- true
				break
			}
		}

		err = tlsConn.CloseWrite()
		if err != nil {
			return
		}
	}()

	// Listen for the disconnect signal and close the connection promptly
	go func() {
		<-disconnect
		_ = tlsConn.Close()
		_ = targetConn.Close()
		_ = clientConn.Close()
	}()

	p.handleResponse(responseBuffer, disconnect, &p.sniffing, p.queue)

	wg.Wait()
}

func (p *Proxy) GetData() (*model.ProxyResponse, error) {
	return p.queue.DequeueBlocking(), nil
}
