package main

import (
	"bufio"
	"context"
	"errors"
	"golang.org/x/net/html"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

func (l *library) download(ctx context.Context, key, dir string) (string, error) {
	if !validKey.MatchString(key) {
		return "", fail("key")
	}
	r, err := l.downloadRequest(ctx, key)
	if err != nil {
		return "", err
	}
	defer r.Body.Close()
	kind := strings.ToLower(r.Header.Get("Content-Type"))
	if r.StatusCode >= 300 || strings.Contains(kind, "text/html") || strings.Contains(kind, "json") {
		return "", downloadResponseError(r.Body)
	}
	reader := bufio.NewReader(r.Body)
	prefix, _ := reader.Peek(512)
	trim := strings.ToLower(strings.TrimSpace(string(prefix)))
	if strings.HasPrefix(trim, "<!doctype html") || strings.HasPrefix(trim, "<html") {
		return "", downloadResponseError(reader)
	}
	_, params, _ := mime.ParseMediaType(r.Header.Get("Content-Disposition"))
	name := params["filename"]
	if name == "" {
		name = r.Request.URL.Query().Get("filename")
	}
	if name == "" {
		name = key
		extensions, _ := mime.ExtensionsByType(strings.Split(kind, ";")[0])
		if len(extensions) > 0 {
			name += extensions[0]
		}
	}
	name = safeFilename(name)
	if name == "" {
		name = key
	}
	if os.MkdirAll(dir, 0755) != nil {
		return "", fail("file")
	}
	target, err := filepath.Abs(filepath.Join(dir, name))
	if err != nil {
		return "", fail("file")
	}
	// Keep partial files hidden; publish only after the stream is complete.
	tmp, err := os.CreateTemp(dir, ".mylib-*")
	if err != nil {
		return "", fail("file")
	}
	defer os.Remove(tmp.Name())
	size, copyErr := io.Copy(tmp, reader)
	syncErr := tmp.Sync()
	closeErr := tmp.Close()
	if copyErr != nil || size == 0 || (r.ContentLength >= 0 && size != r.ContentLength) {
		return "", fail("incomplete")
	}
	if syncErr != nil || closeErr != nil {
		return "", fail("file")
	}
	// A hard link publishes atomically and never overwrites an existing file.
	if err = os.Link(tmp.Name(), target); err != nil {
		if os.IsExist(err) {
			return "", fail("exists")
		}
		return "", fail("file")
	}
	return target, nil
}
func safeFilename(s string) string {
	s = filepath.Base(strings.ReplaceAll(s, "\\", "/"))
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || strings.ContainsRune(`<>:"|?*`, r) {
			return '_'
		}
		return r
	}, s)
	s = strings.Trim(s, " .")
	runes := []rune(s)
	for len([]byte(string(runes))) > 220 {
		runes = runes[:len(runes)-1]
	}
	return string(runes)
}

// Retry only transport failures, before any file is created.
func (l *library) downloadRequest(ctx context.Context, key string) (*http.Response, error) {
	for attempt := 0; ; attempt++ {
		r, err := request(ctx, l.client, "GET", l.fulltext+"/dl/"+key, "", nil)
		var app appError
		if err == nil || attempt == 2 || !errors.As(err, &app) || app.code != "network" {
			return r, err
		}
		timer := time.NewTimer(time.Duration(attempt+1) * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}
func downloadResponseError(body io.Reader) error {
	doc, err := html.Parse(io.LimitReader(body, 256<<10))
	if err != nil {
		return fail("download")
	}
	limited := false
	walk(doc, func(n *html.Node) {
		if n.Type == html.ElementNode && strings.EqualFold(nodeText(n), "Daily limit reached") {
			limited = true
		}
	})
	if limited {
		return fail("downloadLimit")
	}
	return fail("download")
}
