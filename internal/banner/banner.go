// Package banner draws the quanarch logo, version and one-line definition.
package banner

import "strings"

// Definition says what Quanarch is in one sentence.
const Definition = "An architecture between the monolith and microservices: quans live in a quanar and reach it only through quas."

// glyphs is the ANSI Shadow lettering of QUANARCH, six rows per letter.
var glyphs = map[rune][6]string{
	'Q': {" ██████╗ ", "██╔═══██╗", "██║   ██║", "██║▄▄ ██║", "╚██████╔╝", " ╚══▀▀═╝ "},
	'U': {"██╗   ██╗", "██║   ██║", "██║   ██║", "██║   ██║", "╚██████╔╝", " ╚═════╝ "},
	'A': {" █████╗ ", "██╔══██╗", "███████║", "██╔══██║", "██║  ██║", "╚═╝  ╚═╝"},
	'N': {"███╗   ██╗", "████╗  ██║", "██╔██╗ ██║", "██║╚██╗██║", "██║ ╚████║", "╚═╝  ╚═══╝"},
	'R': {"██████╗ ", "██╔══██╗", "██████╔╝", "██╔══██╗", "██║  ██║", "╚═╝  ╚═╝"},
	'C': {" ██████╗", "██╔════╝", "██║     ", "██║     ", "╚██████╗", " ╚═════╝"},
	'H': {"██╗  ██╗", "██║  ██║", "███████║", "██╔══██║", "██║  ██║", "╚═╝  ╚═╝"},
}

// Logo returns QUANARCH in large letters, one string per row.
func Logo() []string {
	rows := make([]string, 6)
	for _, r := range "QUANARCH" {
		for i, part := range glyphs[r] {
			rows[i] += part
		}
	}
	for i := range rows {
		rows[i] = strings.TrimRight(rows[i], " ")
	}
	return rows
}

// Text is the logo, the version and the definition as plain text.
func Text(version string) string {
	var b strings.Builder
	for _, row := range Logo() {
		b.WriteString(row + "\n")
	}
	b.WriteString(version + "\n\n" + Definition + "\n")
	return b.String()
}
