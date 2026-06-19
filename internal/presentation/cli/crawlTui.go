package cli

import (
	"fmt"
	"time"

	"github.com/mereska0/itmowiki/internal/app"

	tea "github.com/charmbracelet/bubbletea"
)

type crawlProgressMsg app.CrawlProgress

type crawlDoneMsg struct {
	count int
	err   error
}

type tickMsg time.Time

type crawlModel struct {
	crawlUseCase *app.CrawlUseCase
	startURL     string
	maxPages     int
	msgs         chan tea.Msg

	err     error
	count   int
	current int
	total   int
	url     string
	cached  bool
	frame   int
	done    bool
}

func RunCrawlTUI(crawlUseCase *app.CrawlUseCase, startURL string, limit int) error {
	model := crawlModel{
		crawlUseCase: crawlUseCase,
		startURL:     startURL,
		maxPages:     limit,
		msgs:         make(chan tea.Msg, limit+2),
		total:        limit,
		url:          startURL,
	}

	finalModel, err := tea.NewProgram(model).Run()
	if err != nil {
		return err
	}

	loadedModel := finalModel.(crawlModel)
	return loadedModel.err
}

func (m crawlModel) Init() tea.Cmd {
	return tea.Batch(m.crawlCmd(), m.waitMsgCmd(), tickCmd())
}

func (m crawlModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || msg.String() == "esc" {
			return m, tea.Quit
		}
	case tickMsg:
		if m.done {
			return m, nil
		}
		m.frame++
		return m, tickCmd()
	case crawlProgressMsg:
		m.current = msg.Current
		m.total = msg.Total
		m.url = msg.URL
		m.cached = msg.Cached
		return m, m.waitMsgCmd()
	case crawlDoneMsg:
		m.count = msg.count
		m.err = msg.err
		m.done = true
		return m, tea.Quit
	}

	return m, nil
}

func (m crawlModel) View() string {
	if m.done {
		if m.err != nil {
			return fmt.Sprintf("Ошибка загрузки: %v\n", m.err)
		}
		return fmt.Sprintf("Обработано страниц: %d\n", m.count)
	}

	total := m.total
	if total == 0 {
		total = m.maxPages
	}
	percent := 0
	if total > 0 {
		percent = m.current * 100 / total
	}
	if percent > 100 {
		percent = 100
	}

	status := "fetch"
	if m.cached {
		status = "cache"
	}
	spinner := []string{"|", "/", "-", "\\"}

	return fmt.Sprintf(
		"%s itmowiki crawl: %d%% (%d/%d) [%s]\n%s\n\nEsc/Ctrl+C - отменить\n",
		spinner[m.frame%len(spinner)],
		percent,
		m.current,
		total,
		status,
		shortText(m.url, 100),
	)
}

func (m crawlModel) crawlCmd() tea.Cmd {
	return func() tea.Msg {
		go func() {
			count, err := m.crawlUseCase.Execute(
				m.startURL,
				m.maxPages,
				func(progress app.CrawlProgress) {
					m.msgs <- crawlProgressMsg(progress)
				},
			)

			m.msgs <- crawlDoneMsg{count: count, err: err}
		}()

		return nil
	}
}

func (m crawlModel) waitMsgCmd() tea.Cmd {
	return func() tea.Msg {
		return <-m.msgs
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func shortText(text string, maxLen int) string {
	runes := []rune(text)
	if len(runes) <= maxLen {
		return text
	}

	return string(runes[:maxLen-3]) + "..."
}
