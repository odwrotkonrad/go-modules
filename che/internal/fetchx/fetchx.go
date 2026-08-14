package fetchx

// [>] 🤖🤖

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"time"
)

type Fetcher interface {
	Download(url, dest string) error
	Fetch(url string) ([]byte, error)
}

var Default Fetcher = Real{}

var (
	Retries    = 10
	RetryDelay = 30 * time.Second
)

var client = &http.Client{
	Transport: &http.Transport{
		DialContext:         (&net.Dialer{Timeout: 30 * time.Second}).DialContext,
		TLSHandshakeTimeout: 30 * time.Second,
		Proxy:               http.ProxyFromEnvironment,
	},
}

type Real struct{}

func get(url string) (*http.Response, error) {
	var lastErr error
	for attempt := 0; attempt <= Retries; attempt++ {
		if attempt > 0 {
			time.Sleep(RetryDelay)
		}
		resp, err := client.Get(url)
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode >= 400 {
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("GET %s: %s", url, resp.Status)
			continue
		}
		return resp, nil
	}
	return nil, fmt.Errorf("GET %s failed after %d attempts: %w", url, Retries+1, lastErr)
}

func (Real) Download(url, dest string) error {
	resp, err := get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	f, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		_ = f.Close()
		_ = os.Remove(dest)
		return fmt.Errorf("download %s: %w", url, err)
	}
	return f.Close()
}

func (Real) Fetch(url string) ([]byte, error) {
	resp, err := get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

type swapT interface {
	Helper()
	Cleanup(func())
}

func Swap(t swapT, f Fetcher) {
	t.Helper()
	prev := Default
	Default = f
	t.Cleanup(func() { Default = prev })
}

// [<] 🤖🤖
