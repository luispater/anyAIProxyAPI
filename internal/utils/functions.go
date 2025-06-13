package utils

import (
	"net/url"
	"path"
)

func InArray(needle interface{}, hystack interface{}) bool {
	switch key := needle.(type) {
	case string:
		for _, item := range hystack.([]string) {
			if key == item {
				return true
			}
		}
	case int:
		for _, item := range hystack.([]int) {
			if key == item {
				return true
			}
		}
	case int64:
		for _, item := range hystack.([]int64) {
			if key == item {
				return true
			}
		}
	default:
		return false
	}
	return false
}

func MatchUrl(patternUrls []string, targetUrl string) bool {
	for _, patternUrl := range patternUrls {
		if patternUrl == targetUrl {
			return true
		}
		parsedPatternUrl, err := url.Parse(patternUrl)
		if err != nil {
			return false
		}
		patternUrlPath := parsedPatternUrl.Path

		parsedTargetUrl, err := url.Parse(targetUrl)
		if err != nil {
			return false
		}
		targetUrlPath := parsedTargetUrl.Path

		matched, err := path.Match(patternUrlPath, targetUrlPath)
		if err != nil {
			return false
		}

		if matched {
			if parsedPatternUrl.Scheme == parsedTargetUrl.Scheme && parsedPatternUrl.Host == parsedTargetUrl.Host {
				return true
			}
		}
	}
	return false
}
