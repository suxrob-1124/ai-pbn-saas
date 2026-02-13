package legacy

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// DecodedLink хранит декодированную внешнюю ссылку сайта.
type DecodedLink struct {
	TargetURL     string
	AnchorText    string
	FoundLine     int
	FoundPath     string
	FoundLocation string
}

// DecodePrimaryHTTPSLinkFromFile извлекает первую внешнюю https-ссылку из body/index.html.
func DecodePrimaryHTTPSLinkFromFile(indexPath string, domainURL string) (*DecodedLink, error) {
	content, err := os.ReadFile(indexPath)
	if err != nil {
		return nil, fmt.Errorf("read index.html: %w", err)
	}
	return DecodePrimaryHTTPSLink(string(content), domainURL)
}

// DecodePrimaryHTTPSLink извлекает первую внешнюю https-ссылку из body HTML.
func DecodePrimaryHTTPSLink(html string, domainURL string) (*DecodedLink, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("parse html: %w", err)
	}
	domainHost := hostOnly(domainURL)
	if domainHost == "" {
		return nil, fmt.Errorf("invalid domain url")
	}

	var found *DecodedLink
	doc.Find("body a[href]").EachWithBreak(func(_ int, sel *goquery.Selection) bool {
		href, ok := sel.Attr("href")
		if !ok {
			return true
		}
		href = strings.TrimSpace(href)
		if !strings.HasPrefix(strings.ToLower(href), "https://") {
			return true
		}
		if isSameDomainURL(href, domainHost) {
			return true
		}
		anchor := normalizeText(sel.Text())
		if anchor == "" {
			anchor = href
		}
		line := findHrefLineInBody(html, href)
		found = &DecodedLink{
			TargetURL:     href,
			AnchorText:    anchor,
			FoundLine:     line,
			FoundPath:     "index.html",
			FoundLocation: fmt.Sprintf("index.html:%d", line),
		}
		return false
	})

	return found, nil
}

func normalizeText(s string) string {
	parts := strings.Fields(strings.TrimSpace(s))
	return strings.Join(parts, " ")
}

func hostOnly(raw string) string {
	v := strings.TrimSpace(raw)
	if v == "" {
		return ""
	}
	if !strings.Contains(v, "://") {
		v = "https://" + v
	}
	u, err := url.Parse(v)
	if err != nil {
		return ""
	}
	h := strings.ToLower(strings.TrimSpace(u.Hostname()))
	h = strings.TrimSuffix(h, ".")
	return h
}

func isSameDomainURL(target, domainHost string) bool {
	u, err := url.Parse(strings.TrimSpace(target))
	if err != nil {
		return false
	}
	h := strings.ToLower(strings.TrimSpace(u.Hostname()))
	h = strings.TrimSuffix(h, ".")
	return h != "" && h == domainHost
}

func findHrefLineInBody(html, href string) int {
	bodyStart := strings.Index(strings.ToLower(html), "<body")
	searchFrom := 0
	if bodyStart >= 0 {
		gt := strings.Index(html[bodyStart:], ">")
		if gt >= 0 {
			searchFrom = bodyStart + gt + 1
		}
	}
	needle1 := `href="` + href + `"`
	needle2 := `href='` + href + `'`
	idx := strings.Index(html[searchFrom:], needle1)
	if idx < 0 {
		idx = strings.Index(html[searchFrom:], needle2)
	}
	if idx < 0 {
		idx = strings.Index(html[searchFrom:], href)
	}
	if idx < 0 {
		if searchFrom > 0 {
			idx = strings.Index(html, href)
			if idx < 0 {
				return 1
			}
		} else {
			return 1
		}
	} else {
		idx += searchFrom
	}
	return 1 + strings.Count(html[:idx], "\n")
}
