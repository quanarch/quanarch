// Package conf reads conf.qu, the file that identifies a Quanarch project.
//
// The syntax is line based. Each line holds one directive: a name followed by
// arguments separated by whitespace. A directive may open a block by ending
// its line with "{"; the block is closed by a line holding only "}".
// Arguments containing whitespace can be quoted with double quotes.
// A "#" outside quotes starts a comment that runs to the end of the line.
//
//	quan billing
//	stack react-ts@1
//
//	port 4001
//
//	qua qua-a@1
//
//	zone admin {
//		qua qua-b@2
//	}
package conf

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// FileName is the name of the project file.
const FileName = "conf.qu"

// Directive is a single line of a conf.qu file, with its block if it has one.
type Directive struct {
	Name     string
	Args     []string
	Block    []*Directive
	HasBlock bool
	Line     int
}

// Error is a problem found at a position in a conf.qu file.
type Error struct {
	File string
	Line int
	Msg  string
}

func (e *Error) Error() string {
	if e.Line == 0 {
		return fmt.Sprintf("%s: %s", e.File, e.Msg)
	}
	return fmt.Sprintf("%s:%d: %s", e.File, e.Line, e.Msg)
}

// ParseDirectives reads the directive tree from r without interpreting it.
// file is used only in error messages.
func ParseDirectives(r io.Reader, file string) ([]*Directive, error) {
	var (
		root  []*Directive
		stack []*Directive // open blocks, innermost last
		line  int
	)
	errorf := func(format string, args ...any) error {
		return &Error{File: file, Line: line, Msg: fmt.Sprintf(format, args...)}
	}
	appendTo := func(d *Directive) {
		if len(stack) == 0 {
			root = append(root, d)
			return
		}
		parent := stack[len(stack)-1]
		parent.Block = append(parent.Block, d)
	}

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line++
		tokens, err := tokenize(scanner.Text())
		if err != nil {
			return nil, errorf("%v", err)
		}
		if len(tokens) == 0 {
			continue
		}

		if tokens[0].text == "}" && !tokens[0].quoted {
			if len(tokens) > 1 {
				return nil, errorf(`"}" must be on a line of its own`)
			}
			if len(stack) == 0 {
				return nil, errorf(`unexpected "}"`)
			}
			stack = stack[:len(stack)-1]
			continue
		}

		d := &Directive{Line: line}
		last := tokens[len(tokens)-1]
		if last.text == "{" && !last.quoted {
			d.HasBlock = true
			tokens = tokens[:len(tokens)-1]
			if len(tokens) == 0 {
				return nil, errorf(`"{" must follow a directive`)
			}
		}
		for i, t := range tokens {
			if !t.quoted && (t.text == "{" || t.text == "}") {
				return nil, errorf("unexpected %q", t.text)
			}
			if i == 0 {
				if t.quoted {
					return nil, errorf("directive name cannot be quoted")
				}
				d.Name = t.text
				continue
			}
			d.Args = append(d.Args, t.text)
		}

		appendTo(d)
		if d.HasBlock {
			stack = append(stack, d)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, &Error{File: file, Msg: err.Error()}
	}
	if len(stack) > 0 {
		open := stack[len(stack)-1]
		return nil, &Error{File: file, Line: open.Line, Msg: fmt.Sprintf("block %q is never closed", open.Name)}
	}
	return root, nil
}

type token struct {
	text   string
	quoted bool
}

// tokenize splits one line into tokens, dropping comments.
func tokenize(line string) ([]token, error) {
	var (
		tokens []token
		cur    strings.Builder
		inWord bool
	)
	flush := func() {
		if inWord {
			tokens = append(tokens, token{text: cur.String()})
			cur.Reset()
			inWord = false
		}
	}

	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case c == '#':
			flush()
			return tokens, nil
		case c == ' ' || c == '\t' || c == '\r':
			flush()
		case c == '"':
			if inWord {
				return nil, fmt.Errorf("unexpected quote inside %q", cur.String())
			}
			text, n, err := readQuoted(line[i+1:])
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, token{text: text, quoted: true})
			i += n
			if next := i + 1; next < len(line) && line[next] != ' ' && line[next] != '\t' && line[next] != '\r' && line[next] != '#' {
				return nil, fmt.Errorf("closing quote must be followed by whitespace")
			}
		default:
			cur.WriteByte(c)
			inWord = true
		}
	}
	flush()
	return tokens, nil
}

// readQuoted reads a quoted string whose opening quote has been consumed.
// It returns the unescaped text and how many bytes were consumed, including
// the closing quote.
func readQuoted(s string) (string, int, error) {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"':
			return b.String(), i + 1, nil
		case '\\':
			if i+1 < len(s) && (s[i+1] == '"' || s[i+1] == '\\') {
				i++
			}
		}
		b.WriteByte(s[i])
	}
	return "", 0, fmt.Errorf("unterminated quoted string")
}
