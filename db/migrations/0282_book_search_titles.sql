-- The library search also finds a book by its titles in other languages: the
-- Japanese "ノルウェイの森" for the English "Norwegian Wood". The books triggers
-- from migration 0250 set the search row's alt text to the author alone; here
-- they add the other-language titles (book_titles, migration 0281) as well,
-- and a title added, changed or removed moves the book's row with it.
DROP TRIGGER IF EXISTS library_search_books_insert;
DROP TRIGGER IF EXISTS library_search_books_update;

CREATE TRIGGER library_search_books_insert AFTER INSERT ON books
BEGIN
    INSERT INTO library_search(rowid, title, alt)
    SELECT NEW.id * 8 + 3, NEW.title,
        trim(coalesce(NEW.author, '') || ' ' || coalesce((SELECT group_concat(title, ' ') FROM book_titles WHERE book_id = NEW.id), ''));
END;

CREATE TRIGGER library_search_books_update AFTER UPDATE OF title, author ON books
BEGIN
    DELETE FROM library_search WHERE rowid = OLD.id * 8 + 3;
    INSERT INTO library_search(rowid, title, alt)
    SELECT NEW.id * 8 + 3, NEW.title,
        trim(coalesce(NEW.author, '') || ' ' || coalesce((SELECT group_concat(title, ' ') FROM book_titles WHERE book_id = NEW.id), ''));
END;

CREATE TRIGGER IF NOT EXISTS library_search_book_titles_insert AFTER INSERT ON book_titles
BEGIN
    DELETE FROM library_search WHERE rowid = NEW.book_id * 8 + 3;
    INSERT INTO library_search(rowid, title, alt)
    SELECT b.id * 8 + 3, b.title,
        trim(coalesce(b.author, '') || ' ' || coalesce((SELECT group_concat(title, ' ') FROM book_titles WHERE book_id = b.id), ''))
    FROM books b WHERE b.id = NEW.book_id;
END;

CREATE TRIGGER IF NOT EXISTS library_search_book_titles_update AFTER UPDATE ON book_titles
BEGIN
    DELETE FROM library_search WHERE rowid = NEW.book_id * 8 + 3;
    INSERT INTO library_search(rowid, title, alt)
    SELECT b.id * 8 + 3, b.title,
        trim(coalesce(b.author, '') || ' ' || coalesce((SELECT group_concat(title, ' ') FROM book_titles WHERE book_id = b.id), ''))
    FROM books b WHERE b.id = NEW.book_id;
END;

CREATE TRIGGER IF NOT EXISTS library_search_book_titles_delete AFTER DELETE ON book_titles
BEGIN
    DELETE FROM library_search WHERE rowid = OLD.book_id * 8 + 3;
    INSERT INTO library_search(rowid, title, alt)
    SELECT b.id * 8 + 3, b.title,
        trim(coalesce(b.author, '') || ' ' || coalesce((SELECT group_concat(title, ' ') FROM book_titles WHERE book_id = b.id), ''))
    FROM books b WHERE b.id = OLD.book_id;
END;
