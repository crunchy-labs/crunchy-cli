package utils

import (
	"net/http"
	"net/url"
	"time"
)

type headerRoundTripper struct {
	http.RoundTripper
	header map[string]string
}

func (rht headerRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	resp, err := rht.RoundTripper.RoundTrip(r)
	if err != nil {
		return nil, err
	}
	for k, v := range rht.header {
		resp.Header.Set(k, v)
	}
	return resp, nil
}

func CreateOrDefaultClient(proxy, useragent string) (*http.Client, error) {
	if proxy == "" {
		return http.DefaultClient, nil
	} else {
		proxyURL, err := url.Parse(proxy)
		if err != nil {
			return nil, err
		}

		var rt http.RoundTripper = &http.Transport{
			DisableCompression: true,
			Proxy:              http.ProxyURL(proxyURL),
		}
		if useragent != "" {
			rt = headerRoundTripper{
				RoundTripper: rt,
				header:       map[string]string{"User-Agent": useragent},
			}
		}

		client := &http.Client{
			Transport: rt,
			Timeout:   30 * time.Second,
		}
		return client, nil
	}
}
