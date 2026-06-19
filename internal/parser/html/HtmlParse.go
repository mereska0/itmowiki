package parser

import "strings"

func startsWith(html []byte, pos int, s string) bool {
	if pos+len(s) > len(html) {
		return false
	}
	for i := 0; i < len(s); i++ {
		if html[pos+i] != s[i] {
			return false
		}
	}
	return true
}

func parseHref(html []byte, pos int) string {
	var link strings.Builder
	for i := pos + 2; i < len(html)-3; i++ {
		if html[i] == 'h' &&
			html[i+1] == 'r' &&
			html[i+2] == 'e' &&
			html[i+3] == 'f' {
			i += 4
			for i < len(html) && html[i] != '=' {
				i++
			}
			if i >= len(html) {
				return ""
			}
			for i < len(html) && html[i] != '"' {
				i++
			}
			if i >= len(html) {
				return ""
			}
			i++
			for i < len(html) && html[i] != '"' {
				link.WriteByte(html[i])
				i++
			}
			return link.String()
		}
	}
	return ""
}

func parseTagText(html []byte, pos int) string {
	i := pos
	for i < len(html) && html[i] != '>' {
		i++
	}
	if i >= len(html) {
		return ""
	}
	i++
	var text strings.Builder
	for i < len(html) && html[i] != '<' {
		text.WriteByte(html[i])
		i++
	}
	return strings.TrimSpace(text.String())
}

func normalizeWord(word string) string {
	word = strings.ToLower(word)
	word = strings.Trim(word, ".,!?;:()[]{}\"'«»—–-")
	return word
}

func Parse(html []byte) *HtmlPage {
	page := NewHtmlPage(html)
	for i := 0; i < len(html); i++ {
		if startsWith(html, i, "<a") {
			href := parseHref(html, i)
			if href != "" {
				page.AppendLink(href)
			}
		}
		if startsWith(html, i, "<h1") {
			text := parseTagText(html, i)

			for _, keyword := range strings.Fields(text) {
				keyword = normalizeWord(keyword)

				if keyword != "" {
					page.AppendKeywords(keyword)
				}
			}
		}
		if startsWith(html, i, "<title") {
			title := parseTagText(html, i)
			if title != "" {
				page.SetTitle(title)
			}
		}
	}
	return page
}
