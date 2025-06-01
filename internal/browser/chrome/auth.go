package chrome

import "github.com/chromedp/cdproto/network"

type AuthInfo struct {
	Cookies      []*network.CookieParam `json:"cookies"`
	LocalStorage map[string]string      `json:"local_storage"`
}
