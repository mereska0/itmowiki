package parser

/* structure for html page*/
type HtmlPage struct {
	title    string
	keywords []string
	links    []string
	htmlcode []byte
}

func NewHtmlPage(htmlcode []byte) *HtmlPage {
	return &HtmlPage{htmlcode: htmlcode}
}

func (h *HtmlPage) SetTitle(title string) {
	h.title = title
}

func (h *HtmlPage) AppendKeywords(keyword string) {
	h.keywords = append(h.keywords, keyword)
}

func (h *HtmlPage) AppendLink(link string) {
	h.links = append(h.links, link)
}

func (h *HtmlPage) GetKeywords() []string {
	return h.keywords
}

func (h *HtmlPage) GetLinks() []string {
	return h.links
}

func (h *HtmlPage) GetTitle() string {
	return h.title
}
func (h *HtmlPage) GetHtml() []byte {
	return h.htmlcode
}
