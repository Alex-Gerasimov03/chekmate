package bot

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/proxy"
)

// httpClient собирает клиент для Telegram API.
func httpClient(proxyURL string) (*http.Client, error) {
	transport := &http.Transport{
		DialContext:         (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
		TLSHandshakeTimeout: 10 * time.Second,
		MaxIdleConns:        10,
		IdleConnTimeout:     90 * time.Second,
	}

	if proxyURL = strings.TrimSpace(proxyURL); proxyURL != "" {
		u, err := url.Parse(proxyURL)
		if err != nil {
			return nil, fmt.Errorf("адрес прокси %q: %w", proxyURL, err)
		}

		switch u.Scheme {
		case "http", "https":
			transport.Proxy = http.ProxyURL(u)

		case "socks5", "socks5h":
			// SOCKS5 не умеет ходить через http.Transport.Proxy,
			// поэтому подменяется само установление соединения.
			dialer, err := proxy.FromURL(u, proxy.Direct)
			if err != nil {
				return nil, fmt.Errorf("прокси %q: %w", proxyURL, err)
			}
			contextDialer, ok := dialer.(proxy.ContextDialer)
			if !ok {
				return nil, fmt.Errorf("прокси %q не поддерживает контекст", proxyURL)
			}
			transport.DialContext = contextDialer.DialContext

		default:
			return nil, fmt.Errorf("неизвестная схема прокси %q: нужны http, https или socks5", u.Scheme)
		}
	}

	return &http.Client{Transport: transport, Timeout: 60 * time.Second}, nil
}
