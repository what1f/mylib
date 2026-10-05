package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx, os.Args[1:]); err != nil {
		if ctx.Err() != nil {
			err = fail("cancelled")
		}
		report(err)
		os.Exit(1)
	}
}
func run(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Println(message("usage"))
		return nil
	}
	command := args[0]
	full := false
	page, limit := 1, 20
	dir := "."
	var positional []string
	for i := 1; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if a == "--help" || a == "-h" {
			fmt.Println(message("usage"))
			return nil
		}
		if a == "--full-text" && command == "search" {
			full = true
			continue
		}
		flag, val, has := strings.Cut(a, "=")
		if flag == "--page" || flag == "--limit" || flag == "--output" {
			if !has {
				i++
				if i >= len(args) {
					return fail("arguments")
				}
				val = args[i]
			}
			if flag == "--output" {
				if command != "download" || val == "" {
					return fail("arguments")
				}
				dir = val
				continue
			}
			if command != "search" {
				return fail("arguments")
			}
			n, err := strconv.Atoi(val)
			if err != nil || n < 1 {
				return fail("arguments")
			}
			if flag == "--page" {
				page = n
			} else {
				limit = n
			}
			continue
		}
		if strings.HasPrefix(a, "-") {
			return fail("arguments")
		}
		positional = append(positional, a)
	}
	if len(positional) != 1 || strings.TrimSpace(positional[0]) == "" || limit > 100 {
		return fail("arguments")
	}
	l := newLibrary()
	switch command {
	case "search":
		books, err := l.search(ctx, positional[0], full, page, limit)
		if err != nil {
			return err
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.SetEscapeHTML(false)
		if enc.Encode(books) != nil {
			return fail("file")
		}
		return nil
	case "download":
		path, err := l.download(ctx, positional[0], dir)
		if err != nil {
			return err
		}
		fmt.Println(path)
		return nil
	default:
		return fail("arguments")
	}
}
