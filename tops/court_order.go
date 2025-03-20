package tops

import (
	"strconv"
	"strings"
	"unicode"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
)

func compareCourts(a, b *Court) int {
	nameA := newNumberedText(a.Name())
	nameB := newNumberedText(b.Name())
	return compareNumberedTexts(nameA, nameB)
}

type numberedTextSpan struct {
	isNumber bool // number or text

	number int
	text   string
}

// Representation of a string with the contained digits
// being seen as an integer instead of lexical characters.
// This takes effect when comparing with compareNumberedTexts.
// E.g. "Court 10" would be greater than "Court 2" because
// 10 > 2
type numberedText struct {
	original string
	spans    []numberedTextSpan
}

func newNumberedText(s string) *numberedText {
	t := &numberedText{
		original: s,
		spans:    make([]numberedTextSpan, 0),
	}
	t.parse()
	return t
}

func (t *numberedText) parse() {
	var numBuf strings.Builder
	var textBuf strings.Builder

	for _, r := range t.original {
		isDigit := unicode.IsDigit(r)

		if isDigit && textBuf.Len() > 0 {
			t.addSpan(&textBuf, false)
		} else if !isDigit && numBuf.Len() > 0 {
			t.addSpan(&numBuf, true)
		}

		if isDigit {
			numBuf.WriteRune(r)
		} else {
			textBuf.WriteRune(r)
		}
	}

	if textBuf.Len() > 0 {
		t.addSpan(&textBuf, false)
	} else if numBuf.Len() > 0 {
		t.addSpan(&numBuf, true)
	}
}

func (t *numberedText) addSpan(buf *strings.Builder, isNumber bool) {
	s := buf.String()
	buf.Reset()

	number := -1
	text := ""

	if isNumber {
		number, _ = strconv.Atoi(s)
	} else {
		text = s
	}

	span := numberedTextSpan{
		isNumber: isNumber,
		number:   number,
		text:     text,
	}

	t.spans = append(t.spans, span)
}

func compareNumberedTexts(a, b *numberedText) int {
	lenA, lenB := len(a.spans), len(b.spans)
	minLen := min(lenA, lenB)

	for i := range minLen {
		comparison := compareSpans(a.spans[i], b.spans[i])
		if comparison != 0 {
			return comparison
		}
	}

	if lenA == lenB {
		return 0
	} else if minLen == lenA {
		return -1
	} else {
		return 1
	}
}

func compareSpans(a, b numberedTextSpan) int {
	if a.isNumber != b.isNumber {
		// Numbers before text
		if a.isNumber {
			return -1
		} else {
			return 1
		}
	}

	if a.isNumber {
		if a.number < b.number {
			return -1
		} else if a.number > b.number {
			return 1
		} else {
			return 0
		}
	} else {
		return strings.Compare(a.text, b.text)
	}
}
