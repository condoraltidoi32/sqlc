# Solution for Issue #1

## 🛠️ Proposed Solution (by Aditya Waghamare)

### Analysis
The SQLite parser/tokenizer in `sqlc` (`internal/engine/sqlite`) tracks AST node positions (`Pos` / `End` or `From` / `To` location offsets) using 0-based Unicode code point (rune) counts instead of byte offsets. However, string slicing in Go (`source[start:end]`) and underlying byte buffers operate on byte indices. 

When a SQL query contains multibyte UTF-8 characters in comments (such as CJK characters = 3 bytes, emojis = 4 bytes, or accented Latin = 2 bytes), the rune count up to a given offset is strictly less than the byte count ($runes < bytes$). Slicing the raw source string using rune indices directly as byte indices causes the start and end offsets to shift left by $N = \text{bytes} - \text{runes}$. As a result, the extracted SQL query string in generated Go constants is truncated at the end by $N$ bytes, causing syntax errors at runtime.

---

### Fix
1. **Rune-to-Byte Offset Conversion**: Introduce a helper utility function `RuneToByteOffset(s string, runeOffset int) int` that converts rune-based token positions into exact UTF-8 byte offsets.
2. **SQLite AST Range Normalization**: Before slicing the raw SQL source string in `internal/engine/sqlite/parser.go` and `internal/compiler/`, convert the AST statement node positions (`node.Pos` / `node.End`) from rune offsets to byte offsets.
3. **Preserve Comment Alignments**: Ensure comment line stripping (`-- name: ...`) accurately handles multibyte rune lengths when extracting annotations and computing the remaining query boundaries.

---

### Implementation

#### 1. Offset Utility (`internal/engine/sqlite/utils.go`)
```go
package sqlite

import (
	"unicode/utf8"
)

// RuneToByteOffset converts a 0-based rune index in UTF-8 string s to its corresponding byte index.
func RuneToByteOffset(s string, runeOffset int) int {
	if runeOffset <= 0 {
		return 0
	}
	byteOffset := 0
	currentRune := 0
	for byteOffset < len(s) && currentRune < runeOffset {
		_, size := utf8.DecodeRuneInString(s[byteOffset:])
		byteOffset += size
		currentRune++
	}
	if byteOffset > len(s) {
		return len(s)
	}
	return byteOffset
}

// ByteToRuneOffset converts a 0-based byte index in UTF-8 string s to its corresponding rune index.
func ByteToRuneOffset(s string, byteOffset int) int {
	if byteOffset <= 0 {
		return 0
	}
	if byteOffset > len(s) {
		byteOffset = len(s)
	}
	return utf8.RuneCountInString(s[:byteOffset])
}
```

#### 2. SQLite AST Position Fix (`internal/engine/sqlite/parser.go`)
```go
package sqlite

import (
	"github.com/sqlc-dev/sqlc/internal/sql/ast"
)

// ExtractStatementText extracts the raw query string from source text using AST node ranges adjusted for multibyte UTF-8 characters.
func ExtractStatementText(source string, node *ast.RawStmt) string {
	if node == nil {
		return ""
	}

	// node.StmtLocation and node.StmtLen are reported in rune offsets by the parser
	startRune := node.StmtLocation
	endRune := node.StmtLocation + node.StmtLen

	// Convert rune offsets to exact byte offsets for string slicing
	startByte := RuneToByteOffset(source, startRune)
	endByte := RuneToByteOffset(source, endRune)

	if startByte < 0 {
		startByte = 0
	}
	if endByte > len(source) {
		endByte = len(source)
	}
	if startByte >= endByte {
		return ""
	}

	return source[startByte:endByte]
}
```

#### 3. Compiler Integration (`internal/compiler/parse.go`)
```go
// Normalizing SQL query text extraction across engines
func SliceSourceByRuneRange(source string, startRune, endRune int) string {
	startByte := sqlite.RuneToByteOffset(source, startRune)
	endByte := sqlite.RuneToByteOffset(source, endRune)
	return source[startByte:endByte]
}
```

---

### Testing

1. **Unit Test (`internal/engine/sqlite/parser_test.go`)**:
```go
package sqlite_test

import (
	"testing"
	"github.com/sqlc-dev/sqlc/internal/engine/sqlite"
)

func TestMultibyteCommentQueryExtraction(t *testing.T) {
	tests := []struct {
		name     string
		sql      string
		expected string
	}{
		{
			name:     "CJK comments in single line",
			sql:      "-- ユーザー情報を取得する\nSELECT id, name FROM users WHERE id = ?;",
			expected: "SELECT id, name FROM users WHERE id = ?;",
		},
		{
			name:     "Emoji in block comment",
			sql:      "/* 🔥 Fetch active orders 🚀 */\nSELECT * FROM orders WHERE status = 'active';",
			expected: "SELECT * FROM orders WHERE status = 'active';",
		},
		{
			name:     "Mixed Cyrillic and Accented characters",
			sql:      "-- Получить данные über uns\nSELECT * FROM company;",
			expected: "SELECT * FROM company;",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extracted := sqlite.ExtractQuery(tt.sql)
			if extracted != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, extracted)
			}
		})
	}
}
```

2. **Verification Steps**:
   - Run `go test ./internal/engine/sqlite/... ./internal/compiler/...` to ensure all existing tests pass and multibyte UTF-8 query extraction tests pass.
   - Run end-to-end codegen tests with `sqlc generate` on schemas containing 2-byte, 3-byte, and 4-byte UTF-8 sequences in leading, block, and trailing comments.

---
*Submitted by Aditya Waghamare*
💰 **Payout Address (Base L2 / EVM):** `0xb61dBcdBc3407F71EaCb64D4CBFAcf9FFfe2415C`