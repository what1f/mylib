package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

type Book struct {
	ID        int64  `json:"id"`
	BookKey   string `json:"book_key"`
	Title     string `json:"title"`
	Publisher string `json:"publisher"`
	Author    string `json:"author"`
	Year      int    `json:"year"`
	Language  string `json:"language"`
}
type library struct {
	client         *http.Client
	base, fulltext string
}

func newLibrary() *library {
	return &library{newClient(), "https://z-library.bz", "https://z-library.biz"}
}
func (l *library) read(ctx context.Context, method, target string, body []byte) ([]byte, error) {
	r, err := request(ctx, l.client, method, target, "application/x-www-form-urlencoded", body)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	if r.StatusCode >= 300 {
		return nil, fail("response")
	}
	b, err := io.ReadAll(io.LimitReader(r.Body, 16<<20))
	if err != nil {
		return nil, fail("network")
	}
	return b, nil
}
func (l *library) search(ctx context.Context, q string, full bool, page, limit int) ([]Book, error) {
	if full {
		return l.fullTextSearch(ctx, q, page, limit)
	}
	// Establish the site's cookies before POSTing. The browser does the same.
	if _, err := l.read(ctx, "GET", l.base+"/", nil); err != nil {
		return nil, err
	}
	data := url.Values{"q": {q}, "page": {strconv.Itoa(page)}, "limit": {strconv.Itoa(limit)}, "order": {"popular"}}
	b, err := l.read(ctx, "POST", l.base+"/api/search", []byte(data.Encode()))
	if err != nil {
		return nil, err
	}
	var result struct {
		Success int `json:"success"`
		Books   []struct {
			ID        int64  `json:"id"`
			Title     string `json:"title"`
			Publisher string `json:"publisher"`
			Author    string `json:"author"`
			Language  string `json:"language"`
			DL        string `json:"dl"`
			Year      int    `json:"year"`
		} `json:"books"`
	}
	if json.Unmarshal(b, &result) != nil {
		return nil, fail("format")
	}
	if result.Success != 1 {
		return nil, fail("response")
	}
	out := make([]Book, 0, len(result.Books))
	for _, v := range result.Books {
		key := strings.TrimPrefix(v.DL, "/dl/")
		if !validKey.MatchString(key) {
			key = ""
		}
		out = append(out, Book{v.ID, key, v.Title, v.Publisher, v.Author, v.Year, v.Language})
	}
	return out, nil
}
func (l *library) fullTextSearch(ctx context.Context, q string, page, limit int) ([]Book, error) {
	b, err := l.read(ctx, "GET", l.fulltext+"/fulltext", nil)
	if err != nil {
		return nil, err
	}
	doc, err := html.Parse(bytes.NewReader(b))
	if err != nil {
		return nil, fail("format")
	}
	var token string
	walk(doc, func(n *html.Node) {
		if n.Data == "input" && attr(n, "name") == "token" {
			token = attr(n, "value")
		}
	})
	if token == "" {
		return nil, fail("format")
	}
	params := url.Values{"q": {q}, "token": {token}, "type": {"words"}, "page": {strconv.Itoa(page)}}
	b, err = l.read(ctx, "GET", l.fulltext+"/fulltext/?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	return parseCards(b, limit)
}
func parseCards(b []byte, limit int) ([]Book, error) {
	doc, err := html.Parse(bytes.NewReader(b))
	if err != nil {
		return nil, fail("format")
	}
	out := []Book{}
	hasResults := false
	walk(doc, func(n *html.Node) {
		if attr(n, "id") == "searchResultBox" {
			hasResults = true
		}
		if n.Data != "z-bookcard" || len(out) >= limit {
			return
		}
		id, _ := strconv.ParseInt(attr(n, "id"), 10, 64)
		if id == 0 {
			return
		}
		year, _ := strconv.Atoi(attr(n, "year"))
		v := Book{ID: id, BookKey: strings.TrimPrefix(attr(n, "download"), "/dl/"), Publisher: attr(n, "publisher"), Language: attr(n, "language"), Year: year}
		walk(n, func(child *html.Node) {
			switch attr(child, "slot") {
			case "title":
				v.Title = nodeText(child)
			case "author":
				v.Author = nodeText(child)
			}
		})
		if !validKey.MatchString(v.BookKey) {
			v.BookKey = ""
		}
		out = append(out, v)
	})
	if !hasResults && len(out) == 0 && !bytes.Contains(b, []byte("Nothing has been found")) {
		return nil, fail("format")
	}
	return out, nil
}

var validKey = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}
func walk(n *html.Node, fn func(*html.Node)) {
	fn(n)
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walk(c, fn)
	}
}
func nodeText(n *html.Node) string {
	var s strings.Builder
	walk(n, func(c *html.Node) {
		if c.Type == html.TextNode {
			s.WriteString(c.Data)
		}
	})
	return strings.Join(strings.Fields(s.String()), " ")
}
