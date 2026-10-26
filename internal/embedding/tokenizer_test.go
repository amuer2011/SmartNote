package embedding

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

// Golden IDs were produced by HuggingFace tokenizers using the exact pinned
// tokenizer.json, including its case-sensitive normalization and 512-token limit.
func TestTokenizerHuggingFaceParity(t *testing.T) {
	vocab, e := resources.ReadFile("assets/vocab.txt")
	if e != nil {
		t.Fatal(e)
	}
	tokenizer, e := NewTokenizer(vocab)
	if e != nil {
		t.Fatal(e)
	}
	raw, e := os.ReadFile("tokenizer-golden.json")
	if e != nil {
		t.Fatal(e)
	}
	var cases []struct {
		Text string  `json:"text"`
		IDs  []int64 `json:"ids"`
	}
	if e = json.Unmarshal(raw, &cases); e != nil {
		t.Fatal(e)
	}
	for i, c := range cases {
		got := tokenizer.Encode(c.Text, 512)
		if !reflect.DeepEqual(got, c.IDs) {
			t.Errorf("case %d: got %v, want %v", i, got, c.IDs)
		}
	}
	if tokenizer.PadID != 0 {
		t.Fatal("unexpected padding")
	}
	if tokenizer.Encode("x", 1) != nil {
		t.Fatal("invalid max length accepted")
	}
}
func TestTokenizerMissingSpecialTokens(t *testing.T) {
	if _, e := NewTokenizer([]byte("x\ny")); e == nil {
		t.Fatal("missing special tokens accepted")
	}
}
