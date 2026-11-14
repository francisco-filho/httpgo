// Package transport implements HTTP execution with bounded, same-origin redirects.
package transport

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"capimgrosso.com/httpgo/internal/runner"
)

const requestTimeout = 10 * time.Second

type ErrorKind string

const (
	Network  ErrorKind = "falha de conexão ou protocolo HTTP"
	Timeout  ErrorKind = "timeout da requisição"
	Canceled ErrorKind = "requisição cancelada"
	TLS      ErrorKind = "falha na verificação TLS"
	Redirect ErrorKind = "redirecionamento bloqueado ou inválido"
	Body     ErrorKind = "falha ao consumir ou fechar a resposta"
)

// Error preserves the original cause for inspection but never prints it.
type Error struct {
	Kind  ErrorKind
	cause error
}

func (e *Error) Error() string { return string(e.Kind) }
func (e *Error) Unwrap() error { return e.cause }

type NetHTTPExecutor struct {
	transport *http.Transport
	timeout   time.Duration
}

func NewNetHTTPExecutor() *NetHTTPExecutor {
	return &NetHTTPExecutor{
		timeout: requestTimeout,
		transport: &http.Transport{
			DialContext:         (&net.Dialer{Timeout: requestTimeout}).DialContext,
			TLSHandshakeTimeout: requestTimeout,
			// Go retries idempotent requests on reused connections. Never reuse
			// connections, and disable HTTP/2 (which has its own replay paths).
			DisableKeepAlives: true,
			ForceAttemptHTTP2: false,
			TLSNextProto:      make(map[string]func(string, *tls.Conn) http.RoundTripper),
		},
	}
}

func (e *NetHTTPExecutor) Execute(parent context.Context, request runner.Request) (runner.Response, error) {
	ctx, cancel := context.WithTimeout(parent, e.timeout)
	defer cancel()
	current, err := url.Parse(request.URL)
	if err != nil {
		return runner.Response{}, &Error{Kind: Network, cause: err}
	}
	current.Scheme = strings.ToLower(current.Scheme)
	origin := current
	method, body := request.Method, request.Body
	if method == "" {
		method = http.MethodGet
	}
	for hops := 0; ; hops++ {
		req, err := http.NewRequestWithContext(ctx, method, current.String(), bytes.NewReader(body))
		if err != nil {
			return runner.Response{}, classify(ctx, Network, err)
		}
		for _, header := range request.Headers {
			if strings.EqualFold(header.Name, "Host") {
				req.Host = header.Value
			} else {
				req.Header.Add(header.Name, header.Value)
			}
		}
		response, err := e.transport.RoundTrip(req)
		if err != nil {
			return runner.Response{}, classify(ctx, Network, err)
		}
		_, readErr := io.Copy(io.Discard, response.Body)
		closeErr := response.Body.Close()
		if readErr != nil {
			return runner.Response{}, classify(ctx, Body, readErr)
		}
		if closeErr != nil {
			return runner.Response{}, classify(ctx, Body, closeErr)
		}
		if ctx.Err() != nil {
			return runner.Response{}, classify(ctx, Network, ctx.Err())
		}
		if !isRedirect(response.StatusCode) || response.Header.Get("Location") == "" {
			return runner.Response{Status: response.StatusCode}, nil
		}
		next, err := response.Location()
		if err != nil {
			return runner.Response{}, &Error{Kind: Redirect, cause: err}
		}
		next.Scheme = strings.ToLower(next.Scheme)
		if hops >= 10 || !sameOrigin(origin, next) {
			return runner.Response{}, &Error{Kind: Redirect}
		}
		if response.StatusCode == 301 || response.StatusCode == 302 || response.StatusCode == 303 {
			if method != http.MethodGet && method != http.MethodHead {
				method, body = http.MethodGet, nil
			}
		}
		current = next
	}
}

func isRedirect(status int) bool {
	switch status {
	case 301, 302, 303, 307, 308:
		return true
	default:
		return false
	}
}

func sameOrigin(a, b *url.URL) bool {
	return strings.EqualFold(a.Scheme, b.Scheme) && strings.EqualFold(a.Hostname(), b.Hostname()) && effectivePort(a) == effectivePort(b)
}

func effectivePort(u *url.URL) string {
	if port := u.Port(); port != "" {
		if number, err := strconv.ParseUint(port, 10, 16); err == nil {
			return strconv.FormatUint(number, 10)
		}
		return port
	}
	if strings.EqualFold(u.Scheme, "https") {
		return "443"
	}
	return "80"
}

func classify(ctx context.Context, fallback ErrorKind, cause error) error {
	if errors.Is(ctx.Err(), context.Canceled) || errors.Is(cause, context.Canceled) {
		return &Error{Kind: Canceled, cause: context.Canceled}
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(cause, context.DeadlineExceeded) {
		return &Error{Kind: Timeout, cause: context.DeadlineExceeded}
	}
	var authority x509.UnknownAuthorityError
	var hostname x509.HostnameError
	var certificate x509.CertificateInvalidError
	if errors.As(cause, &authority) || errors.As(cause, &hostname) || errors.As(cause, &certificate) {
		return &Error{Kind: TLS, cause: cause}
	}
	return &Error{Kind: fallback, cause: cause}
}
