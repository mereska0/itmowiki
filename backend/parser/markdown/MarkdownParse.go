package markdown

import (
	"fmt"
	stdhtml "html"
	"strings"
	"unicode"
)

type node struct {
	tag      string
	attrs    map[string]string
	text     string
	children []*node
}

/*
pre: html byte code

post: string of markdown
*/
func Parse(rawHTML []byte) string {
	return convert(string(rawHTML))
}

func convert(rawHTML string) string {
	root := parse(rawHTML)
	if content := findByClass(root, "mw-parser-output"); content != nil {
		return cleanup(renderChildren(content.children))
	}
	if content := findByID(root, "mw-content-text"); content != nil {
		return cleanup(renderChildren(content.children))
	}
	if body := findFirst(root, "body"); body != nil {
		return cleanup(renderChildren(body.children))
	}

	return cleanup(renderChildren(root.children))
}

func parse(rawHTML string) *node {
	root := &node{}
	stack := []*node{root}

	for pos := 0; pos < len(rawHTML); {
		if rawHTML[pos] != '<' {
			next := strings.IndexByte(rawHTML[pos:], '<')
			if next == -1 {
				next = len(rawHTML) - pos
			}
			appendText(stack[len(stack)-1], rawHTML[pos:pos+next])
			pos += next
			continue
		}

		end := strings.IndexByte(rawHTML[pos:], '>')
		if end == -1 {
			appendText(stack[len(stack)-1], rawHTML[pos:])
			break
		}

		tagContent := strings.TrimSpace(rawHTML[pos+1 : pos+end])
		pos += end + 1

		if tagContent == "" || strings.HasPrefix(tagContent, "!") || strings.HasPrefix(tagContent, "?") {
			continue
		}
		if strings.HasPrefix(tagContent, "/") {
			closeFields := strings.Fields(strings.TrimSpace(tagContent[1:]))
			if len(closeFields) == 0 {
				continue
			}
			closeTag := strings.ToLower(closeFields[0])
			for len(stack) > 1 {
				last := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				if last.tag == closeTag {
					break
				}
			}
			continue
		}

		tag, attrs, selfClosing := parseOpenTag(tagContent)
		if tag == "" {
			continue
		}

		child := &node{tag: tag, attrs: attrs}
		parent := stack[len(stack)-1]
		parent.children = append(parent.children, child)
		if !selfClosing && !isVoidTag(tag) {
			stack = append(stack, child)
		}
	}

	return root
}

func appendText(parent *node, text string) {
	if text == "" {
		return
	}
	parent.children = append(parent.children, &node{text: text})
}

func parseOpenTag(content string) (string, map[string]string, bool) {
	selfClosing := strings.HasSuffix(content, "/")
	content = strings.TrimSpace(strings.TrimSuffix(content, "/"))
	if content == "" {
		return "", nil, selfClosing
	}

	nameEnd := 0
	for nameEnd < len(content) && !unicode.IsSpace(rune(content[nameEnd])) {
		nameEnd++
	}

	tag := strings.ToLower(content[:nameEnd])
	attrs := parseAttrs(content[nameEnd:])
	return tag, attrs, selfClosing
}

func parseAttrs(raw string) map[string]string {
	attrs := make(map[string]string)

	for i := 0; i < len(raw); {
		for i < len(raw) && unicode.IsSpace(rune(raw[i])) {
			i++
		}
		if i >= len(raw) {
			break
		}

		keyStart := i
		for i < len(raw) && raw[i] != '=' && !unicode.IsSpace(rune(raw[i])) {
			i++
		}
		key := strings.ToLower(strings.TrimSpace(raw[keyStart:i]))
		for i < len(raw) && unicode.IsSpace(rune(raw[i])) {
			i++
		}

		value := ""
		if i < len(raw) && raw[i] == '=' {
			i++
			for i < len(raw) && unicode.IsSpace(rune(raw[i])) {
				i++
			}
			if i < len(raw) && (raw[i] == '"' || raw[i] == '\'') {
				quote := raw[i]
				i++
				valueStart := i
				for i < len(raw) && raw[i] != quote {
					i++
				}
				value = raw[valueStart:i]
				if i < len(raw) {
					i++
				}
			} else {
				valueStart := i
				for i < len(raw) && !unicode.IsSpace(rune(raw[i])) {
					i++
				}
				value = raw[valueStart:i]
			}
		}

		if key != "" {
			attrs[key] = stdhtml.UnescapeString(value)
		}
	}

	return attrs
}

func findFirst(n *node, tag string) *node {
	if n.tag == tag {
		return n
	}
	for _, child := range n.children {
		if found := findFirst(child, tag); found != nil {
			return found
		}
	}

	return nil
}

func findByID(n *node, id string) *node {
	if n.attrs["id"] == id {
		return n
	}
	for _, child := range n.children {
		if found := findByID(child, id); found != nil {
			return found
		}
	}

	return nil
}

func findByClass(n *node, class string) *node {
	if hasClass(n, class) {
		return n
	}
	for _, child := range n.children {
		if found := findByClass(child, class); found != nil {
			return found
		}
	}

	return nil
}

func renderChildren(children []*node) string {
	var b strings.Builder
	for _, child := range children {
		b.WriteString(render(child))
	}

	return b.String()
}

func render(n *node) string {
	if n.tag == "" {
		return normalizeTextNode(stdhtml.UnescapeString(n.text))
	}
	if shouldSkip(n) {
		return ""
	}

	switch n.tag {
	case "script", "style", "head", "noscript":
		return ""
	case "body", "html", "main", "article", "section", "div", "span":
		return renderChildren(n.children)
	case "h1", "h2", "h3", "h4", "h5", "h6":
		level := int(n.tag[1] - '0')
		return "\n\n" + strings.Repeat("#", level) + " " + inline(n) + "\n\n"
	case "p":
		text := inline(n)
		if text == "" {
			return ""
		}
		return "\n\n" + text + "\n\n"
	case "br":
		return "  \n"
	case "hr":
		return "\n\n---\n\n"
	case "a":
		text := inline(n)
		href := strings.TrimSpace(n.attrs["href"])
		if text != "" {
			return text
		}
		if href == "" || isServiceLink(href) {
			return ""
		}
		return shortHref(href)
	case "strong", "b":
		return wrapInline("**", inline(n))
	case "em", "i":
		return wrapInline("*", inline(n))
	case "code":
		return wrapInline("`", inline(n))
	case "pre":
		text := strings.Trim(stdhtml.UnescapeString(textContent(n)), "\n")
		if text == "" {
			return ""
		}
		return "\n\n```\n" + text + "\n```\n\n"
	case "ul":
		return "\n" + renderList(n, false) + "\n"
	case "ol":
		return "\n" + renderList(n, true) + "\n"
	case "li":
		text := listItemText(n)
		if text == "" {
			return ""
		}
		return "- " + text + "\n"
	case "blockquote":
		return "\n\n" + renderBlockquote(cleanup(renderChildren(n.children))) + "\n\n"
	case "img":
		src := strings.TrimSpace(n.attrs["src"])
		alt := strings.TrimSpace(n.attrs["alt"])
		if looksLikeTex(alt) {
			return normalizeMathText(alt)
		}
		if src == "" {
			return ""
		}
		return "![" + escapeLinkText(alt) + "](" + src + ")"
	case "table":
		return renderTable(n)
	default:
		return renderChildren(n.children)
	}
}

func inline(n *node) string {
	return strings.TrimSpace(renderChildren(n.children))
}

func wrapInline(mark string, text string) string {
	if text == "" {
		return ""
	}

	return mark + text + mark
}

func renderList(n *node, ordered bool) string {
	var b strings.Builder
	for _, child := range n.children {
		if child.tag != "li" {
			continue
		}
		text := listItemText(child)
		if text == "" {
			continue
		}
		if ordered {
			b.WriteString("1. ")
		} else {
			b.WriteString("- ")
		}
		b.WriteString(text)
		b.WriteByte('\n')
	}

	return b.String()
}

func listItemText(n *node) string {
	return strings.TrimSpace(strings.ReplaceAll(cleanup(renderChildren(n.children)), "\n", " "))
}

func renderBlockquote(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = "> " + line
	}

	return strings.Join(lines, "\n")
}

func renderTable(n *node) string {
	rows := tableRows(n)
	if len(rows) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n\n")
	for i, row := range rows {
		b.WriteString("| ")
		b.WriteString(strings.Join(row, " | "))
		b.WriteString(" |\n")
		if i == 0 {
			b.WriteString("| ")
			for j := range row {
				if j > 0 {
					b.WriteString(" | ")
				}
				b.WriteString("---")
			}
			b.WriteString(" |\n")
		}
	}
	b.WriteString("\n")

	return b.String()
}

func tableRows(n *node) [][]string {
	var rows [][]string
	var walkRows func(*node)
	walkRows = func(current *node) {
		if current.tag == "tr" {
			var row []string
			for _, cell := range current.children {
				if cell.tag == "td" || cell.tag == "th" {
					row = append(row, escapeTableCell(inline(cell)))
				}
			}
			if len(row) > 0 {
				rows = append(rows, row)
			}
			return
		}
		for _, child := range current.children {
			walkRows(child)
		}
	}

	walkRows(n)
	return rows
}

func textContent(n *node) string {
	if n.tag == "" {
		return n.text
	}

	var b strings.Builder
	for _, child := range n.children {
		b.WriteString(textContent(child))
	}

	return b.String()
}

func normalizeInline(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

func normalizeTextNode(text string) string {
	if strings.TrimSpace(text) == "" {
		if text == "" {
			return ""
		}
		if strings.ContainsAny(text, "\n\r") {
			return ""
		}
		return " "
	}

	hasLeadingSpace := unicode.IsSpace(rune(text[0]))
	hasTrailingSpace := unicode.IsSpace(rune(text[len(text)-1]))
	text = normalizeInline(normalizeMathText(text))
	if hasLeadingSpace {
		text = " " + text
	}
	if hasTrailingSpace {
		text += " "
	}

	return text
}

var texReplacer = strings.NewReplacer(
	"\\leftrightarrow", "<->",
	"\\Leftrightarrow", "<=>",
	"\\rightarrow", "->",
	"\\Rightarrow", "=>",
	"\\leftarrow", "<-",
	"\\Leftarrow", "<=",
	"\\longleftrightarrow", "<->",
	"\\Longleftrightarrow", "<=>",
	"\\longrightarrow", "->",
	"\\Longrightarrow", "=>",
	"\\longleftarrow", "<-",
	"\\Longleftarrow", "<=",
	"\\mapsto", "|->",
	"\\iff", "<=>",
	"\\to", "->",
	"\\gets", "<-",
	"\\infty", "∞",
	"\\leq", "<=",
	"\\le", "<=",
	"\\geq", ">=",
	"\\ge", ">=",
	"\\neq", "!=",
	"\\ne", "!=",
	"\\equiv", "===",
	"\\approx", "~",
	"\\sim", "~",
	"\\cdot", "*",
	"\\times", "*",
	"\\div", "/",
	"\\pm", "+/-",
	"\\mp", "-/+",
	"\\land", "∧",
	"\\wedge", "∧",
	"\\lor", "∨",
	"\\vee", "∨",
	"\\neg", "¬",
	"\\lnot", "¬",
	"\\forall", "∀",
	"\\exists", "∃",
	"\\nexists", "∄",
	"\\in", "∈",
	"\\notin", "∉",
	"\\ni", "∋",
	"\\subseteq", "⊆",
	"\\subset", "⊂",
	"\\supseteq", "⊇",
	"\\supset", "⊃",
	"\\cup", "∪",
	"\\cap", "∩",
	"\\emptyset", "∅",
	"\\varnothing", "∅",
	"\\nabla", "∇",
	"\\partial", "∂",
	"\\sum", "Σ",
	"\\prod", "Π",
	"\\int", "∫",
	"\\alpha", "α",
	"\\beta", "β",
	"\\gamma", "γ",
	"\\Gamma", "Γ",
	"\\delta", "δ",
	"\\Delta", "Δ",
	"\\epsilon", "ε",
	"\\varepsilon", "ε",
	"\\zeta", "ζ",
	"\\eta", "η",
	"\\theta", "θ",
	"\\Theta", "Θ",
	"\\lambda", "λ",
	"\\Lambda", "Λ",
	"\\mu", "μ",
	"\\nu", "ν",
	"\\xi", "ξ",
	"\\Xi", "Ξ",
	"\\pi", "π",
	"\\Pi", "Π",
	"\\rho", "ρ",
	"\\sigma", "σ",
	"\\Sigma", "Σ",
	"\\tau", "τ",
	"\\phi", "φ",
	"\\varphi", "φ",
	"\\Phi", "Φ",
	"\\omega", "ω",
	"\\Omega", "Ω",
	"\\ldots", "...",
	"\\dots", "...",
	"\\quad", " ",
	"\\qquad", " ",
)

func normalizeMathText(text string) string {
	if !looksLikeTex(text) {
		return text
	}

	text = replaceTexCommandWithTwoGroups(text, "\\frac", "(%s)/(%s)")
	text = replaceTexCommandWithOneGroup(text, "\\sqrt", "sqrt(%s)")
	text = replaceTexCommandWithOneGroup(text, "\\overline", "overline(%s)")
	text = replaceTexCommandWithOneGroup(text, "\\underline", "underline(%s)")
	text = replaceTexCommandWithOneGroup(text, "\\text", "%s")
	text = replaceTexCommandWithOneGroup(text, "\\mathrm", "%s")
	text = replaceTexCommandWithOneGroup(text, "\\mathbf", "%s")
	text = replaceTexCommandWithOneGroup(text, "\\mathit", "%s")
	text = texReplacer.Replace(text)

	for _, command := range []string{
		"\\math",
		"\\displaystyle",
		"\\textstyle",
		"\\scriptstyle",
		"\\scriptscriptstyle",
		"\\left",
		"\\right",
		"\\,",
		"\\;",
		"\\:",
		"\\!",
	} {
		text = strings.ReplaceAll(text, command, "")
	}

	text = normalizeGroupedScripts(text)
	return strings.TrimSpace(normalizeInline(text))
}

func looksLikeTex(text string) bool {
	return strings.Contains(text, "\\") || strings.Contains(text, "^{") || strings.Contains(text, "_{")
}

func replaceTexCommandWithOneGroup(text string, command string, format string) string {
	for {
		start := strings.Index(text, command+"{")
		if start == -1 {
			return text
		}

		groupStart := start + len(command)
		group, end, ok := readBraceGroup(text, groupStart)
		if !ok {
			return text
		}

		replacement := formatTexGroup(format, normalizeMathText(group))
		text = text[:start] + replacement + text[end:]
	}
}

func replaceTexCommandWithTwoGroups(text string, command string, format string) string {
	for {
		start := strings.Index(text, command+"{")
		if start == -1 {
			return text
		}

		first, firstEnd, ok := readBraceGroup(text, start+len(command))
		if !ok {
			return text
		}
		second, secondEnd, ok := readBraceGroup(text, firstEnd)
		if !ok {
			return text
		}

		replacement := formatTexGroup(format, normalizeMathText(first), normalizeMathText(second))
		text = text[:start] + replacement + text[secondEnd:]
	}
}

func formatTexGroup(format string, values ...any) string {
	return strings.TrimSpace(strings.ReplaceAll(fmt.Sprintf(format, values...), "  ", " "))
}

func readBraceGroup(text string, start int) (string, int, bool) {
	for start < len(text) && unicode.IsSpace(rune(text[start])) {
		start++
	}
	if start >= len(text) || text[start] != '{' {
		return "", start, false
	}

	depth := 0
	for i := start; i < len(text); i++ {
		switch text[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return text[start+1 : i], i + 1, true
			}
		}
	}

	return "", start, false
}

func normalizeGroupedScripts(text string) string {
	for {
		index := strings.IndexAny(text, "_^")
		if index == -1 || index+1 >= len(text) {
			return text
		}
		if text[index+1] != '{' {
			next := index + 1
			text = text[:next] + normalizeGroupedScripts(text[next:])
			return text
		}

		group, end, ok := readBraceGroup(text, index+1)
		if !ok {
			return text
		}
		text = text[:index+1] + "(" + normalizeMathText(group) + ")" + text[end:]
	}
}

func cleanup(markdown string) string {
	lines := strings.Split(strings.TrimSpace(markdown), "\n")
	cleaned := make([]string, 0, len(lines))
	blank := false

	for _, line := range lines {
		line = strings.TrimRightFunc(line, unicode.IsSpace)
		if strings.TrimSpace(line) == "" {
			if !blank && len(cleaned) > 0 {
				cleaned = append(cleaned, "")
			}
			blank = true
			continue
		}
		line = strings.Join(strings.Fields(line), " ")
		cleaned = append(cleaned, line)
		blank = false
	}

	return strings.TrimSpace(strings.Join(cleaned, "\n"))
}

func escapeLinkText(text string) string {
	replacer := strings.NewReplacer("[", "\\[", "]", "\\]")
	return replacer.Replace(text)
}

func escapeTableCell(text string) string {
	return strings.ReplaceAll(text, "|", "\\|")
}

func shouldSkip(n *node) bool {
	if n.tag == "footer" {
		return true
	}

	switch n.attrs["id"] {
	case "toc", "catlinks", "printfooter", "mw-navigation", "mw-head", "mw-panel", "footer", "siteNotice":
		return true
	}

	for _, class := range []string{
		"mw-editsection",
		"noprint",
		"metadata",
		"navbox",
		"ambox",
		"vertical-navbox",
		"printfooter",
	} {
		if hasClass(n, class) {
			return true
		}
	}

	return false
}

func hasClass(n *node, class string) bool {
	for _, current := range strings.Fields(n.attrs["class"]) {
		if current == class {
			return true
		}
	}

	return false
}

func isServiceLink(href string) bool {
	href = strings.TrimSpace(strings.ToLower(href))
	return href == "" || strings.HasPrefix(href, "#") || strings.HasPrefix(href, "javascript:")
}

func shortHref(href string) string {
	href = strings.TrimSpace(href)
	if href == "" {
		return ""
	}
	if len(href) <= 80 {
		return href
	}

	return href[:77] + "..."
}

func isVoidTag(tag string) bool {
	switch tag {
	case "area", "base", "br", "col", "embed", "hr", "img", "input", "link", "meta", "param", "source", "track", "wbr":
		return true
	default:
		return false
	}
}
