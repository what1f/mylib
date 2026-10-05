package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCookieCheckReplaysPost(t *testing.T) {
	attempts := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		body, _ := io.ReadAll(r.Body)
		if r.Method != "POST" || string(body) != "q=hello" {
			t.Error("POST lost during retry")
		}
		if _, err := r.Cookie("__jhash_"); err != nil {
			http.SetCookie(w, &http.Cookie{Name: "__js_p_", Value: "6,1800,0,0,0", Path: "/"})
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<script>function get_jhash(){}</script>`))
			return
		}
		cookie, _ := r.Cookie("__jua_")
		if cookie == nil || strings.Contains(cookie.Value, "+") {
			t.Error("user agent cookie is incorrectly encoded")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success":1}`))
	}))
	defer s.Close()
	r, err := request(context.Background(), newClient(), "POST", s.URL, "application/x-www-form-urlencoded", []byte("q=hello"))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	if attempts != 2 {
		t.Fatal(attempts)
	}
}
