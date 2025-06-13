package chrome

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/domstorage"
	"github.com/chromedp/cdproto/fetch"
	"github.com/chromedp/cdproto/io"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
	"github.com/luispater/anyAIProxyAPI/internal/adapter"
	"github.com/luispater/anyAIProxyAPI/internal/utils"
	log "github.com/sirupsen/logrus"
	"net/url"
	"os"
	"sync"
)

type Page struct {
	ctx          context.Context
	cancel       context.CancelFunc
	queue        *utils.Queue[*AIResponse]
	adapterName  string
	RequestMutex sync.Mutex
	URL          string
}

func NewPage(browserCtx context.Context, adapterName string, url string, authFilePath string, sniffURLs ...[]string) (*Page, error) {
	var sniffURL []string
	if len(sniffURLs) > 0 {
		sniffURL = sniffURLs[0]
	} else {
		sniffURL = make([]string, 0)
	}

	if browserCtx == nil {
		return nil, fmt.Errorf("browser context not initialized. Call LaunchBrowserAndContext first")
	}

	var newTargetID target.ID
	var newPageCtx context.Context
	var newPageCancel context.CancelFunc

	err := chromedp.Run(
		browserCtx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			newTargetID, err = target.CreateTarget(url).Do(ctx)
			return err
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create new target (tab): %w", err)
	}

	newPageCtx, newPageCancel = chromedp.NewContext(browserCtx, chromedp.WithTargetID(newTargetID))

	queue := utils.NewQueue[*AIResponse]()
	chromedp.ListenTarget(newPageCtx, func(ifEv interface{}) {
		switch ev := ifEv.(type) {
		case *network.EventRequestWillBeSent:
		case *network.EventResponseReceived:
		case *network.EventDataReceived:
		case *network.EventLoadingFinished:
		case *fetch.EventRequestPaused:
			go func() {
				ctx := chromedp.FromContext(newPageCtx)
				exec := cdp.WithExecutor(newPageCtx, ctx.Target)

				if ev.Request.Method == "POST" {
					if utils.MatchUrl(sniffURL, ev.Request.URL) {
						var buf bytes.Buffer

						handle, errTakeResponseBodyAsStream := fetch.TakeResponseBodyAsStream(ev.RequestID).Do(exec)
						if errTakeResponseBodyAsStream == nil {
							for {
								chunk, eof, errRead := io.Read(handle).WithSize(128).Do(exec)
								if errRead != nil {
									break
								}
								buf.Write([]byte(chunk))
								// fmt.Print(chunk)
								queue.Enqueue(&AIResponse{
									data: buf.Bytes(),
									done: eof,
								})
								if eof {
									break
								}
							}
						}
						// os.WriteFile("1.dump", buf.Bytes(), 0644)

						err = fetch.FulfillRequest(ev.RequestID, ev.ResponseStatusCode).WithResponseHeaders(ev.ResponseHeaders).
							WithBody(base64.StdEncoding.EncodeToString(buf.Bytes())).
							Do(exec)
						if err != nil {
							log.Printf("Failed to FulfillRequest request: %v", err)
						}
						return
					}
				} else if ev.Request.Method == "GET" {
					if ev.Request.URL == url {
						cookies, errGetCookies := GetCookies(newPageCtx)
						if err != nil {
							log.Errorf("get cookies error: %v", errGetCookies)
						} else {
							_, errStat := os.Stat(authFilePath)
							if len(cookies) == 0 && !os.IsNotExist(errStat) {
								log.Debug("Start to loading auth info...")

								err = fetch.FulfillRequest(ev.RequestID, ev.ResponseStatusCode).
									WithBody(base64.StdEncoding.EncodeToString([]byte("prepare to load auth info..."))).
									Do(exec)
								if err != nil {
									log.Printf("Failed to FulfillRequest request: %v", err)
								}

								err = LoadAuthInfo(newPageCtx, url, authFilePath)
								if err != nil {
									log.Debugf("Failed to load auth info: %v", err)
								} else {
									log.Debug("Successfully loaded auth info")
								}
								log.Debug("Stop to loading auth info...")

								err = chromedp.Run(newPageCtx, chromedp.Navigate(url))
								if err != nil {
									log.Errorf("Navigate error: %v", err)
								}
								return
							}
						}
					}
				}

				req := fetch.ContinueRequest(ev.RequestID)
				if err = req.Do(exec); err != nil {
					log.Printf("Failed to continue request: %v", err)
				}
			}()
		}
	})

	err = chromedp.Run(
		newPageCtx,
		fetch.Enable().WithPatterns([]*fetch.RequestPattern{
			{URLPattern: "*", RequestStage: "Response"},
		}),
		chromedp.Navigate(url),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to navigate new page to %s: %w", url, err)
	}

	log.Debugf("New Chromedp page (targetID: %s) created.", newTargetID)

	return &Page{
		ctx:         newPageCtx,
		cancel:      newPageCancel,
		queue:       queue,
		adapterName: adapterName,
		URL:         url,
	}, nil
}

func (p *Page) ResponseData() (*adapter.AdapterResponse, error) {
	data := p.queue.DequeueBlocking()
	if data.done {
		p.queue.Clear()
	}

	if adp, ok := adapter.Adapters[p.adapterName]; ok {
		return adp.HandleResponse(data.data, data.done)
	}
	return nil, fmt.Errorf("adapter %s not found", p.adapterName)
}

func (p *Page) GetContext() context.Context {
	return p.ctx
}

func SetLocalStorages(pageCtx context.Context, origin string, items map[string]string) error {
	parsedUrl, err := url.Parse(origin)
	if err != nil {
		return err
	}
	err = chromedp.Run(pageCtx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			storageID := &domstorage.StorageID{
				SecurityOrigin: fmt.Sprintf("%s://%s", parsedUrl.Scheme, parsedUrl.Host),
				IsLocalStorage: true,
			}
			for key, value := range items {
				err = domstorage.SetDOMStorageItem(storageID, key, value).Do(ctx)
				if err != nil {
					return err
				}
			}
			return nil
		}),
	)
	return err
}

func (p *Page) GetLocalStorages() (map[string]string, error) {
	var items []domstorage.Item
	parsedUrl, err := url.Parse(p.URL)
	if err != nil {
		return nil, err
	}
	err = chromedp.Run(p.GetContext(),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var errGetDOMStorageItems error
			items, errGetDOMStorageItems = domstorage.GetDOMStorageItems(&domstorage.StorageID{
				SecurityOrigin: fmt.Sprintf("%s://%s", parsedUrl.Scheme, parsedUrl.Host),
				IsLocalStorage: true,
			}).Do(ctx)
			return errGetDOMStorageItems
		}),
	)
	if err != nil {
		return nil, err
	}
	localStorage := make(map[string]string)
	for _, item := range items {
		localStorage[item[0]] = item[1]
	}
	return localStorage, nil
}

func (p *Page) Close() {
	p.cancel()
}

func LoadAuthInfo(pageCtx context.Context, url string, authFilePath string) error {
	if _, errStat := os.Stat(authFilePath); !os.IsNotExist(errStat) {
		authData, errReadFile := os.ReadFile(authFilePath)
		if errReadFile != nil {
			log.Debugf("Error reading auth info to file %s: %v", authFilePath, errReadFile)
			return errReadFile
		}
		var auth AuthInfo
		err := json.Unmarshal(authData, &auth)
		if err != nil {
			log.Debugf("Error unmarshalling auto info from JSON: %v", err)
			return err
		}

		err = SetCookies(pageCtx, auth.Cookies)
		if err != nil {
			log.Debugf("Error writing cookies: %v", err)
			return err
		}
		err = SetLocalStorages(pageCtx, url, auth.LocalStorage)
		if err != nil {
			log.Debugf("Error writing local storage: %v", err)
			return err
		}

		log.Debugf("Successfully loaded auth info from file %s", authFilePath)
	}
	return nil
}
