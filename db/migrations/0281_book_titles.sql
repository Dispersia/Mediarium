-- A book's titles in other languages (#187): the English "Norwegian Wood",
-- the Japanese "ノルウェイの森", the Chinese "挪威的森林". A release of a book
-- translated into another language is named by that language's title, so a
-- search for the catalogue title alone never finds it. The titles are read
-- from Open Library's editions and from Google Books, and kept here.
--
--   lang    the language name Mediarium uses ("Japanese"), for comparing with
--           the languages wanted in Settings > Quality
--   title   the book's title in that language
--   source  where the title came from: "openlibrary", "google" or "manual"
--
-- One row per language; the title a release is filed under comes from here
-- (see books.TitleForRelease).
CREATE TABLE IF NOT EXISTS book_titles (
    book_id INTEGER NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    lang    TEXT    NOT NULL,
    title   TEXT    NOT NULL,
    source  TEXT    NOT NULL DEFAULT '',
    PRIMARY KEY (book_id, lang)
);

-- When a book's titles were last looked up (RFC 3339), so a book with none is
-- not asked about again every time its page is opened.
ALTER TABLE books ADD COLUMN titles_checked_at TEXT NOT NULL DEFAULT '';
