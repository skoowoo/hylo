package urlembed

import (
	"io"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

// parseOG reads r only as far as the end of <head>.
func parseOG(r io.Reader, base *url.URL) (title, desc, site, image string) {
	var docTitle, ogTitle, twTitle string
	var ogDesc, twDesc, metaDesc string
	var ogImage, twImage string
	z := html.NewTokenizer(r)
	var inTitle bool
	var titleBuf strings.Builder
	for {
		switch z.Next() {
		case html.ErrorToken:
			return pickOG(base, docTitle, ogTitle, twTitle, ogDesc, twDesc, metaDesc, site, ogImage, twImage)
		case html.StartTagToken, html.SelfClosingTagToken:
			name, _ := z.TagName()
			switch string(name) {
			case "meta":
				prop, content := metaAttr(z)
				if content == "" {
					continue
				}
				switch prop {
				case "og:title":
					if ogTitle == "" {
						ogTitle = content
					}
				case "twitter:title":
					if twTitle == "" {
						twTitle = content
					}
				case "og:description":
					if ogDesc == "" {
						ogDesc = content
					}
				case "twitter:description":
					if twDesc == "" {
						twDesc = content
					}
				case "description":
					if metaDesc == "" {
						metaDesc = content
					}
				case "og:site_name":
					if site == "" {
						site = content
					}
				case "og:image", "og:image:url", "og:image:secure_url":
					if ogImage == "" {
						ogImage = content
					}
				case "twitter:image", "twitter:image:src":
					if twImage == "" {
						twImage = content
					}
				}
			case "title":
				inTitle = true
				titleBuf.Reset()
			case "body":
				return pickOG(base, docTitleText(docTitle, &titleBuf), ogTitle, twTitle, ogDesc, twDesc, metaDesc, site, ogImage, twImage)
			}
		case html.EndTagToken:
			name, _ := z.TagName()
			switch string(name) {
			case "title":
				inTitle = false
				if docTitle == "" {
					docTitle = titleBuf.String()
				}
			case "head":
				return pickOG(base, docTitleText(docTitle, &titleBuf), ogTitle, twTitle, ogDesc, twDesc, metaDesc, site, ogImage, twImage)
			}
		case html.TextToken:
			if inTitle {
				titleBuf.Write(z.Text())
			}
		}
	}
}

func docTitleText(closed string, buf *strings.Builder) string {
	if strings.TrimSpace(closed) != "" {
		return closed
	}
	return buf.String()
}

func metaAttr(z *html.Tokenizer) (prop, content string) {
	for {
		k, v, more := z.TagAttr()
		key := strings.ToLower(string(k))
		val := strings.TrimSpace(string(v))
		switch key {
		case "property", "name":
			if prop == "" {
				prop = strings.ToLower(val)
			}
		case "content", "value":
			if content == "" {
				content = val
			}
		}
		if !more {
			break
		}
	}
	return prop, content
}

func pickOG(base *url.URL, docTitle, ogTitle, twTitle, ogDesc, twDesc, metaDesc, site, ogImage, twImage string) (string, string, string, string) {
	return cleanText(first(ogTitle, twTitle, docTitle), 180),
		cleanText(first(ogDesc, twDesc, metaDesc), 240),
		cleanText(site, 80),
		resolveRef(base, first(ogImage, twImage))
}

func first(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func cleanText(s string, max int) string {
	s = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, s)
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if len(r) > max {
		return string(r[:max])
	}
	return s
}

func resolveRef(base *url.URL, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || base == nil {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	abs := base.ResolveReference(u)
	if abs.Scheme != "http" && abs.Scheme != "https" {
		return ""
	}
	if len(abs.String()) > 2000 {
		return ""
	}
	return abs.String()
}

func looksLikeHTML(body []byte) bool {
	s := strings.ToLower(string(body[:min(len(body), 4096)]))
	s = strings.TrimSpace(s)
	return strings.HasPrefix(s, "<!doctype html") || strings.Contains(s, "<html") || strings.Contains(s, "<head") || strings.Contains(s, "<meta")
}
