package books

import (
	"strings"
	"unicode"
)

// A book's titles in other languages. A release of a book that was translated
// is named by that language's title (the Japanese "ノルウェイの森" for the English
// "Norwegian Wood"), so the book is searched for - and a release matched and
// filed - under every title known for it. The titles come from Open Library's
// editions and from Google Books (see the api package); this package only
// keeps them and uses them.

// Title is one of a book's titles in a language.
type Title struct {
	Lang   string // language name, e.g. "Japanese"; "" is never stored
	Title  string // the book's title in that language
	Source string // "openlibrary", "google" or "manual"
}

// TitleForLanguage returns the book's title in lang ("" when there is none).
func (b Book) TitleForLanguage(lang string) string {
	for _, t := range b.Titles {
		if strings.EqualFold(t.Lang, lang) && strings.TrimSpace(t.Title) != "" {
			return t.Title
		}
	}
	return ""
}

// LanguageWanted reports whether lang is one of the wanted language names (the
// list from Settings > Quality, best first, e.g. "Chinese,Japanese,English").
func LanguageWanted(wanted []string, lang string) bool {
	lang = strings.TrimSpace(lang)
	if lang == "" {
		return false
	}
	for _, w := range wanted {
		if strings.EqualFold(strings.TrimSpace(w), lang) {
			return true
		}
	}
	return false
}

// LanguageFromScript is the language a title written in its own script is in,
// as release names often are: "Japanese" for kana (or kana with kanji),
// "Chinese" for Han on its own and "Korean" for Hangul. "" for a title in the
// Latin alphabet or a script it does not recognise.
//
// A Japanese title made only of kanji cannot be told from a Chinese one, so it
// counts as Chinese here. When the book's own title in another language is
// known, that language wins over this guess (see MatchTitle).
func LanguageFromScript(s string) string {
	kana, han, hangul := false, false, false
	for _, r := range s {
		switch {
		case unicode.In(r, unicode.Hiragana, unicode.Katakana):
			kana = true
		case unicode.Is(unicode.Han, r):
			han = true
		case unicode.Is(unicode.Hangul, r):
			hangul = true
		}
	}
	switch {
	case kana:
		return "Japanese"
	case hangul:
		return "Korean"
	case han:
		return "Chinese"
	}
	return ""
}

// openLibraryLanguage maps the MARC language code an Open Library edition
// carries ("/languages/jpn") to the name Mediarium uses. Only the languages a
// release can be wanted in are listed; an edition in another one is left out.
var openLibraryLanguage = map[string]string{
	"eng": "English", "fre": "French", "fra": "French", "ger": "German", "deu": "German",
	"spa": "Spanish", "ita": "Italian", "jpn": "Japanese", "chi": "Chinese", "zho": "Chinese",
	"kor": "Korean", "rus": "Russian", "por": "Portuguese", "dut": "Dutch", "nld": "Dutch",
	"pol": "Polish", "swe": "Swedish", "dan": "Danish", "nor": "Norwegian", "nob": "Norwegian",
	"fin": "Finnish", "tur": "Turkish", "gre": "Greek", "ell": "Greek", "hun": "Hungarian",
	"cze": "Czech", "ces": "Czech", "ara": "Arabic", "heb": "Hebrew", "hin": "Hindi",
	"ukr": "Ukrainian", "vie": "Vietnamese", "tha": "Thai",
}

// LanguageFromMARC is the language name for a MARC code ("jpn" is "Japanese"),
// or "" when it is not one Mediarium knows.
func LanguageFromMARC(code string) string {
	return openLibraryLanguage[strings.ToLower(strings.TrimSpace(code))]
}

// isoLanguage maps a two- or three-letter ISO 639 code (what Google Books'
// volumeInfo.language holds) to the language name Mediarium uses, for the few
// codes MARC does not cover.
var isoLanguage = map[string]string{
	"en": "English", "eng": "English", "fr": "French", "fre": "French", "fra": "French",
	"de": "German", "ger": "German", "deu": "German", "es": "Spanish", "spa": "Spanish",
	"it": "Italian", "ita": "Italian", "ja": "Japanese", "jpn": "Japanese", "zh": "Chinese",
	"cn": "Chinese", "chi": "Chinese", "zho": "Chinese", "ko": "Korean", "kor": "Korean",
	"ru": "Russian", "rus": "Russian", "pt": "Portuguese", "por": "Portuguese",
	"nl": "Dutch", "dut": "Dutch", "nld": "Dutch", "pl": "Polish", "pol": "Polish",
	"sv": "Swedish", "swe": "Swedish", "da": "Danish", "dan": "Danish", "no": "Norwegian",
	"nb": "Norwegian", "nor": "Norwegian", "fi": "Finnish", "fin": "Finnish",
	"tr": "Turkish", "tur": "Turkish", "el": "Greek", "gre": "Greek", "ell": "Greek",
	"hu": "Hungarian", "hun": "Hungarian", "cs": "Czech", "cze": "Czech", "ces": "Czech",
	"ar": "Arabic", "ara": "Arabic", "he": "Hebrew", "heb": "Hebrew", "hi": "Hindi",
	"hin": "Hindi", "uk": "Ukrainian", "ukr": "Ukrainian", "vi": "Vietnamese",
	"vie": "Vietnamese", "th": "Thai", "tha": "Thai",
}

// LanguageFromISO is the language name for an ISO 639 code ("ja" is
// "Japanese"), or "" when it is not one Mediarium knows.
func LanguageFromISO(code string) string {
	return isoLanguage[strings.ToLower(strings.TrimSpace(code))]
}
