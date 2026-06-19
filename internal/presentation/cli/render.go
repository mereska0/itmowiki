package cli

import (
	"strings"

	"github.com/mereska0/itmowiki/internal/domain"
	"github.com/mereska0/itmowiki/internal/parser/markdown"

	"github.com/charmbracelet/glamour"
)

type PageRenderer struct {
	style string
}

func NewPageRenderer(style string) *PageRenderer {
	return &PageRenderer{style: style}
}

func (r *PageRenderer) Render(page domain.Page) (string, error) {
	var b strings.Builder

	if page.Title != "" {
		b.WriteString(page.Title)
		b.WriteString("\n")
	}

	b.WriteString(page.URL)
	b.WriteString("\n\n")

	content, err := glamour.Render(markdown.Parse(page.HTML), r.style)
	if err != nil {
		return "", err
	}

	b.WriteString(content)
	return b.String(), nil
}
