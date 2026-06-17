// Package slug converts arbitrary strings into URL-safe slugs using only the
// standard library. A slug consists of lowercase ASCII letters, digits, and
// single hyphen separators, with no leading or trailing hyphens.
package slug

import (
	"strings"
	"unicode"
)

// transliterations maps common accented or special Latin runes to their ASCII
// equivalents so that, for example, "Café" becomes "cafe" rather than dropping
// the accented character entirely.
var transliterations = map[rune]string{
	'à': "a", 'á': "a", 'â': "a", 'ã': "a", 'ä': "a", 'å': "a", 'ā': "a", 'ă': "a", 'ą': "a",
	'è': "e", 'é': "e", 'ê': "e", 'ë': "e", 'ē': "e", 'ĕ': "e", 'ė': "e", 'ę': "e", 'ě': "e",
	'ì': "i", 'í': "i", 'î': "i", 'ï': "i", 'ī': "i", 'ĭ': "i", 'į': "i", 'ı': "i",
	'ò': "o", 'ó': "o", 'ô': "o", 'õ': "o", 'ö': "o", 'ø': "o", 'ō': "o", 'ŏ': "o", 'ő': "o",
	'ù': "u", 'ú': "u", 'û': "u", 'ü': "u", 'ū': "u", 'ŭ': "u", 'ů': "u", 'ű': "u", 'ų': "u",
	'ý': "y", 'ÿ': "y",
	'ñ': "n", 'ń': "n", 'ņ': "n", 'ň': "n",
	'ç': "c", 'ć': "c", 'ĉ': "c", 'ċ': "c", 'č': "c",
	'š': "s", 'ś': "s", 'ŝ': "s", 'ş': "s", 'ș': "s",
	'ž': "z", 'ź': "z", 'ż': "z",
	'ð': "d", 'đ': "d", 'ď': "d",
	'ł': "l", 'ĺ': "l", 'ļ': "l", 'ľ': "l",
	'ŕ': "r", 'ř': "r",
	'ţ': "t", 'ť': "t", 'ț': "t",
	'ğ': "g", 'ĝ': "g", 'ġ': "g", 'ģ': "g",
	'ĥ': "h", 'ħ': "h",
	'ĵ': "j",
	'ķ': "k",
	'ŵ': "w",
	'þ': "th",
	'ß': "ss",
	'æ': "ae",
	'œ': "oe",
	'&': "and",
}

// Slugify returns the slug form of s. It lowercases the input, transliterates
// common accented characters to ASCII, replaces every run of unsupported
// characters with a single hyphen, and trims leading and trailing hyphens.
//
// The result only ever contains the bytes a-z, 0-9, and '-'. When s contains no
// usable characters (for example "" or "!!!"), Slugify returns "".
func Slugify(s string) string {
	var b strings.Builder
	b.Grow(len(s))

	// prevHyphen tracks whether the previously written rune was a separator so
	// that consecutive separators collapse into a single hyphen.
	prevHyphen := false

	for _, r := range strings.ToLower(s) {
		switch {
		case r < unicode.MaxASCII && (r >= 'a' && r <= 'z' || r >= '0' && r <= '9'):
			b.WriteRune(r)
			prevHyphen = false
		default:
			if repl, ok := transliterations[r]; ok {
				b.WriteString(repl)
				prevHyphen = false
				continue
			}
			if !prevHyphen {
				b.WriteByte('-')
				prevHyphen = true
			}
		}
	}

	return strings.Trim(b.String(), "-")
}
