package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rdborg/mediarium/internal/books"
)

// A book's titles in other languages (books.Title). They are read from Open
// Library's editions and from Google Books, kept in the book_titles table, and
// used to search the indexers for a translation and to match and file its
// release. A release of "Norwegian Wood" is named "ノルウェイの森", so without
// this the Japanese release is never found or matched.

// bookTitleFetches keeps one lookup per book in flight, so opening a book page
// a few times asks the catalogues once.
var bookTitleFetches sync.Map // int64 -> struct{}

// bookTitlesLookupWait is how long the background lookup of a book's titles
// may take before it is given up and tried another time.
const bookTitlesLookupWait = 30 * time.Second

// bookTitlesLater looks a book's titles in other languages up in the
// background when they have not been looked up yet. It is called when a book
// is added and when its page is opened. A book added by hand is left alone.
func (s *Server) bookTitlesLater(b books.Book) {
	if b.TitlesCheckedAt != "" || books.IsManualKey(b.OLKey) {
		return
	}
	if _, busy := bookTitleFetches.LoadOrStore(b.ID, struct{}{}); busy {
		return
	}
	s.background(func() {
		defer bookTitleFetches.Delete(b.ID)
		ctx, cancel := context.WithTimeout(context.Background(), bookTitlesLookupWait)
		defer cancel()
		if _, err := s.refreshBookTitles(ctx, b); err != nil {
			slog.Info("books: look up the titles in other languages", "book", b.ID, "err", err)
		}
	})
}

// refreshBookTitles fetches a book's titles in other languages and saves them.
// It asks Open Library's editions first, then Google Books (all languages, not
// the info language), keeps one title per language, and keeps a title a person
// set by hand. A title equal to the catalogue title is not stored. When
// neither catalogue answered, nothing is saved and the book is looked up again
// another time.
func (s *Server) refreshBookTitles(ctx context.Context, b books.Book) ([]books.Title, error) {
	if books.IsManualKey(b.OLKey) {
		return b.Titles, nil
	}
	var out []books.Title
	add := func(lang, title, source string) {
		lang, title = strings.TrimSpace(lang), strings.TrimSpace(title)
		if lang == "" || title == "" || strings.EqualFold(title, b.Title) {
			return
		}
		for i := range out {
			if strings.EqualFold(out[i].Lang, lang) {
				return // one title per language: the first source wins
			}
		}
		out = append(out, books.Title{Lang: lang, Title: title, Source: source})
	}
	for _, t := range b.Titles {
		if t.Source == "manual" {
			out = append(out, t)
		}
	}
	answered := false
	titles, err := s.OpenLibrary.Titles(ctx, b.OLKey)
	switch {
	case err == nil, errors.Is(err, books.ErrNotFound):
		answered = true
		for _, t := range titles {
			add(t.Lang, t.Title, "openlibrary")
		}
	default:
		slog.Info("books: Open Library editions", "book", b.ID, "err", err)
	}
	// NDL Search is the authoritative Japanese catalogue: ask it for every
	// book, not only one written in Japanese, because a translation is filed
	// under a Japanese title the English catalogues do not list (the Japanese
	// "冒険者カールの地球ダンジョン" for "Dungeon Crawler Carl"). Its title wins
	// over Google Books'.
	if s.NDL != nil {
		ndl, err := s.NDL.Titles(ctx, b.Title, b.Author)
		if err == nil {
			answered = true
			for _, t := range ndl {
				add(t.Lang, t.Title, "ndl")
			}
		} else {
			slog.Info("books: NDL Search", "book", b.ID, "err", err)
		}
	}
	q := strings.TrimSpace(b.Title + " " + b.Author)
	hits, err := s.googleBooks(s.savedGoogleBooksKey()).SearchAny(ctx, q, 20)
	if err == nil {
		answered = true
		for _, h := range hits {
			if lang := books.LanguageFromISO(h.Language); lang != "" {
				add(lang, h.Title, "google")
			}
		}
	} else {
		slog.Info("books: Google Books editions", "book", b.ID, "err", err)
	}
	if !answered {
		return nil, errors.New("neither Open Library nor Google Books answered")
	}
	if err := s.BookRepo.SetTitles(b.ID, out); err != nil {
		return nil, err
	}
	return out, nil
}

// handleRefreshBookTitles looks a book's titles in other languages up again and
// answers with the book (POST /api/books/{id}/titles/refresh). The book page's
// "Look up titles" button uses it when a title is missing.
func (s *Server) handleRefreshBookTitles(w http.ResponseWriter, r *http.Request) {
	b, ok := s.bookFromPath(w, r)
	if !ok {
		return
	}
	titles, err := s.refreshBookTitles(r.Context(), b)
	if err != nil {
		writeError(w, http.StatusBadGateway, "The titles in other languages couldn't be looked up. Try again in a moment.")
		return
	}
	b.Titles = titles
	writeJSON(w, http.StatusOK, toBookPayload(b, isAdminRequest(r)))
}
