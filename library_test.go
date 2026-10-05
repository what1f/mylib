package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testLibrary(t *testing.T, h http.HandlerFunc) *library {
	t.Helper()
	s := httptest.NewServer(h)
	t.Cleanup(s.Close)
	return &library{newClient(), s.URL, s.URL}
}
func TestSearchMetadata(t *testing.T) {
	l := testLibrary(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			return
		}
		if r.Method != "POST" || r.ParseForm() != nil || r.Form.Get("q") != "书 & book" || r.Form.Get("page") != "2" {
			t.Error("incorrect search request")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success":1,"books":[{"id":123,"title":"标题","publisher":null,"author":"Author","year":2020,"language":"Chinese","dl":"/dl/Abc123"}]}`))
	})
	books, err := l.search(context.Background(), "书 & book", false, 2, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(books) != 1 || books[0].ID != 123 || books[0].BookKey != "Abc123" || books[0].Publisher != "" {
		t.Fatalf("unexpected metadata: %+v", books)
	}
	data, _ := json.Marshal(books)
	if !strings.Contains(string(data), `"publisher":""`) {
		t.Fatal("missing required field")
	}
}
func TestFullTextTokenAndCards(t *testing.T) {
	l := testLibrary(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/fulltext" {
			w.Write([]byte(`<input name="token" value="session-token">`))
			return
		}
		if r.URL.Query().Get("token") != "session-token" || r.URL.Query().Get("type") != "words" || r.URL.Query().Get("q") != "phrase & text" {
			t.Error("incorrect full-text request")
		}
		w.Write([]byte(`<div id="searchResultBox"><z-bookcard id="123" download="/dl/key123" publisher="A &amp; B" year="2001" language="English"><div slot="title">A <b>Book</b></div><div slot="author">Some Author</div></z-bookcard></div>`))
	})
	books, err := l.search(context.Background(), "phrase & text", true, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(books) != 1 || books[0].Title != "A Book" || books[0].Publisher != "A & B" {
		t.Fatalf("unexpected full-text result: %+v", books)
	}
}
func TestDownloadAndNoOverwrite(t *testing.T) {
	l := testLibrary(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/epub+zip")
		w.Header().Set("Content-Disposition", `attachment; filename="../../book.epub"`)
		w.Write([]byte("PK-book"))
	})
	dir := t.TempDir()
	path, err := l.download(context.Background(), "abc", dir)
	if err != nil {
		t.Fatal(err)
	}
	if path != filepath.Join(dir, "book.epub") {
		t.Fatal(path)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "PK-book" {
		t.Fatal("wrong file contents")
	}
	_, err = l.download(context.Background(), "abc", dir)
	if err == nil {
		t.Fatal("overwrote existing file")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatal("partial files remain")
	}
}
func TestDownloadRejectsErrorAndPartialFile(t *testing.T) {
	for _, mode := range []string{"html", "json", "short", "empty", "disguised"} {
		t.Run(mode, func(t *testing.T) {
			l := testLibrary(t, func(w http.ResponseWriter, r *http.Request) {
				switch mode {
				case "html":
					w.Header().Set("Content-Type", "text/html")
					w.Write([]byte("<html>Sign in</html>"))
				case "json":
					w.Header().Set("Content-Type", "application/json")
					w.Write([]byte(`{"error":"limit"}`))
				case "short":
					w.Header().Set("Content-Length", "100")
					w.Header().Set("Content-Type", "application/octet-stream")
					w.Write([]byte("short"))
				case "disguised":
					w.Header().Set("Content-Type", "application/octet-stream")
					w.Write([]byte("<!DOCTYPE html><html>Error</html>"))
				case "empty":
					w.Header().Set("Content-Type", "application/octet-stream")
				}
			})
			dir := t.TempDir()
			if _, err := l.download(context.Background(), "abc", dir); err == nil {
				t.Fatal("accepted bad download")
			}
			entries, _ := os.ReadDir(dir)
			if len(entries) != 0 {
				t.Fatal("left a failed download on disk")
			}
		})
	}
}
func TestCommandInterface(t *testing.T) {
	for _, args := range [][]string{{"downlaod", "abc"}, {"download", "--book-key", "abc"}, {"download"}, {"download", "../abc"}, {"search", "--limit", "101", "query"}, {"search", "--output", ".", "query"}} {
		if run(context.Background(), args) == nil {
			t.Fatalf("accepted invalid arguments %v", args)
		}
	}
}
func TestUnknownSearchPageIsError(t *testing.T) {
	if _, err := parseCards([]byte(`<html><h1>Sign in</h1></html>`), 20); err == nil {
		t.Fatal("treated a sign-in page as empty results")
	}
}
