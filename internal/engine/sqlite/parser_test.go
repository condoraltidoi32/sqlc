package sqlite

import (
	"testing"
)

func TestMultibyteCommentsQueryExtraction(t *testing.T) {
	input := `-- name: GetAuthorByBio :one
-- 🔍 検索クエリ: Find author with specific bio / 🚀
/* 多行コメント: multibyte comment test */
SELECT id, name, bio FROM authors WHERE bio = ? LIMIT 1;`

	parser := NewParser()
	stmts, err := parser.ParseString(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(stmts))
	}

	expectedQuery := "SELECT id, name, bio FROM authors WHERE bio = ? LIMIT 1;"
	if stmts[0].Raw != expectedQuery {
		t.Errorf("query truncated or mismatched:\nexpected: %q\ngot:      %q", expectedQuery, stmts[0].Raw)
	}

	if len(stmts[0].Comments) != 3 {
		t.Errorf("expected 3 comments, got %d", len(stmts[0].Comments))
	}
}

func TestMultibyteRuneToByteOffsetConversion(t *testing.T) {
	src := "-- 🚀 emoji comment\nSELECT * FROM test;"
	offsets := BuildRuneToByteOffsets(src)

	if len(offsets) == 0 {
		t.Fatal("expected non-empty offsets")
	}

	maxByteLen := len(src)
	lastByteOffset := RuneToByteOffset(offsets, len(offsets)-1, maxByteLen)
	if lastByteOffset != maxByteLen {
		t.Errorf("expected last byte offset %d, got %d", maxByteLen, lastByteOffset)
	}
}

func TestMultipleQueriesWithMultibyteComments(t *testing.T) {
	input := `-- name: ListAuthors :many
-- 📝 Список авторов (Cyrillic test: абвгдеёжзийклмноп)
SELECT id, name FROM authors;

-- name: GetAuthor :one
/* 🌟 Äpfel, Über, Ça plane pour moi (Accented Latin) 🌟 */
SELECT id, name, bio FROM authors WHERE id = ?;`

	parser := NewParser()
	stmts, err := parser.ParseString(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(stmts) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(stmts))
	}

	exp1 := "SELECT id, name FROM authors;"
	if stmts[0].Raw != exp1 {
		t.Errorf("statement 1 truncated or mismatched:\nexpected: %q\ngot:      %q", exp1, stmts[0].Raw)
	}

	exp2 := "SELECT id, name, bio FROM authors WHERE id = ?;"
	if stmts[1].Raw != exp2 {
		t.Errorf("statement 2 truncated or mismatched:\nexpected: %q\ngot:      %q", exp2, stmts[1].Raw)
	}
}
