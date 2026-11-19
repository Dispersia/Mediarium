package books

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The NDL SRU answer, as NDL Search sends it: each recordData holds the
// record's Dublin Core XML as escaped text.
const ndlFixture = `<?xml version="1.0" encoding="UTF-8"?>
<searchRetrieveResponse xmlns="http://www.loc.gov/zing/srw/">
  <version>1.2</version>
  <numberOfRecords>2</numberOfRecords>
  <records>
    <record>
      <recordSchema>info:srw/schema/1/dc-v1.1</recordSchema>
      <recordPacking>string</recordPacking>
      <recordData>
      &lt;srw_dc:dc xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:srw_dc="info:srw/schema/1/dc-v1.1"&gt;
          &lt;dc:title&gt;[インドネシア語]Norwegian wood :&lt;/dc:title&gt;
          &lt;dc:creator&gt;HarukiMurakami.&lt;/dc:creator&gt;
          &lt;dc:description&gt;本文はインドネシア語. 「ノルウェイの森」村上春樹著のインドネシア語版&lt;/dc:description&gt;
      &lt;/srw_dc:dc&gt;
    </recordData>
      <recordPosition>1</recordPosition>
    </record>
    <record>
      <recordSchema>info:srw/schema/1/dc-v1.1</recordSchema>
      <recordPacking>string</recordPacking>
      <recordData>
      &lt;srw_dc:dc xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:srw_dc="info:srw/schema/1/dc-v1.1"&gt;
          &lt;dc:title&gt;ノルウェイの森&lt;/dc:title&gt;
          &lt;dc:creator&gt;村上春樹&lt;/dc:creator&gt;
          &lt;dc:description&gt;Norwegian Wood の翻訳&lt;/dc:description&gt;
          &lt;dc:language&gt;jpn&lt;/dc:language&gt;
      &lt;/srw_dc:dc&gt;
    </recordData>
      <recordPosition>2</recordPosition>
    </record>
  </records>
</searchRetrieveResponse>`

func TestNDLTitles(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query().Get("query")
		if r.URL.Query().Get("operation") != "searchRetrieve" {
			t.Errorf("operation: %q", r.URL.Query().Get("operation"))
		}
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(ndlFixture))
	}))
	defer srv.Close()
	n := NewNDL("test")
	n.Base = srv.URL
	titles, err := n.Titles(context.Background(), "Norwegian Wood", "Haruki Murakami")
	if err != nil {
		t.Fatal(err)
	}
	if gotQuery != `title="Norwegian Wood"` {
		t.Errorf("query: %q", gotQuery)
	}
	// The Indonesian edition is skipped (its title is not Japanese); the
	// Japanese edition is kept.
	if len(titles) != 1 || titles[0].Lang != "Japanese" || titles[0].Title != "ノルウェイの森" || titles[0].Source != "ndl" {
		t.Fatalf("titles: %+v", titles)
	}
}

func TestNDLOnlyTakesRecordsThatNameTheBook(t *testing.T) {
	rec := ndlRecord{titles: []string{"ノルウェイの森"}, creators: []string{"村上春樹"}}
	if !recordNamesBook(rec, "ノルウェイの森", "") {
		t.Error("the title should tie the record to the book")
	}
	if !recordNamesBook(rec, "", "村上春樹") {
		t.Error("the author should tie the record to the book")
	}
	// "村上春樹" does not match "Murakami", and this record does not name the
	// English title, so it is left alone rather than taken for another book.
	if recordNamesBook(rec, "Norwegian Wood", "Haruki Murakami") {
		t.Error("a record that names neither the title nor the author must not be taken")
	}
}

func TestJapaneseTitle(t *testing.T) {
	tests := []struct{ in, want string }{
		{"ノルウェイの森", "ノルウェイの森"},
		{"【大活字版】ノルウェイの森", "ノルウェイの森"},
		{"[インドネシア語]Norwegian wood :", ""},
		{"Norwegian Wood", ""},
		{"", ""},
	}
	for _, tc := range tests {
		if got := japaneseTitle(tc.in); got != tc.want {
			t.Errorf("japaneseTitle(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
	if !HasJapaneseScript("ノルウェイの森") || HasJapaneseScript("Norwegian Wood") {
		t.Error("HasJapaneseScript wrongly answers")
	}
}
