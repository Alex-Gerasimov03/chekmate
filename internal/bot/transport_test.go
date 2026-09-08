package bot

import (
	"net/http"
	"testing"
)

func TestHTTPClientDirect(t *testing.T) {
	c, err := httpClient("")
	if err != nil {
		t.Fatal(err)
	}
	if c.Transport.(*http.Transport).Proxy != nil {
		t.Error("без адреса прокси соединение должно быть прямым")
	}
}

func TestHTTPClientHTTPProxy(t *testing.T) {
	c, err := httpClient("http://user:pass@proxy.example:3128")
	if err != nil {
		t.Fatal(err)
	}

	tr := c.Transport.(*http.Transport)
	if tr.Proxy == nil {
		t.Fatal("прокси не настроен")
	}

	req, _ := http.NewRequest(http.MethodGet, "https://api.telegram.org/", nil)
	u, err := tr.Proxy(req)
	if err != nil {
		t.Fatal(err)
	}
	if u.Host != "proxy.example:3128" {
		t.Errorf("адрес прокси %q", u.Host)
	}
}

// SOCKS5 подменяет установление соединения, а не поле Proxy.
func TestHTTPClientSocks5(t *testing.T) {
	c, err := httpClient("socks5://proxy.example:1080")
	if err != nil {
		t.Fatal(err)
	}
	if c.Transport.(*http.Transport).Proxy != nil {
		t.Error("для socks5 поле Proxy не используется")
	}
}

func TestHTTPClientRejectsBadProxy(t *testing.T) {
	for _, addr := range []string{"ftp://proxy.example:21", "://broken"} {
		if _, err := httpClient(addr); err == nil {
			t.Errorf("адрес %q должен отвергаться", addr)
		}
	}
}
