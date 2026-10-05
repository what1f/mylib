package main

import (
	"fmt"
	"os"
	"strings"
)

type appError struct{ code string }

func (e appError) Error() string { return message(e.code) }
func fail(code string) error     { return appError{code} }

var catalog = map[string][2]string{
	"usage":      {"mylib search [--full-text] [--page N] [--limit N] \"query\"\nmylib download <book_key> [--output directory]", "mylib search [--full-text] [--page N] [--limit N] \"关键词\"\nmylib download <book_key> [--output 目录]"},
	"arguments":  {"Invalid arguments. Run mylib --help for usage.", "参数无效。运行 mylib --help 查看用法。"},
	"network":    {"Cannot reach the library. Check your connection and try again.", "无法连接书库。请检查网络后重试。"},
	"response":   {"The library could not complete this request. Try again later.", "书库暂时无法完成请求，请稍后重试。"},
	"check":      {"The library's browser check could not be completed. Try again later.", "无法完成书库的浏览器验证，请稍后重试。"},
	"format":     {"The library returned an unreadable response.", "无法读取书库返回的内容。"},
	"download":   {"This book cannot be downloaded now. Open its page to check availability or sign in.", "目前无法下载这本书。请打开书籍页面查看是否可用，或登录后重试。"},
	"key":        {"Use a book_key from the search results.", "请使用搜索结果中的 book_key。"},
	"file":       {"Cannot save the file. Check the output directory and available space.", "无法保存文件，请检查下载目录和剩余空间。"},
	"exists":     {"A file with this name already exists. Choose another output directory.", "同名文件已存在，请选择其他下载目录。"},
	"incomplete": {"The download is incomplete. Try again.", "下载未完成，请重试。"},
	"cancelled":  {"Cancelled.", "已取消。"},
}

func message(code string) string {
	lang := os.Getenv("LANG")
	i := 0
	if strings.HasPrefix(strings.ToLower(lang), "zh") {
		i = 1
	}
	if v, ok := catalog[code]; ok {
		return v[i]
	}
	return catalog["response"][i]
}
func report(err error) { fmt.Fprintln(os.Stderr, err) }
