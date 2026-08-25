package sqlite

import (
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"unicode/utf8"
)

// Statement represents an extracted SQL statement with its raw source text and metadata.
type Statement struct {
	Raw      string
	Comments []string
	Pos      int // Byte offset start in raw source
	End      int // Byte offset end in raw source
}

// Parser parses SQLite SQL source code into statements while preserving exact byte offsets.
type Parser struct{}

// NewParser creates a new SQLite Parser instance.
func NewParser() *Parser {
	return &Parser{}
}

// BuildRuneToByteOffsets creates a slice mapping rune index -> byte offset in source string s.
func BuildRuneToByteOffsets(s string) []int {
	runeCount := utf8.RuneCountInString(s)
	offsets := make([]int, runeCount+1)
	rIdx := 0
	for byteOffset := range s {
		offsets[rIdx] = byteOffset
		rIdx++
	}
	offsets[runeCount] = len(s)
	return offsets
}

// RuneToByteOffset converts a rune offset into a byte offset using precalculated offsets table.
func RuneToByteOffset(offsets []int, runeOffset int, maxByteLen int) int {
	if runeOffset < 0 {
		return 0
	}
	if runeOffset >= len(offsets) {
		return maxByteLen
	}
	return offsets[runeOffset]
}

// ParseString parses the SQL string into individual statements with accurate byte positions.
func (p *Parser) ParseString(sql string) ([]*Statement, error) {
	var stmts []*Statement
	n := len(sql)
	i := 0

	for i < n {
		// Skip leading whitespace
		for i < n && (sql[i] == ' ' || sql[i] == '\t' || sql[i] == '\n' || sql[i] == '\r') {
			i++
		}
		if i >= n {
			break
		}

		var comments []string
		stmtStart := i

		// Collect leading comments before the query statement
		for i < n {
			if i+1 < n && sql[i] == '-' && sql[i+1] == '-' {
				commentStart := i
				i += 2
				for i < n && sql[i] != '\n' {
					i++
				}
				comments = append(comments, sql[commentStart:i])
				for i < n && (sql[i] == ' ' || sql[i] == '\t' || sql[i] == '\n' || sql[i] == '\r') {
					i++
				}
				continue
			}
			if i+1 < n && sql[i] == '/' && sql[i+1] == '*' {
				commentStart := i
				i += 2
				for i+1 < n && !(sql[i] == '*' && sql[i+1] == '/') {
					i++
				}
				if i+1 < n {
					i += 2
				} else {
					i = n
				}
				comments = append(comments, sql[commentStart:i])
				for i < n && (sql[i] == ' ' || sql[i] == '\t' || sql[i] == '\n' || sql[i] == '\r') {
					i++
				}
				continue
			}
			break
		}

		if i >= n {
			break
		}

		queryStart := i
		inSingleQuote := false
		inDoubleQuote := false
		inBacktick := false
		stmtEnd := n

		for i < n {
			b := sql[i]
			if inSingleQuote {
				if b == '\'' {
					if i+1 < n && sql[i+1] == '\'' {
						i += 2
						continue
					}
					inSingleQuote = false
				}
				i++
				continue
			}
			if inDoubleQuote {
				if b == '"' {
					if i+1 < n && sql[i+1] == '"' {
						i += 2
						continue
					}
					inDoubleQuote = false
				}
				i++
				continue
			}
			if inBacktick {
				if b == '`' {
					inBacktick = false
				}
				i++
				continue
			}

			// Check string delimiters
			if b == '\'' {
				inSingleQuote = true
				i++
				continue
			}
			if b == '"' {
				inDoubleQuote = true
				i++
				continue
			}
			if b == '`' {
				inBacktick = true
				i++
				continue
			}

			// Check comments inside statement
			if i+1 < n && b == '-' && sql[i+1] == '-' {
				i += 2
				for i < n && sql[i] != '\n' {
					i++
				}
				continue
			}
			if i+1 < n && b == '/' && sql[i+1] == '*' {
				i += 2
				for i+1 < n && !(sql[i] == '*' && sql[i+1] == '/') {
					i++
				}
				if i+1 < n {
					i += 2
				} else {
					i = n
				}
				continue
			}

			// Statement termination on semicolon
			if b == ';' {
				i++
				stmtEnd = i
				break
			}

			// Handle multi-byte UTF-8 character advancement
			_, width := utf8.DecodeRuneInString(sql[i:])
			if width > 0 {
				i += width
			} else {
				i++
			}
		}

		if stmtEnd > n {
			stmtEnd = n
		}

		queryRaw := strings.TrimSpace(sql[queryStart:stmtEnd])
		if queryRaw != "" {
			stmts = append(stmts, &Statement{
				Raw:      queryRaw,
				Comments: comments,
				Pos:      stmtStart,
				End:      stmtEnd,
			})
		}
	}

	return stmts, nil
}

// Parse parses SQL from an io.Reader.
func (p *Parser) Parse(r io.Reader) ([]*Statement, error) {
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read sql source: %w", err)
	}
	return p.ParseString(string(b))
}
