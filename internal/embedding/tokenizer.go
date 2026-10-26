// Package embedding runs the bundled Chinese BGE model without a network service.
package embedding

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// maxInputCharsPerWord mirrors HuggingFace's
// BertWordpieceTokenizer.max_input_chars_per_word. A word longer than this many
// Unicode code points is emitted as a single [UNK].
const maxInputCharsPerWord = 100

// Canonical BERT special tokens. The first four are required for the tokenizer
// to be usable; [MASK] is optional and only preserved when present.
const (
	padToken  = "[PAD]"
	unkToken  = "[UNK]"
	clsToken  = "[CLS]"
	sepToken  = "[SEP]"
	maskToken = "[MASK]"
)

// Tokenizer is a BERT cased WordPiece tokenizer backed by a fixed vocabulary.
// It is safe for concurrent use by multiple goroutines once constructed.
type Tokenizer struct {
	// PadID is the vocabulary id of the [PAD] token. Callers need it when
	// building fixed-length input batches and attention masks.
	PadID int64

	vocab map[string]int64

	clsID int64
	sepID int64
	unkID int64

	// specials lists the recognized special tokens present in the vocabulary,
	// ordered longest first so longest-match scanning is unambiguous.
	specials []string
}

// NewTokenizer builds a tokenizer from the raw bytes of a BERT vocabulary
// file: one token per line, where the zero-based line number is the token id.
// Both LF and CRLF line endings are accepted, as is a trailing newline.
//
// The vocabulary must contain [PAD], [UNK], [CLS] and [SEP]; a leading UTF-8
// BOM is ignored. [MASK] is optional.
func NewTokenizer(vocab []byte) (*Tokenizer, error) {
	raw := strings.TrimPrefix(string(vocab), "\ufeff")
	lines := strings.Split(raw, "\n")
	// A trailing newline produces a final empty element in Go's Split; it is
	// not a vocabulary entry. (readlines() in HuggingFace behaves the same.)
	if n := len(lines); n > 0 && lines[n-1] == "" {
		lines = lines[:n-1]
	}

	t := &Tokenizer{vocab: make(map[string]int64, len(lines))}
	for i, line := range lines {
		token := strings.TrimSuffix(line, "\r")
		t.vocab[token] = int64(i)
	}

	var ok bool
	if t.PadID, ok = t.vocab[padToken]; !ok {
		return nil, fmt.Errorf("embedding: vocabulary is missing required token %s", padToken)
	}
	if t.unkID, ok = t.vocab[unkToken]; !ok {
		return nil, fmt.Errorf("embedding: vocabulary is missing required token %s", unkToken)
	}
	if t.clsID, ok = t.vocab[clsToken]; !ok {
		return nil, fmt.Errorf("embedding: vocabulary is missing required token %s", clsToken)
	}
	if t.sepID, ok = t.vocab[sepToken]; !ok {
		return nil, fmt.Errorf("embedding: vocabulary is missing required token %s", sepToken)
	}

	t.specials = make([]string, 0, 5)
	for _, token := range []string{padToken, unkToken, clsToken, sepToken, maskToken} {
		if _, present := t.vocab[token]; present {
			t.specials = append(t.specials, token)
		}
	}
	sort.Slice(t.specials, func(i, j int) bool {
		return len(t.specials[i]) > len(t.specials[j])
	})

	return t, nil
}

// Encode tokenizes text and returns its BERT input ids, wrapped in [CLS] and
// [SEP]. When the tokenized text does not fit, it is truncated from the right
// so that maxLength ids are returned with the closing [SEP] retained:
//
//	[CLS] token_1 ... token_n [SEP]
//
// maxLength must be at least 2 ([CLS] + [SEP]); smaller values return nil.
func (t *Tokenizer) Encode(text string, maxLength int) []int64 {
	if maxLength < 2 {
		return nil
	}

	tokens := t.tokenize(text)
	if limit := maxLength - 2; len(tokens) > limit {
		tokens = tokens[:limit]
	}

	ids := make([]int64, 0, len(tokens)+2)
	ids = append(ids, t.clsID)
	ids = append(ids, tokens...)
	ids = append(ids, t.sepID)
	return ids
}

// tokenize runs the full basic + WordPiece pipeline without adding special
// tokens. Special tokens embedded in text are kept as their vocabulary ids.
func (t *Tokenizer) tokenize(text string) []int64 {
	var ids []int64
	for _, chunk := range t.splitSpecials(cleanText(text)) {
		if chunk.special {
			ids = append(ids, t.vocab[chunk.text])
			continue
		}
		ids = append(ids, t.tokenizeChunk(chunk.text)...)
	}
	return ids
}

// tokenizeChunk applies the basic tokenizer to a run of ordinary text.
func (t *Tokenizer) tokenizeChunk(text string) []int64 {
	var ids []int64
	for _, token := range strings.Fields(surroundChineseChars(text)) {
		// The bundled tokenizer_config.json sets do_lower_case=false and
		// strip_accents=null: preserve case and accents exactly.
		for _, piece := range splitOnPunctuation(token) {
			if piece == "" {
				continue
			}
			ids = append(ids, t.wordpiece(piece)...)
		}
	}
	return ids
}

// wordpiece performs greedy longest-prefix matching for a single token, adding
// the "##" continuation marker for every piece after the first.
func (t *Tokenizer) wordpiece(token string) []int64 {
	chars := []rune(token)
	if len(chars) > maxInputCharsPerWord {
		return []int64{t.unkID}
	}

	var ids []int64
	for start := 0; start < len(chars); {
		end := len(chars)
		id := int64(-1)
		for start < end {
			sub := string(chars[start:end])
			if start > 0 {
				sub = "##" + sub
			}
			if v, ok := t.vocab[sub]; ok {
				id = v
				break
			}
			end--
		}
		if id < 0 {
			// Nothing matched from this position: the whole word is unknown,
			// matching HuggingFace's is_bad handling.
			return []int64{t.unkID}
		}
		ids = append(ids, id)
		start = end
	}
	return ids
}

// textChunk is a piece of cleaned text that is either an ordinary run or one
// preserved special token.
type textChunk struct {
	text    string
	special bool
}

// splitSpecials splits cleaned text around any special tokens that occur
// verbatim in it. Matching is greedy longest-first, so [MASK] wins over any
// shorter special token sharing its prefix.
func (t *Tokenizer) splitSpecials(text string) []textChunk {
	var chunks []textChunk
	start := 0
	for i := 0; i < len(text); {
		matched := ""
		for _, special := range t.specials {
			if strings.HasPrefix(text[i:], special) {
				matched = special
				break
			}
		}
		if matched == "" {
			_, size := utf8.DecodeRuneInString(text[i:])
			i += size
			continue
		}
		if i > start {
			chunks = append(chunks, textChunk{text: text[start:i]})
		}
		chunks = append(chunks, textChunk{text: matched, special: true})
		i += len(matched)
		start = i
	}
	if start < len(text) {
		chunks = append(chunks, textChunk{text: text[start:]})
	}
	return chunks
}

// cleanText drops NUL, U+FFFD and control characters, and collapses all
// whitespace to a single ASCII space.
func cleanText(text string) string {
	var b strings.Builder
	b.Grow(len(text))
	for _, r := range text {
		switch {
		case r == 0 || r == 0xFFFD:
			continue
		case isControl(r):
			continue
		case isWhitespace(r):
			b.WriteByte(' ')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// isWhitespace mirrors HuggingFace's _is_whitespace: ASCII space, tab, newline,
// carriage return, or any Unicode Zs character.
func isWhitespace(r rune) bool {
	switch r {
	case ' ', '\t', '\n', '\r':
		return true
	}
	return unicode.Is(unicode.Zs, r)
}

// isControl mirrors HuggingFace's _is_control: Cc or Cf, except that tab,
// newline and carriage return are treated as whitespace instead.
func isControl(r rune) bool {
	switch r {
	case '\t', '\n', '\r':
		return false
	}
	return unicode.Is(unicode.Cc, r) || unicode.Is(unicode.Cf, r)
}

// surroundChineseChars pads every CJK ideograph with spaces so that the
// whitespace split turns each character into its own token.
func surroundChineseChars(text string) string {
	var b strings.Builder
	b.Grow(len(text) + 8)
	for _, r := range text {
		if isChineseChar(r) {
			b.WriteByte(' ')
			b.WriteRune(r)
			b.WriteByte(' ')
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// isChineseChar reports whether cp is in one of the CJK ranges used by
// HuggingFace's BasicTokenizer._is_chinese_char.
func isChineseChar(cp rune) bool {
	return (cp >= 0x4E00 && cp <= 0x9FFF) ||
		(cp >= 0x3400 && cp <= 0x4DBF) ||
		(cp >= 0x20000 && cp <= 0x2A6DF) ||
		(cp >= 0x2A700 && cp <= 0x2B73F) ||
		(cp >= 0x2B740 && cp <= 0x2B81F) ||
		(cp >= 0x2B820 && cp <= 0x2CEAF) ||
		(cp >= 0xF900 && cp <= 0xFAFF) ||
		(cp >= 0x2F800 && cp <= 0x2FA1F)
}

// splitOnPunctuation splits a token so that every punctuation character forms
// its own piece, matching HuggingFace's _run_split_on_punc.
func splitOnPunctuation(token string) []string {
	var pieces []string
	var current strings.Builder
	flush := func() {
		if current.Len() > 0 {
			pieces = append(pieces, current.String())
			current.Reset()
		}
	}
	for _, r := range token {
		if isPunctuation(r) {
			flush()
			pieces = append(pieces, string(r))
			continue
		}
		current.WriteRune(r)
	}
	flush()
	return pieces
}

// isPunctuation mirrors HuggingFace's _is_punctuation: ASCII punctuation
// ranges plus every Unicode P* category.
func isPunctuation(r rune) bool {
	if (r >= 33 && r <= 47) || (r >= 58 && r <= 64) ||
		(r >= 91 && r <= 96) || (r >= 123 && r <= 126) {
		return true
	}
	return unicode.Is(unicode.P, r)
}
