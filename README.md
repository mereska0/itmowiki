<img width="386" height="107" alt="image" src="https://github.com/user-attachments/assets/af8dfd56-017c-433a-812f-71e0c0016686" />


`itmowiki` is a terminal-based search tool for ITMO Wikiconspects.

It crawls pages from the ITMO wiki, stores them locally, lets you search through indexed pages, and renders selected pages directly in the terminal as readable Markdown.

## Features

* Crawl ITMO Wikiconspects pages from the terminal
* Crawl pages concurrently
* Store crawled pages locally
* Search pages by keywords
* View saved pages in the terminal
* Render HTML pages as readable Markdown
* No browser required
* No external database required

## Tech Stack

* Go
* `net/http`
* Bubble Tea
* Glamour
* Local JSON storage
* Homebrew


## Installation

### HomeBrew

```bash
brew install mereska0/tap/itmowiki
```

## Usage

### Show help

```bash
itmowiki help
```

### Crawl pages(this command should be ran initially)

```bash
itmowiki crawl
```

You can also provide a custom start URL and page limit:

```bash
itmowiki crawl "https://neerc.ifmo.ru/wiki/index.php?title=Заглавная_страница" 50
```

`crawl` runs pages concurrently by default, so indexing is faster than a
one-page-at-a-time crawl.

Usage:

```text
itmowiki crawl [url] [limit]
```

### Search pages

```bash
itmowiki search алгоритмы
```

The command prints matching pages with their IDs.

### Show a page

```bash
itmowiki show <page_id>
```

The selected page will be rendered in the terminal.

## License

MIT
