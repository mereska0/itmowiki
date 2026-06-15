package markdown

import (
	"strings"
	"testing"
)

func TestParseUsesMediaWikiParserOutputOnly(t *testing.T) {
	rawHTML := []byte(`
		<html>
			<body>
				<div id="mw-navigation"><p>Навигация</p></div>
				<div class="mw-parser-output">
					<h1>Эволюционные алгоритмы</h1>
					<p>Основной текст статьи.</p>
				</div>
				<footer><p>Подвал</p></footer>
			</body>
		</html>
	`)

	got := Parse(rawHTML)
	want := "# Эволюционные алгоритмы\n\nОсновной текст статьи."
	if got != want {
		t.Fatalf("markdown = %q, want %q", got, want)
	}
}

func TestParseFallsBackToMwContentText(t *testing.T) {
	rawHTML := []byte(`
		<body>
			<div id="mw-content-text">
				<p>Контент без parser-output.</p>
			</div>
			<div id="mw-panel">Меню</div>
		</body>
	`)

	got := Parse(rawHTML)
	want := "Контент без parser-output."
	if got != want {
		t.Fatalf("markdown = %q, want %q", got, want)
	}
}

func TestParseSkipsMediaWikiServiceBlocks(t *testing.T) {
	rawHTML := []byte(`
		<div class="mw-parser-output">
			<div id="toc">Оглавление</div>
			<p>До <span class="mw-editsection">[править]</span> после.</p>
			<div class="noprint">Не печатать</div>
			<div class="metadata">Метаданные</div>
			<div class="navbox">Навигационный блок</div>
			<div class="printfooter">Источник</div>
			<div id="catlinks">Категории</div>
		</div>
	`)

	got := Parse(rawHTML)
	if got != "До после." {
		t.Fatalf("markdown = %q, want %q", got, "До после.")
	}
}

func TestParseRendersLinksAsTextWithoutLongURLs(t *testing.T) {
	rawHTML := []byte(`
		<div class="mw-parser-output">
			<p><a href="/wiki/index.php?title=%D0%A2%D0%B5%D0%BE%D1%80%D0%B5%D0%BC%D0%B0_%D0%BE_%D0%B4%D1%80%D0%B8%D1%84%D1%82%D0%B5">Теорема о дрифте</a></p>
			<p><a href="/short"></a></p>
			<p><a href="#toc"></a></p>
		</div>
	`)

	got := Parse(rawHTML)
	if strings.Contains(got, "%D0") || strings.Contains(got, "](") {
		t.Fatalf("markdown contains long URL markup: %q", got)
	}
	if !strings.Contains(got, "Теорема о дрифте") {
		t.Fatalf("markdown does not contain link text: %q", got)
	}
	if !strings.Contains(got, "/short") {
		t.Fatalf("markdown does not contain short empty-link href: %q", got)
	}
}

func TestParseKeepsBasicMarkdownElements(t *testing.T) {
	rawHTML := []byte(`
		<div class="mw-parser-output">
			<h2>Раздел</h2>
			<p><strong>Жирный</strong>, <em>курсив</em> и <code>code</code>.</p>
			<ul><li>Один</li><li>Два</li></ul>
			<table>
				<tr><th>A</th><th>B</th></tr>
				<tr><td>1</td><td>2</td></tr>
			</table>
			<pre>x := 1</pre>
		</div>
	`)

	got := Parse(rawHTML)
	for _, want := range []string{
		"## Раздел",
		"**Жирный**, *курсив* и `code`.",
		"- Один\n- Два",
		"| A | B |",
		"```\nx := 1\n```",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("markdown %q does not contain %q", got, want)
		}
	}
}
