package main

import (
	"bufio"
	"context"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

func (l *library) download(ctx context.Context, key, dir string) (string, error) {
	if !validKey.MatchString(key) {
		return "", fail("key")
	}
	r, err := request(ctx, l.client, "GET", l.fulltext+"/dl/"+key, "", nil)
	if err != nil {
		return "", err
	}
	defer r.Body.Close()
	kind := strings.ToLower(r.Header.Get("Content-Type"))
	if r.StatusCode >= 300 || strings.Contains(kind, "text/html") || strings.Contains(kind, "json") {
		return "", fail("download")
	}
	reader := bufio.NewReader(r.Body)
	prefix, _ := reader.Peek(512)
	trim := strings.ToLower(strings.TrimSpace(string(prefix)))
	if strings.HasPrefix(trim, "<!doctype html") || strings.HasPrefix(trim, "<html") {
		return "", fail("download")
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
