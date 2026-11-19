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
func TestBookTitlesFindAndFileOtherLanguages(t *testing.T) {
	server, base, client := loginNewServer(t)
	ol := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search.json":
			fmt.Fprint(w, `{"docs":[{"key":"/works/OLNW","title":"Norwegian Wood","author_name":["Haruki Murakami"],"author_key":["OL1A"],"first_publish_year":1987,"cover_i":1,"format":["eBook"]}]}`)
		case "/works/OLNW.json":
			fmt.Fprint(w, `{"title":"Norwegian Wood","covers":[1]}`)
		case "/works/OLNW/editions.json":
			fmt.Fprint(w, `{"entries":[{"title":"Norwegian Wood","languages":[{"key":"/languages/eng"}]},{"title":"ノルウェイの森","languages":[{"key":"/languages/jpn"}]}]}`)
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

	// "Look up titles" stores the Japanese title (the English one is dropped:
	// it equals the catalogue title).
	refreshed := postJSON[map[string]any](t, client, bookURL+"/titles/refresh", nil, http.StatusOK)
	titles, _ := refreshed["titles"].([]any)
	if len(titles) != 1 {
		t.Fatalf("titles: %v", refreshed["titles"])
	}
	if got := titles[0].(map[string]any); got["lang"] != "Japanese" || got["title"] != "ノルウェイの森" {
		t.Fatalf("title: %v", got)
	}
	// It is on the book from now on.
	if got := getJSON[map[string]any](t, client, bookURL); len(got["titles"].([]any)) != 1 {
		t.Fatalf("the book payload carries the titles: %v", got["titles"])
	}

	// An indexer that only has the book in Japanese.
	articles := map[string]nntpArticle{
		"e1@x": {fileName: "ノルウェイの森.epub", content: []byte("epub " + strings.Repeat("e", 300))},
	}
	host, port := newArticleNNTPServer(t, articles)
	idx := newMusicIndexer(t, []string{"村上春樹 - ノルウェイの森 (1987) [EPUB]"}, nzbFor(articles))
	postJSON[map[string]any](t, client, base+"/api/indexers", map[string]any{"name": "Books", "definitionId": "x", "baseUrl": idx.URL, "apiKey": "k", "protocol": "usenet"}, http.StatusCreated)
	postJSON[map[string]any](t, client, base+"/api/usenet-servers", map[string]any{"name": "Fixture Usenet", "host": host, "port": port, "useSsl": false, "connections": 2}, http.StatusCreated)

	releases := getJSON[[]map[string]any](t, client, bookURL+"/releases?format=ebook")
	if len(releases) != 1 || releases[0]["title"] != "村上春樹 - ノルウェイの森 (1987) [EPUB]" {
		t.Fatalf("the Japanese release should be listed: %v", releases)
	}

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
