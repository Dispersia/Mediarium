package api_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A book's titles in other languages are looked up when it is added, and the
// release search uses them: an English book's Japanese release is found,
// matched and filed under its Japanese title.
// NDL Search is asked for an English book too: the Japanese edition of
// "Dungeon Crawler Carl" is filed as "冒険者カールの地球ダンジョン", a title Open
// Library and Google Books do not list.
func TestBookTitlesAskNDLForAnEnglishBook(t *testing.T) {
	server, base, client := loginNewServer(t)
	ol := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/works/OLDCC1W/editions.json" {
			fmt.Fprint(w, `{"entries":[{"title":"Dungeon Crawler Carl","languages":[{"key":"/languages/eng"}]}]}`)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(ol.Close)
	server.TestSetOpenLibrary(ol.URL)
	server.TestSetGoogleBooks("http://127.0.0.1:1")

	var queries []string
	ndl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queries = append(queries, r.URL.Query().Get("query"))
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?>
<searchRetrieveResponse xmlns="http://www.loc.gov/zing/srw/">
  <version>1.2</version>
  <records><record><recordData>
  &lt;srw_dc:dc xmlns:dc="http://purl.org/dc/elements/1.1/"&gt;
    &lt;dc:title&gt;冒険者カールの地球ダンジョン&lt;/dc:title&gt;
    &lt;dc:creator&gt;マット・ディニマン 著&lt;/dc:creator&gt;
    &lt;dc:description&gt;原タイトル: DUNGEON CRAWLER CARL&lt;/dc:description&gt;
    &lt;dc:language&gt;jpn&lt;/dc:language&gt;
  &lt;/srw_dc:dc&gt;
  </recordData></record></records>
</searchRetrieveResponse>`)
	}))
	t.Cleanup(ndl.Close)
	server.TestSetNDL(ndl.URL)

	ebooks := filepath.Join(t.TempDir(), "ebooks")
	if err := os.MkdirAll(ebooks, 0o755); err != nil {
		t.Fatal(err)
	}
	postJSONMethod[map[string]any](t, client, http.MethodPut, base+"/api/modules", map[string]any{"ebooks": true}, http.StatusOK)
	postJSONMethod[map[string]any](t, client, http.MethodPut, base+"/api/settings", map[string]any{"ebooksPath": ebooks}, http.StatusOK)
	book := postJSON[map[string]any](t, client, base+"/api/books", map[string]any{
		"olKey": "OLDCC1W", "title": "Dungeon Crawler Carl", "author": "Matt Dinniman", "year": 2020, "ebook": true}, http.StatusCreated)
	bookURL := fmt.Sprintf("%s/api/books/%v", base, book["id"])

	refreshed := postJSON[map[string]any](t, client, bookURL+"/titles/refresh", nil, http.StatusOK)
	titles, _ := refreshed["titles"].([]any)
	if len(titles) != 1 {
		t.Fatalf("titles: %v", refreshed["titles"])
	}
	if got := titles[0].(map[string]any); got["lang"] != "Japanese" || got["title"] != "冒険者カールの地球ダンジョン" {
		t.Fatalf("title: %v", got)
	}
	if len(queries) == 0 || queries[0] != `title="Dungeon Crawler Carl"` {
		t.Fatalf("NDL should be asked for the English title: %v", queries)
	}
}

func TestBookTitlesFindAndFileOtherLanguages(t *testing.T) {
	server, base, client := loginNewServer(t)
	ol := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search.json":
			fmt.Fprint(w, `{"docs":[{"key":"/works/OLNW","title":"Norwegian Wood","author_name":["Haruki Murakami"],"author_key":["OL1A"],"first_publish_year":1987,"cover_i":1,"format":["eBook"]}]}`)
		case "/works/OLNW.json":
			fmt.Fprint(w, `{"title":"Norwegian Wood","covers":[1]}`)
		case "/works/OLNW/editions.json":
			fmt.Fprint(w, `{"entries":[{"title":"Norwegian Wood","languages":[{"key":"/languages/eng"}]},{"title":"ノルウェイの森","languages":[{"key":"/languages/jpn"}]},{"title":"挪威的森林","languages":[{"key":"/languages/chi"}]}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(ol.Close)
	server.TestSetOpenLibrary(ol.URL)
	// Google Books is not asked in this test: an address nothing answers on, so
	// the lookup falls back to the Open Library edition quickly.
	server.TestSetGoogleBooks("http://127.0.0.1:1")

	root := t.TempDir()
	ebooks := filepath.Join(root, "ebooks")
	if err := os.MkdirAll(ebooks, 0o755); err != nil {
		t.Fatal(err)
	}
	postJSONMethod[map[string]any](t, client, http.MethodPut, base+"/api/modules", map[string]any{"ebooks": true}, http.StatusOK)
	postJSONMethod[map[string]any](t, client, http.MethodPut, base+"/api/settings", map[string]any{"ebooksPath": ebooks}, http.StatusOK)

	book := postJSON[map[string]any](t, client, base+"/api/books", map[string]any{
		"olKey": "OLNW", "title": "Norwegian Wood", "author": "Haruki Murakami", "year": 1987, "ebook": true}, http.StatusCreated)
	bookURL := fmt.Sprintf("%s/api/books/%v", base, book["id"])

	// "Look up titles" stores the Japanese and Chinese titles (the English one
	// is dropped: it equals the catalogue title).
	refreshed := postJSON[map[string]any](t, client, bookURL+"/titles/refresh", nil, http.StatusOK)
	titles, _ := refreshed["titles"].([]any)
	if len(titles) != 2 {
		t.Fatalf("titles: %v", refreshed["titles"])
	}
	got := map[string]string{}
	for _, raw := range titles {
		tl := raw.(map[string]any)
		got[tl["lang"].(string)] = tl["title"].(string)
	}
	if got["Japanese"] != "ノルウェイの森" || got["Chinese"] != "挪威的森林" {
		t.Fatalf("titles: %v", titles)
	}
	// They are on the book from now on.
	if got := getJSON[map[string]any](t, client, bookURL); len(got["titles"].([]any)) != 2 {
		t.Fatalf("the book payload carries the titles: %v", got["titles"])
	}

	// An indexer that only has the book in Japanese.
	articles := map[string]nntpArticle{
		"e1@x": {fileName: "ノルウェイの森.epub", content: []byte("epub " + strings.Repeat("e", 300))},
	}
	host, port := newArticleNNTPServer(t, articles)
	idx := newMusicIndexer(t, []string{"村上春樹 - ノルウェイの森 (1987) [EPUB]", "村上春樹 - 挪威的森林 (1987) [EPUB]"}, nzbFor(articles))
	postJSON[map[string]any](t, client, base+"/api/indexers", map[string]any{"name": "Books", "definitionId": "x", "baseUrl": idx.URL, "apiKey": "k", "protocol": "usenet"}, http.StatusCreated)
	postJSON[map[string]any](t, client, base+"/api/usenet-servers", map[string]any{"name": "Fixture Usenet", "host": host, "port": port, "useSsl": false, "connections": 2}, http.StatusCreated)

	// The names say their language by their script, so neither is taken while
	// English is the wanted language.
	releases := getJSON[[]map[string]any](t, client, bookURL+"/releases?format=ebook")
	if len(releases) != 2 {
		t.Fatalf("the Japanese and Chinese releases should be listed: %v", releases)
	}
	langs := map[string]string{}
	for _, res := range releases {
		langs[res["title"].(string)] = res["language"].(string)
		if res["languageFit"] != "other" {
			t.Fatalf("a release in another language should be left alone: %v", res)
		}
	}
	if langs["村上春樹 - ノルウェイの森 (1987) [EPUB]"] != "Japanese" || langs["村上春樹 - 挪威的森林 (1987) [EPUB]"] != "Chinese" {
		t.Fatalf("the releases should be read as Japanese and Chinese: %v", langs)
	}
	postJSONMethod[map[string]any](t, client, http.MethodPut, base+"/api/settings", map[string]any{"qualityLanguage": "Japanese", "qualityLanguageOnly": true}, http.StatusOK)

	now := postJSON[map[string]any](t, client, bookURL+"/search?format=ebook", nil, http.StatusOK)
	if now["grabbed"] != float64(1) {
		t.Fatalf("search now: %v", now)
	}
	waitBackground(t, server)

	want := filepath.Join(ebooks, "Haruki Murakami", "ノルウェイの森 (1987)", "ノルウェイの森.epub")
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("the ebook should be filed under its Japanese title at %s: %v", want, err)
	}
}
