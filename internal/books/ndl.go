package books

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"
)

// NDL Search (ndlsearch.ndl.go.jp) is the National Diet Library's catalogue of
// Japanese books, the authoritative one for Japanese editions. Mediarium asks
// it for a book to find the title its Japanese edition is known by. It is an
// SRU (searchRetrieve over URL) service answering XML.
//
// A Japanese edition is filed under its Japanese title, which a release is
// then named by. NDL records a translation under that title and often names
// the original one in a note; only a record that names the book is used, so a
// record that is not about the book is never taken.

const ndlSRUBase = "https://ndlsearch.ndl.go.jp/api/sru"

// NDL talks to the NDL Search API.
type NDL struct {
	Base      string // https://ndlsearch.ndl.go.jp/api/sru; tests point it elsewhere
	UserAgent string
	HTTP      *http.Client
}

// NewNDL returns a client for the real NDL Search.
func NewNDL(userAgent string) *NDL {
	return &NDL{Base: ndlSRUBase, UserAgent: userAgent, HTTP: &http.Client{Timeout: 20 * time.Second}}
}

// ndlRecord is one record's Dublin Core fields, flattened for the lookup.
type ndlRecord struct {
	titles       []string
	creators     []string
	descriptions []string
	publishers   []string
	languages    []string
}

// text is everything the record says, for judging whether it is about a book.
func (r ndlRecord) text() string {
	all := make([]string, 0, len(r.titles)+len(r.creators)+len(r.descriptions)+len(r.publishers)+len(r.languages))
	all = append(all, r.titles...)
	all = append(all, r.creators...)
	all = append(all, r.descriptions...)
	all = append(all, r.publishers...)
	all = append(all, r.languages...)
	return strings.Join(all, " ")
}

// Titles asks NDL for a book's Japanese titles: records that name the searched
// title (or the author) and whose own title is written in Japanese. It returns
// none when NDL does not name the book, which is common for a translation whose
// record does not carry the original title.
func (n *NDL) Titles(ctx context.Context, title, author string) ([]Title, error) {
	q := strings.TrimSpace(title)
	if len([]rune(q)) < 2 {
		return nil, nil
	}
	records, err := n.search(ctx, `title="`+cqlEscape(q)+`"`, 20)
	if err != nil {
		return nil, err
	}
	var out []Title
	seen := map[string]bool{}
	for _, rec := range records {
		jp := japaneseTitle(firstOf(rec.titles))
		if jp == "" || seen[jp] || !recordNamesBook(rec, title, author) {
			continue
		}
		seen[jp] = true
		out = append(out, Title{Lang: "Japanese", Title: jp, Source: "ndl"})
	}
	return out, nil
}

// search runs one SRU title search and returns the parsed records.
func (n *NDL) search(ctx context.Context, cql string, limit int) ([]ndlRecord, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	base := strings.TrimRight(n.Base, "/")
	endpoint := base + "?" + url.Values{
		"operation":      {"searchRetrieve"},
		"version":        {"1.2"},
		"query":          {cql},
		"maximumRecords": {fmt.Sprint(limit)},
	}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	if n.UserAgent != "" {
		req.Header.Set("User-Agent", n.UserAgent)
	}
	client := n.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("ndl: %w", err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusTooManyRequests, http.StatusServiceUnavailable, http.StatusBadGateway, http.StatusGatewayTimeout:
		return nil, busy(fmt.Sprintf("NDL Search is busy or asks for fewer requests (it answered %d). Try again in a moment.", resp.StatusCode))
	default:
		return nil, fmt.Errorf("ndl answered %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("ndl: read the answer: %w", err)
	}
	return parseNDL(body)
}

type ndlSRUResponse struct {
	Records []struct {
		Data string `xml:"recordData"`
	} `xml:"records>record"`
	Diagnostics []struct {
		Message string `xml:"message"`
	} `xml:"diagnostics>diagnostic"`
}

type ndlDC struct {
	Titles       []string `xml:"title"`
	Creators     []string `xml:"creator"`
	Descriptions []string `xml:"description"`
	Publishers   []string `xml:"publisher"`
	Languages    []string `xml:"language"`
}

// parseNDL reads the SRU answer: each recordData holds the record's Dublin Core
// XML as escaped text, so it is read a second time.
func parseNDL(body []byte) ([]ndlRecord, error) {
	var sru ndlSRUResponse
	if err := xml.Unmarshal(body, &sru); err != nil {
		return nil, fmt.Errorf("ndl: read the answer: %w", err)
	}
	if len(sru.Diagnostics) > 0 && len(sru.Records) == 0 {
		return nil, fmt.Errorf("ndl refused the search: %s", strings.TrimSpace(sru.Diagnostics[0].Message))
	}
	out := make([]ndlRecord, 0, len(sru.Records))
	for _, rec := range sru.Records {
		var dc ndlDC
		if err := xml.Unmarshal([]byte(rec.Data), &dc); err != nil {
			continue // a record in a shape we do not read is skipped
		}
		out = append(out, ndlRecord{titles: dc.Titles, creators: dc.Creators, descriptions: dc.Descriptions, publishers: dc.Publishers, languages: dc.Languages})
	}
	return out, nil
}

// cqlEscape makes a title safe inside a CQL double-quoted string.
func cqlEscape(s string) string {
	return strings.TrimSpace(strings.NewReplacer(`"`, " ", `\`, " ", "\n", " ").Replace(s))
}

// firstOf is the first non-empty string.
func firstOf(list []string) string {
	for _, s := range list {
		if s = strings.TrimSpace(s); s != "" {
			return s
		}
	}
	return ""
}

// HasJapaneseScript reports whether s holds Japanese script (kana or kanji).
func HasJapaneseScript(s string) bool {
	for _, r := range s {
		if unicode.In(r, unicode.Hiragana, unicode.Katakana, unicode.Han) {
			return true
		}
	}
	return false
}

// japaneseTitle is the Japanese title an NDL record is filed under, or "". A
// leading bracketed qualifier ("[インドネシア語]", "【大活字版】") is dropped, and
// the rest has to hold Japanese script: a record titled only in another
// language or transliteration is not a Japanese edition.
func japaneseTitle(title string) string {
	title = strings.TrimSpace(title)
	for {
		if strings.HasPrefix(title, "[") {
			if end := strings.Index(title, "]"); end >= 0 {
				title = strings.TrimSpace(title[end+1:])
				continue
			}
		}
		if strings.HasPrefix(title, "【") {
			if end := strings.Index(title, "】"); end >= 0 {
				title = strings.TrimSpace(title[end+len("】"):])
				continue
			}
		}
		break
	}
	title = strings.TrimSpace(strings.TrimRight(title, " :/／-"))
	if title == "" {
		return ""
	}
	hasJapanese, startsLatin := false, false
	for i, r := range title {
		switch {
		case unicode.In(r, unicode.Hiragana, unicode.Katakana, unicode.Han):
			hasJapanese = true
		case i == 0 && unicode.Is(unicode.Latin, r):
			startsLatin = true
		}
	}
	if !hasJapanese || startsLatin {
		return ""
	}
	return title
}

// recordNamesBook reports whether the record is about the book: it names the
// searched title, or its author. A Japanese record for a translation may name
// neither, and is then left alone rather than taken for another book.
func recordNamesBook(rec ndlRecord, title, author string) bool {
	text := strings.ToLower(rec.text())
	if t := strings.ToLower(strings.TrimSpace(title)); t != "" && strings.Contains(text, t) {
		return true
	}
	if a := words(author); len(a) > 0 && strings.Contains(text, a[len(a)-1]) {
		return true
	}
	return false
}
