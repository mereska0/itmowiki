package domain

type IndexStats struct {
	PagesDiscovered int
	PagesCrawled    int
	LinksStored     int
	KeywordsIndexed int
	TopLinkedPages  []LinkedPageStat
	TopKeywords     []KeywordStat
}

type LinkedPageStat struct {
	PageID        int
	Title         string
	URL           string
	IncomingLinks int
}

type KeywordStat struct {
	Keyword string
	Count   int
}
