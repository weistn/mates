package main

import (
	"strings"
	"unicode/utf8"
)

// Token describes the kind of token recognized by the scanner.
type Token int

const (
	// TokenEOF denotes the end of file.
	TokenEOF Token = iota
	// TokenText denotes plain text
	TokenText
	// TokenSection denotes a tag
	TokenSection
	// TokenValue is a key:value string that can follow a section token
	TokenValue
	// TokenEnum denotes an enumeration tag
	TokenEnum
	// TokenStyle denotes a CSS style directive
	TokenStyle
	// TokenEntity denotes an entity
	TokenEntity
	// TokenCodeText denotes text that represents code
	TokenCodeText
	// TokenMathText denotes text that represents math markup
	TokenMathText
	// TokenConfigText denotes text that represents a configuration, e.g. YAML
	TokenConfigText
	// TokenTableCell denotes text that represents a table cell
	TokenTableCell
	// TokenTableRow denotes text that represents a table row
	TokenTableRow

//	TokenSetvar
//	TokenVar
)

// scannerMode defines the state of the scanner.
type scannerMode int

const (
	modeNormal scannerMode = iota
	// modeSection means that the scanner saw a tag (TokenSection) and is not expecting either
	// TokenValue or text.
	modeSection
	// modeNewTag means that the scanner is expecting a new tag. If it sees text or style or entities instead,
	// it assumes a new paragraph.
	modeNewTag
	modeStyle
	modeEmbed
	modeInjectSection
	modeConfig
)

// textMode defines how the scanner handles lines with text.
type textMode int

const (
	textNormal textMode = iota
	textTable
	textCode
	textMath
	textParagraph
)

// String denoting start and end of the frontmatter.
var yamlSeparator = []byte("---")

// ScannerError reports a lexicographic error,
// which is either an illegal character or a malformed UTF-8 encoding.
type ScannerError struct {
	Pos  ScannerRange
	Text string
}

// ScannerRange describes a position in the input markdown.
type ScannerRange struct {
	FromLine    int
	FromLinePos int // Measured in bytes
	From        int // Measured in bytes
	To          int // Measured in bytes
}

// Scanner splits markdown into tokens.
type Scanner struct {
	// source

	src      []byte // Source markdown
	mode     scannerMode
	nextMode scannerMode
	textMode textMode

	// scanning state

	ch         rune // current character
	offset     int  // Offset of `ch` in `src` (position of the current character)
	readOffset int  // reading offset (position after current character)
	lineOffset int  // start of the current line in `src`.
	lineCount  int
	indent     int // The indentation level of the last tag

	// Errors denotes the lexicographical errors detected while scanning.
	Errors []ScannerError
}

// Error returns a textual description of the error.
func (err ScannerError) Error() string {
	return err.Text
}

// NewScanner returns a new scanner that tokenizes the markdown text
func NewScanner(markdown []byte) *Scanner {
	return &Scanner{src: markdown, mode: modeNewTag, nextMode: modeNormal}
}

// Read the next Unicode char into scanner.ch.
// scanner.ch < 0 means end-of-file.
func (scanner *Scanner) next() {
	if scanner.readOffset < len(scanner.src) {
		scanner.offset = scanner.readOffset
		if scanner.ch == '\n' {
			scanner.lineOffset = scanner.offset
			scanner.lineCount++
		}
		r, w := rune(scanner.src[scanner.readOffset]), 1
		switch {
		case r == 0:
			scanner.error(scanner.lineCount, scanner.offset-scanner.lineOffset, scanner.offset, "illegal character NUL")
		case r >= 0x80:
			// not ASCII
			r, w = utf8.DecodeRune(scanner.src[scanner.readOffset:])
			if r == utf8.RuneError && w == 1 {
				scanner.error(scanner.lineCount, scanner.offset-scanner.lineOffset, scanner.offset, "illegal UTF-8 encoding")
			}
		}
		scanner.readOffset += w
		scanner.ch = r
	} else {
		scanner.offset = len(scanner.src)
		if scanner.ch == '\n' {
			scanner.lineOffset = scanner.offset
			scanner.lineCount++
		}
		scanner.ch = -1 // eof
	}
}

func (scanner *Scanner) skipWhitespace(newline bool) {
	for scanner.ch == ' ' || scanner.ch == '\t' || (newline && scanner.ch == '\n') || scanner.ch == '\r' {
		scanner.next()
	}
}

// isStartOfLine returns true if the current character `scanner.ch`
// is the first non-space character in the line.
func (scanner *Scanner) isStartOfLine() bool {
	if scanner.lineOffset == scanner.offset {
		return true
	}
	for i := scanner.lineCount; i < scanner.offset; i++ {
		ch := scanner.src[i]
		if ch != ' ' && ch != '\t' && ch != '\r' {
			return false
		}
	}
	return true
}

func (scanner *Scanner) skipUntilEmptyLine() {
	for ; scanner.ch != -1; scanner.next() {
		if scanner.ch == '\n' {
			scanner.next()
			scanner.skipWhitespace(false)
			if scanner.ch == '\n' {
				scanner.next()
				break
			}
		}
	}
}

func (scanner *Scanner) skipString(bstr []byte) bool {
	if scanner.offset+len(bstr) > len(scanner.src) {
		return false
	}
	for i := 0; i < len(bstr); i++ {
		if bstr[i] != scanner.src[scanner.offset+i] {
			return false
		}
	}
	for i := 0; i < len(bstr); i++ {
		scanner.next()
	}
	return true
}

func (scanner *Scanner) error(line, linepos int, offset int, text string) {
	scanner.Errors = append(scanner.Errors, ScannerError{ScannerRange{line, linepos, offset, offset}, text})
}

// Scan returns the next token.
func (scanner *Scanner) Scan() (ScannerRange, Token, string) {
	var r ScannerRange
	r.From = scanner.offset
	r.FromLine = scanner.lineCount
	r.FromLinePos = scanner.offset - scanner.lineOffset
	t, s := scanner.scan()
	r.To = scanner.offset
	return r, t, s
}

func (scanner *Scanner) scan() (Token, string) {
	// When in Code-Layout, scan until EOF or a '#' at the beginning of a line
	if scanner.textMode == textCode && (scanner.mode == modeNormal || scanner.mode == modeNewTag) {
		start := scanner.offset
		scanner.skipUntilEmptyLine()
		// Scan until a line starts with the same or less indent as the previous tag
		for ; scanner.ch != -1 && ((scanner.offset-scanner.lineOffset > scanner.indent) || scanner.ch == ' ' || scanner.ch == '\t' || scanner.ch == '\n' || scanner.ch == '\r'); scanner.next() {
		}
		scanner.textMode = textNormal
		scanner.mode = modeNewTag
		return TokenCodeText, string(scanner.src[start:scanner.offset])
	}
	// When in Math-Layout, scan until EOF or a '#' at the beginning of a line
	if scanner.textMode == textMath && scanner.mode == modeNormal {
		start := scanner.offset
		scanner.skipUntilEmptyLine()
		// Scan until a line starts with the same or less indent as the previous tag
		for ; scanner.ch != -1 && ((scanner.offset-scanner.lineOffset > scanner.indent) || scanner.ch == ' ' || scanner.ch == '\t' || scanner.ch == '\n' || scanner.ch == '\r'); scanner.next() {
		}
		scanner.textMode = textNormal
		scanner.mode = modeNewTag
		return TokenMathText, string(scanner.src[start:scanner.offset])
	}
	// When in Config-Mode, scan until EOF or an empty line.
	// This mode is used by the "#define" tag, because YAML config follows directly after "#define".
	if scanner.mode == modeConfig {
		start := scanner.offset
		scanner.skipUntilEmptyLine()
		scanner.mode = scanner.nextMode
		return TokenConfigText, string(scanner.src[start:scanner.offset])
	}
	if scanner.mode == modeInjectSection {
		scanner.mode = modeNormal
		return TokenSection, ""
	}
	// Beginning of file? -> Move to the first non-whitespace character
	if scanner.offset == 0 {
		scanner.next()
		scanner.skipWhitespace(true)
	}
scanAgain:
	switch scanner.ch {
	case -1: // EOF
		return TokenEOF, ""
	case 0: // Strange, but who knows
		scanner.next()
		goto scanAgain
	case '%':
		if scanner.lineOffset == scanner.offset || scanner.mode == modeNewTag {
			for scanner.next(); scanner.ch != -1 && scanner.ch != '\n'; scanner.next() {
			}
			if scanner.ch == '\n' {
				scanner.next()
			}
			goto scanAgain
		}
	case '>', '#':
		// The '#' and '>' symbols are treated special when it is at the beginning of a line
		if scanner.lineOffset == scanner.offset || scanner.mode == modeNewTag {
			scanner.indent = scanner.offset - scanner.lineOffset
			scanner.mode = modeSection
			scanner.nextMode = modeNormal
			scanner.textMode = textNormal
			// Consume all following hashes and characters up to a whitespace or ';'
			start := scanner.offset
			scanner.next()
			for ; scanner.ch != -1 && scanner.ch != ' ' && scanner.ch != '\r' && scanner.ch != '\t' && scanner.ch != '\n' && scanner.ch != ';'; scanner.next() {
			}
			name := string(scanner.src[start:scanner.offset])
			if scanner.ch == ' ' || scanner.ch == '\r' || scanner.ch == '\t' || scanner.ch == '\n' {
				scanner.mode = modeNormal
				scanner.skipWhitespace(false)
			} else {
				scanner.next() // Skip the ';'
			}
			return TokenSection, name
		}
	case '-', '+':
		if scanner.lineOffset == scanner.offset || scanner.mode == modeNewTag {
			indent := scanner.offset - scanner.lineOffset
			start := scanner.offset
			// YAML?
			/* if scanner.skipString(yamlSeparator) {
				start := scanner.offset
				end := scanner.offset
				for scanner.ch != -1 {
					if scanner.skipString(yamlSeparator) {
						break
					}
					scanner.next()
					end = scanner.offset
				}
				scanner.mode = scanner.nextMode
				return TokenConfigText, string(scanner.src[start:end])
			} */
			// Skip the '-' or '+' character
			scanner.next()
			str := string(scanner.src[start:scanner.offset])
			// A space indicates that normal text follows. Otherwise CSS markup follows
			if scanner.ch == ' ' || scanner.ch == '\t' || scanner.ch == '\r' || scanner.ch == '\n' {
				scanner.mode = modeNormal
			} else {
				scanner.mode = modeSection
				scanner.nextMode = modeNormal
			}
			scanner.textMode = textNormal
			scanner.indent = indent
			return TokenEnum, str
		}
	case '_':
		if scanner.mode == modeNormal {
			scanner.next()
			return TokenStyle, "_"
		}
	case '*':
		if scanner.mode == modeNormal {
			scanner.next()
			return TokenStyle, "*"
		}
	case '|':
		// Table mode or start of a new table?
		if scanner.mode == modeNormal && (scanner.textMode == textTable || scanner.isStartOfLine()) {
			scanner.textMode = textTable
			start := scanner.offset
			// Skip all following `|` characters (required for multi-column cells)
			for scanner.next(); scanner.ch == '|'; scanner.next() {
			}
			end := scanner.offset
			scanner.skipWhitespace(false)
			// | followed by newline is the end of a row
			if scanner.ch == '\n' {
				scanner.next()
				scanner.skipWhitespace(false)
				// An empty line terminates the table
				if scanner.ch == '\n' {
					scanner.textMode = textNormal
					scanner.skipWhitespace(true)
					scanner.mode = modeNewTag
				}
				return TokenTableRow, string(scanner.src[start:end])
			}
			return TokenTableCell, string(scanner.src[start:end])
		}
	case '\r':
		// Windows only. Ignore
		scanner.next()
		goto scanAgain
	case ' ', '\t':
		if scanner.mode == modeSection {
			scanner.next()
			scanner.mode = scanner.nextMode
			scanner.skipWhitespace(true)
			goto scanAgain
		}
		if scanner.mode == modeEmbed || scanner.mode == modeStyle {
			scanner.next()
			scanner.mode = modeNormal
			goto scanAgain
		}
		if scanner.mode == modeNewTag {
			scanner.next()
			goto scanAgain
		}
	case '{':
		if scanner.mode == modeNormal {
			scanner.mode = modeStyle
			scanner.nextMode = modeNormal
			scanner.next()
			return TokenStyle, "{"
		}
	case '}':
		if scanner.mode == modeStyle || scanner.mode == modeNormal {
			scanner.mode = modeNormal
			scanner.next()
			return TokenStyle, "}"
		}
	case '~':
		if scanner.mode == modeEmbed {
			scanner.mode = modeNormal
			scanner.next()
			goto scanAgain
		} else if scanner.mode == modeNormal {
			// Consume all following characters up to '~' or ';' or the beginning of a new line
			scanner.next()
			start := scanner.offset
			for ; scanner.ch != -1 && scanner.ch != '~' && scanner.ch != '\r' && scanner.ch != '\n' && scanner.ch != ';'; scanner.next() {
			}
			name := string(scanner.src[start:scanner.offset])
			if scanner.ch == '~' {
				scanner.next()
			} else if scanner.ch == ';' {
				scanner.next()
				scanner.mode = modeEmbed
				scanner.nextMode = modeNormal
			}
			return TokenEntity, name
		}
	case '`':
		if scanner.mode == modeNormal {
			scanner.next()
			start := scanner.offset
			var str string
			// Code mode is terminated by a backtick or a newline.
			for ; scanner.ch != '`' && scanner.ch != -1 && scanner.ch != '\r' && scanner.ch != '\n'; scanner.next() {
				// If "\`" then the "`" is part of the text.
				// Otherwise, do not treat "\" special
				if scanner.ch == '\\' {
					scanner.next()
					if scanner.ch == '`' {
						str += string(scanner.src[start : scanner.offset-1])
						start = scanner.offset
						scanner.next()
					}
				}
			}
			str += string(scanner.src[start:scanner.offset])
			if scanner.ch == '`' {
				scanner.next()
			}
			return TokenCodeText, str
		}
	case '$':
		if scanner.mode == modeNormal {
			scanner.next()
			start := scanner.offset
			var str string
			// Math mode can only be terminated by "$". A newline does not terminate math mode.
			for ; scanner.ch != '$' && scanner.ch != -1; scanner.next() {
				// If "\$" then the $ is part of the text.
				// Otherwise, do not treat "\" special
				if scanner.ch == '\\' {
					scanner.next()
					if scanner.ch == '$' {
						str += string(scanner.src[start : scanner.offset-1])
						start = scanner.offset
						scanner.next()
					}
				}
			}
			str += string(scanner.src[start:scanner.offset])
			if scanner.ch == '$' {
				scanner.next()
			}
			return TokenMathText, str
		}
	}
	start := scanner.offset
	switch scanner.mode {
	case modeStyle, modeEmbed, modeSection:
		var result string
		for scanner.ch != -1 && scanner.ch != ' ' && scanner.ch != '\t' && scanner.ch != '\r' && scanner.ch != '\n' && (scanner.mode != modeEmbed || scanner.ch != '~') && (scanner.mode != modeStyle || scanner.ch != '}') && scanner.ch != ';' {
			if scanner.ch == '\\' {
				result += string(scanner.src[start:scanner.offset])
				scanner.next()
				start = scanner.offset
			}
			scanner.next()
		}
		if scanner.ch != ';' {
			scanner.mode = scanner.nextMode
		}
		str := result + string(scanner.src[start:scanner.offset])
		scanner.next()
		if len(str) == 0 {
			goto scanAgain
		}
		return TokenValue, str
	case modeNewTag:
		// A new tag is required. Since no other markup could be found, we assume it to be a paragraph
		scanner.mode = modeNormal
		scanner.indent = scanner.offset - scanner.lineOffset
		return TokenSection, "#p"
	default:
		newline := scanner.lineOffset == scanner.offset
		// Text starts with an empty line? -> a paragraph section
		if scanner.ch == '\n' {
			scanner.next()
			scanner.skipWhitespace(false)
			newline = true
			if scanner.ch == '\n' {
				scanner.skipWhitespace(true)
				// If a '#' or '-' or '+' or '>' follows the empty line then ignore the white space that has been skipped
				if scanner.ch == '#' || scanner.ch == '-' || scanner.ch == '+' || scanner.ch == '>' || scanner.ch == '%' {
					scanner.mode = modeNewTag
					goto scanAgain
				}
				// Report the empty line as the beginning of a new paragraph
				scanner.indent = scanner.offset - scanner.lineOffset
				return TokenSection, "#p"
			}
		}
		// Scan text until the next markup
		var result string
		colon := false
		for scanner.ch != -1 && scanner.ch != '{' && scanner.ch != '}' && scanner.ch != '~' && scanner.ch != '`' && scanner.ch != '$' && scanner.ch != '_' && scanner.ch != '*' &&
			(!newline || (scanner.ch != '#' && scanner.ch != '+' && scanner.ch != '-' && scanner.ch != '%' && scanner.ch != '>')) &&
			(scanner.textMode != textTable || scanner.ch != '|') &&
			(scanner.textMode != textParagraph || scanner.ch != '\n' || !colon) {
			// Break upon an empty line
			if newline && scanner.ch == '\n' {
				scanner.next()
				scanner.mode = modeNewTag
				break
			}
			// Still seeing a new line that contains at most spaces?
			if scanner.ch == '\r' || scanner.ch == '\n' || (newline && (scanner.ch == ' ' || scanner.ch == '\t')) {
				scanner.next()
				newline = true
				continue
			}
			// The line is not empty and it does not start a tag
			newline = false
			if scanner.ch == ':' {
				colon = true
			} else if scanner.ch == '\\' {
				colon = false
				result += string(scanner.src[start:scanner.offset])
				scanner.next()
				start = scanner.offset
			} else if scanner.ch != ' ' && scanner.ch != '\t' && scanner.ch != '\r' {
				colon = false
			}
			scanner.next()
		}
		result += string(scanner.src[start:scanner.offset])
		// The next markup starts a new line or at least a line that contained spaces so far?
		// Then force the scanner to recognize a new tag when going futher.
		if newline && (scanner.ch == '#' || scanner.ch == '+' || scanner.ch == '-' || scanner.ch == '%' || scanner.ch == '>') {
			scanner.mode = modeNewTag
		} else if colon && scanner.textMode == textParagraph && scanner.ch == '\n' {
			scanner.skipWhitespace(true)
			scanner.mode = modeNewTag
			scanner.textMode = textNormal
			result = strings.TrimRight(result, " \r\t\n")
		}
		return TokenText, result
	}
}
