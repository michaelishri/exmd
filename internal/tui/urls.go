package tui

import (
	"regexp"
	"strings"
)

// URL detection is deliberately limited to explicit http(s) schemes: bare
// domains and version numbers in log output are too easy to mistake for
// links. Parentheses are allowed inside a URL so wiki-style links survive;
// trailing punctuation and unmatched closers are trimmed below.
var urlPattern = regexp.MustCompile("(?i)https?://[^\\s<>\"'`\\[\\]{}]+")

// outputURLs extracts URLs from plain block output, trims sentence
// punctuation, and removes duplicates while keeping first-seen order.
func outputURLs(text string) []string {
	var urls []string
	seen := make(map[string]bool)
	for _, match := range urlPattern.FindAllString(text, -1) {
		url := trimURLTail(match)
		if url == "" || seen[url] {
			continue
		}
		seen[url] = true
		urls = append(urls, url)
	}
	return urls
}

// trimURLTail removes punctuation that usually follows a URL in prose. A
// closing bracket is only dropped when it has no matching opener in the URL,
// so "https://x/a_(b)" stays intact while "(https://x)" yields "https://x".
func trimURLTail(url string) string {
	url = strings.TrimRight(url, ".,;:!?")
	for url != "" {
		var open byte
		switch url[len(url)-1] {
		case ')':
			open = '('
		case ']':
			open = '['
		case '}':
			open = '{'
		default:
			return url
		}
		closer := string(url[len(url)-1])
		if strings.Count(url, closer) <= strings.Count(url, string(open)) {
			return url
		}
		url = strings.TrimRight(url[:len(url)-1], ".,;:!?")
	}
	return url
}
