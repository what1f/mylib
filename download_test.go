package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

type downloadTransport func(*http.Request) (*http.Response, error)

func (f downloadTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestDownloadRetriesNetworkFailure(t *testing.T) {
	for _, recover := range []bool{true, false} {
		calls := 0
		l := newLibrary()
		l.client.Transport = downloadTransport(func(r *http.Request) (*http.Response, error) {
			calls++
			if !recover || calls == 1 {
				return nil, errors.New("connection lost")
			}
			return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"application/pdf"}}, Body: io.NopCloser(strings.NewReader("%PDF-test")), ContentLength: 9, Request: r}, nil
		})
		_, err := l.download(context.Background(), "abc", t.TempDir())
		if recover && (err != nil || calls != 2) {
			t.Fatalf("recovery: calls=%d err=%v", calls, err)
		}
		if !recover && (err == nil || calls != 3) {
			t.Fatalf("limit: calls=%d err=%v", calls, err)
		}
	}
}
func TestDownloadLimitDoesNotRetry(t *testing.T) {
	calls := 0
	l := testLibrary(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><h1>Daily <span>limit</span> reached</h1></html>`))
	})
	dir := t.TempDir()
	_, err := l.download(context.Background(), "abc", dir)
	var app appError
	if !errors.As(err, &app) || app.code != "downloadLimit" || calls != 1 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatal("saved an error page")
	}
	for _, lang := range []string{"en_US.UTF-8", "zh_TW.UTF-8"} {
		t.Setenv("LANG", lang)
		want := "daily download limit"
		if strings.HasPrefix(lang, "zh") {
			want = "每日下载额度"
		}
		if !strings.Contains(err.Error(), want) {
			t.Fatal(err)
		}
	}
}
