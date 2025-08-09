package api

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
)

func NewCDPReverseProxy(browserBaseURL, frontendBaseURL string) (*httputil.ReverseProxy, error) {
	target, err := url.Parse(browserBaseURL)
	if err != nil {
		return nil, err
	}
	internalPort := portOf(target.Host)

	proxy := httputil.NewSingleHostReverseProxy(target)

	proxy.Director = func(r *http.Request) {
		originalHost := r.Host
		r.Header.Set("X-Forwarded-Host", originalHost)

		r.URL.Scheme = target.Scheme
		r.URL.Host = target.Host
		r.Host = target.Host
	}

	proxy.ModifyResponse = func(resp *http.Response) error {
		extHost := resp.Request.Header.Get("X-Forwarded-Host")
		extPort := portOf(extHost)
		if extPort == "" || extPort == internalPort {
			return nil
		}

		if !strings.HasPrefix(resp.Header.Get("Content-Type"), "application/json") {
			return nil
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		_ = resp.Body.Close()

		rewritten := bytes.ReplaceAll(body,
			[]byte(":"+internalPort),
			[]byte(":"+extPort))

		resp.Body = io.NopCloser(bytes.NewReader(rewritten))
		resp.ContentLength = int64(len(rewritten))
		resp.Header.Set("Content-Length", strconv.Itoa(len(rewritten)))
		return nil
	}

	return proxy, nil
}

func portOf(hostPort string) string {
	_, port, err := net.SplitHostPort(hostPort)
	if err != nil {
		return ""
	}
	return port
}
