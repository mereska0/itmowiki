package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/mereska0/itmowiki/internal/app"
	"github.com/mereska0/itmowiki/internal/domain"
	"github.com/mereska0/itmowiki/internal/presentation/cli"
	"github.com/mereska0/itmowiki/internal/storage"
)

const (
	defaultCrawlURL   = "https://neerc.ifmo.ru/wiki/index.php"
	defaultCrawlLimit = 100
)

func main() {
	if len(os.Args) == 1 {
		printHome()
		return
	}

	command := os.Args[1]
	if command == "help" || command == "--help" || command == "-h" {
		printHelp()
		return
	}

	store, err := storage.NewLocalStore(storage.DefaultLocalStorePath())
	if err != nil {
		log.Fatal("failed to open local storage:", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			log.Println("failed to close storage:", err)
		}
	}()

	searchUseCase := app.NewSearchUseCase(store)
	showUseCase := app.NewShowUseCase(store)
	crawlUseCase := app.NewCrawlUseCase(store)
	pageRenderer := cli.NewPageRenderer("dark")

	switch command {
	case "search":
		runSearch(searchUseCase, os.Args[2:])
	case "crawl":
		runCrawl(crawlUseCase, os.Args[2:])
	case "show":
		runShow(showUseCase, pageRenderer, os.Args[2:])
	default:
		fmt.Printf("Неизвестная команда: %s\n\n", command)
		printHelp()
		os.Exit(1)
	}
}

func printHome() {
	fmt.Print(`╭────────────────────────────────────────╮
│                                        │
│        ☆  i t m o w i k i  ☆           │
│      маленький поиск по конспектам     │
│                                        │
╰────────────────────────────────────────╯

Команды:
  itmowiki search <запрос>
  itmowiki show <id>
  itmowiki crawl
  itmowiki crawl <url> [limit]
  itmowiki help
`)
}

func printHelp() {
	fmt.Print(`itmowiki - поиск по Викиконспектам

Использование:
  itmowiki
  itmowiki search <запрос>
  itmowiki show <id>
  itmowiki crawl
  itmowiki crawl <url> [limit]

Примеры:
  itmowiki search алгоритмы
  itmowiki search теория вероятностей
  itmowiki show 15
  itmowiki crawl
  itmowiki crawl "https://neerc.ifmo.ru/wiki/index.php?title=Заглавная_страница" 20
`)
}

func runSearch(searchUseCase *app.SearchUseCase, args []string) {
	if len(args) == 0 {
		log.Fatal("Использование: itmowiki search <запрос>")
	}

	query := strings.Join(args, " ")

	results, err := searchUseCase.Execute(query)
	if err != nil {
		log.Fatal("search failed:", err)
	}

	if len(results) == 0 {
		fmt.Println("Ничего не найдено")
		return
	}

	fmt.Printf("Найдено страниц: %d\n\n", len(results))

	for _, result := range results {
		title := result.Title
		if title == "" {
			title = "(без заголовка)"
		}

		fmt.Printf("[%d] %s\n", result.ID, title)
		fmt.Println(result.URL)
		fmt.Println()
	}
}

func runShow(showUseCase *app.ShowUseCase, pageRenderer *cli.PageRenderer, args []string) {
	if len(args) != 1 {
		log.Fatal("Использование: itmowiki show <id>")
	}

	id, err := strconv.Atoi(args[0])
	if err != nil {
		log.Fatal("id должен быть числом")
	}

	page, err := showUseCase.Execute(id)
	if errors.Is(err, domain.ErrPageNotFound) {
		log.Fatalf("страница с id %d не найдена", id)
	}
	if err != nil {
		log.Fatal("failed to load page:", err)
	}

	rendered, err := pageRenderer.Render(page)
	if err != nil {
		log.Fatal("failed to render page:", err)
	}

	fmt.Println(rendered)
}

func runCrawl(crawlUseCase *app.CrawlUseCase, args []string) {
	startURL := defaultCrawlURL
	limit := defaultCrawlLimit

	if len(args) >= 1 {
		startURL = args[0]
	}
	if len(args) >= 2 {
		value, err := strconv.Atoi(args[1])
		if err != nil || value <= 0 {
			log.Fatal("limit должен быть положительным числом")
		}
		limit = value
	}
	if len(args) > 2 {
		log.Fatal("Использование: itmowiki crawl [url] [limit]")
	}

	if !cli.IsTerminal(os.Stdout) {
		count, err := crawlUseCase.Execute(startURL, limit, nil)
		if err != nil {
			log.Fatal("crawl failed:", err)
		}
		fmt.Printf("Обработано страниц: %d\n", count)
		return
	}

	if err := cli.RunCrawlTUI(crawlUseCase, startURL, limit); err != nil {
		log.Fatal("crawl failed:", err)
	}
}
