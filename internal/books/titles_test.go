package books

import "testing"

func TestLanguageFromMARCAndISO(t *testing.T) {
	cases := []struct {
		marc, iso, want string
	}{
		{"jpn", "ja", "Japanese"},
		{"zho", "zh", "Chinese"},
		{"chi", "cn", "Chinese"},
		{"kor", "ko", "Korean"},
		{"ger", "de", "German"},
	}
	for _, tc := range cases {
		if got := LanguageFromMARC(tc.marc); got != tc.want {
			t.Errorf("LanguageFromMARC(%q) = %q, want %q", tc.marc, got, tc.want)
		}
		if got := LanguageFromISO(tc.iso); got != tc.want {
			t.Errorf("LanguageFromISO(%q) = %q, want %q", tc.iso, got, tc.want)
		}
	}
	if got := LanguageFromMARC("xyz"); got != "" {
		t.Errorf("unknown MARC code: %q", got)
	}
	if got := LanguageFromISO(""); got != "" {
		t.Errorf("empty ISO code: %q", got)
	}
}

func TestLanguageWantedAndTitleForLanguage(t *testing.T) {
	wanted := []string{"Chinese", "japanese", "English"}
	if !LanguageWanted(wanted, "Japanese") {
		t.Error("Japanese should be wanted, case-insensitively")
	}
	if LanguageWanted(wanted, "Korean") {
		t.Error("Korean is not wanted")
	}
	if LanguageWanted(wanted, "") {
		t.Error("an empty language is never wanted")
	}
	b := Book{Title: "Norwegian Wood", Titles: []Title{{Lang: "Japanese", Title: "ノルウェイの森"}}}
	if got := b.TitleForLanguage("japanese"); got != "ノルウェイの森" {
		t.Errorf("TitleForLanguage: %q", got)
	}
	if got := b.TitleForLanguage("Korean"); got != "" {
		t.Errorf("TitleForLanguage(unknown): %q", got)
	}
}

func TestMatchTitleTriesOtherLanguages(t *testing.T) {
	b := Book{Title: "Norwegian Wood", Author: "Haruki Murakami", Titles: []Title{
		{Lang: "Japanese", Title: "ノルウェイの森"},
		{Lang: "Chinese", Title: "挪威的森林"},
	}}
	if ti, ok := MatchTitle("Haruki Murakami - Norwegian Wood (1987) [EPUB]", b); !ok || ti.Lang != "" {
		t.Fatalf("the catalogue title should match: %+v %v", ti, ok)
	}
	// A Japanese release with the author, and one without: both match, because
	// the title is in another script than the catalogue's.
	if ti, ok := MatchTitle("村上春樹 - ノルウェイの森 (1987) [EPUB]", b); !ok || ti.Lang != "Japanese" {
		t.Fatalf("the Japanese title with the author: %+v %v", ti, ok)
	}
	if ti, ok := MatchTitle("ノルウェイの森 [EPUB]", b); !ok || ti.Title != "ノルウェイの森" {
		t.Fatalf("the Japanese title without the author: %+v %v", ti, ok)
	}
	if ti, ok := MatchTitle("挪威的森林 [EPUB]", b); !ok || ti.Lang != "Chinese" {
		t.Fatalf("the Chinese title: %+v %v", ti, ok)
	}
	// A Latin title still needs the author, or a stray "Norwegian Wood" in any
	// release name would match.
	if Matches("Norwegian Wood (1987) [EPUB]", b) {
		t.Fatal("a Latin title matched without the author")
	}
	// Another book by the same author is not this one.
	if Matches("村上春樹 - 海辺のカフカ [EPUB]", b) {
		t.Fatal("a different title matched")
	}
}

func TestTitleForRelease(t *testing.T) {
	b := Book{Title: "Norwegian Wood", Author: "Haruki Murakami", Titles: []Title{{Lang: "Japanese", Title: "ノルウェイの森"}}}
	if got := TitleForRelease("ノルウェイの森 [EPUB]", b); got != "ノルウェイの森" {
		t.Errorf("a Japanese release is filed under the Japanese title: %q", got)
	}
	if got := TitleForRelease("Haruki Murakami - Norwegian Wood (1987) [EPUB]", b); got != "Norwegian Wood" {
		t.Errorf("an English release keeps the catalogue title: %q", got)
	}
}
