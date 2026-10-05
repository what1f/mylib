package main

import (
	"bytes"
	"context"
	"crypto/sha1"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const userAgent = "Mozilla/5.0 (compatible; mylib/1.0)"

func newClient() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{Jar: jar, Timeout: 5 * time.Minute, CheckRedirect: func(r *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return fail("response")
		}
		if r.URL.Scheme != "https" && r.URL.Hostname() != "127.0.0.1" {
			return fail("response")
		}
		return nil
	}}
}
func request(ctx context.Context, c *http.Client, method, target, contentType string, body []byte) (*http.Response, error) {
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(body))
		if err != nil {
			return nil, fail("network")
		}
		req.Header.Set("User-Agent", userAgent)
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		resp, err := c.Do(req)
		if err != nil {
			return nil, fail("network")
		}
		if !strings.Contains(resp.Header.Get("Content-Type"), "text/html") {
			if resp.StatusCode >= 300 {
				resp.Body.Close()
				return nil, fail("response")
			}
			return resp, nil
		}
		prefix, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		if err != nil {
			resp.Body.Close()
			return nil, fail("network")
		}
		if !bytes.Contains(prefix, []byte("function get_jhash")) && !bytes.Contains(prefix, []byte("<title>Checking your browser")) {
			resp.Body = &joinedBody{io.MultiReader(bytes.NewReader(prefix), resp.Body), resp.Body}
			return resp, nil
		}
		resp.Body.Close()
		if err := solveCheck(ctx, c, resp.Request.URL, prefix); err != nil {
			return nil, err
		}
	}
	return nil, fail("check")
}

type joinedBody struct {
	io.Reader
	closer io.Closer
}

func (b *joinedBody) Close() error { return b.closer.Close() }

var proofPattern = regexp.MustCompile(`\['([A-F0-9]{40})','c_token=','array'\]`)

// Compute the site's known cookie checks without executing remote JavaScript.
func solveCheck(ctx context.Context, c *http.Client, u *url.URL, body []byte) error {
	if bytes.Contains(body, []byte("function get_jhash")) {
		for _, cookie := range c.Jar.Cookies(u) {
			if cookie.Name != "__js_p_" {
				continue
			}
			parts := strings.Split(cookie.Value, ",")
			code, err := strconv.ParseInt(parts[0], 10, 64)
			if err != nil {
				return fail("check")
			}
			x := int64(123456789)
			k := int64(0)
			for i := int64(0); i < 1677696; i++ {
				if i%50000 == 0 && ctx.Err() != nil {
					return ctx.Err()
				}
				x = ((x + code) ^ (x + x%3 + x%17 + code) ^ i) % 16776960
				if x%117 == 0 {
					k = (k + 1) % 1111
				}
			}
			c.Jar.SetCookies(u, []*http.Cookie{{Name: "__jhash_", Value: strconv.FormatInt(k, 10), Path: "/"}, {Name: "__jua_", Value: strings.ReplaceAll(url.QueryEscape(userAgent), "+", "%20"), Path: "/"}})
			return nil
		}
	}
	if m := proofPattern.FindSubmatch(body); m != nil {
		challenge := string(m[1])
		pos, _ := strconv.ParseInt(challenge[:1], 16, 32)
		start := time.Now()
		for i := 0; i < 20000000; i++ {
			if i%50000 == 0 {
				if err := ctx.Err(); err != nil {
					return err
				}
			}
			token := challenge + strconv.Itoa(i)
			sum := sha1.Sum([]byte(token))
			if sum[pos] == 0xb0 && sum[pos+1] == 0x0b {
				c.Jar.SetCookies(u, []*http.Cookie{{Name: "c_token", Value: token, Path: "/"}, {Name: "c_time", Value: fmt.Sprintf("%.6f", time.Since(start).Seconds()), Path: "/"}})
				return nil
			}
		}
	}
	return fail("check")
}
