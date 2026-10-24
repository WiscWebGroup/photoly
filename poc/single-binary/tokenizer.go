package main

// The SigLIP2 (Gemma) tokenizer from tokenizer.json, as the Hugging Face tokenizers library
// applies it: added tokens are matched in the raw text first; every other piece has spaces
// replaced by "▁" and goes through BPE with byte fallback; "<eos>" ends the sequence.

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

const (
	maxTokens = 64
	// Queries are at most 64 tokens, far below this. The cap keeps BPE's quadratic merge
	// loop cheap on hostile input.
	maxQueryBytes = 1024
)

var errQuery = errors.New("invalid query")

type merge struct{ rank, id int32 }

type Tokenizer struct {
	vocab  map[string]int32
	merges map[[2]int32]merge
	added  []addedToken
	bytes  [256]int32 // byte-fallback tokens <0x00>..<0xFF>
	eos    int32
}

type addedToken struct {
	content string
	id      int32
}

func loadTokenizer(path string) (*Tokenizer, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var raw struct {
		AddedTokens []struct {
			ID      int32  `json:"id"`
			Content string `json:"content"`
		} `json:"added_tokens"`
		Model struct {
			Type         string           `json:"type"`
			ByteFallback bool             `json:"byte_fallback"`
			Vocab        map[string]int32 `json:"vocab"`
			Merges       [][2]string      `json:"merges"`
		} `json:"model"`
	}
	if err := json.NewDecoder(f).Decode(&raw); err != nil {
		return nil, fmt.Errorf("tokenizer.json: %w", err)
	}
	if raw.Model.Type != "BPE" || !raw.Model.ByteFallback {
		return nil, errors.New("tokenizer.json: expected a BPE model with byte fallback")
	}
	t := &Tokenizer{vocab: raw.Model.Vocab, merges: make(map[[2]int32]merge, len(raw.Model.Merges))}
	for rank, m := range raw.Model.Merges {
		a, okA := t.vocab[m[0]]
		b, okB := t.vocab[m[1]]
		id, ok := t.vocab[m[0]+m[1]]
		if !okA || !okB || !ok {
			return nil, fmt.Errorf("tokenizer.json: merge %q %q is not in the vocabulary", m[0], m[1])
		}
		t.merges[[2]int32{a, b}] = merge{int32(rank), id}
	}
	for _, a := range raw.AddedTokens {
		t.added = append(t.added, addedToken{a.Content, a.ID})
	}
	unk, okUnk := t.vocab["<unk>"]
	eos, okEOS := t.vocab["<eos>"]
	if !okUnk || !okEOS {
		return nil, errors.New("tokenizer.json: <unk> or <eos> missing")
	}
	t.eos = eos
	for b := range t.bytes {
		// Some bytes (tab, newline) are ordinary tokens with no <0x..> form; like Hugging
		// Face, fall back to <unk> if one is ever needed.
		id, ok := t.vocab[fmt.Sprintf("<0x%02X>", b)]
		if !ok {
			id = unk
		}
		t.bytes[b] = id
	}
	return t, nil
}

// Encode returns the token ids of text, ending with <eos>, without padding.
func (t *Tokenizer) Encode(text string) []int32 {
	var ids []int32
	for {
		pos, tok := t.nextAdded(text)
		ids = t.bpe(ids, text[:pos])
		if tok == nil {
			return append(ids, t.eos)
		}
		ids = append(ids, tok.id)
		text = text[pos+len(tok.content):]
	}
}

// nextAdded finds the leftmost added token in text, preferring the longest at that spot.
func (t *Tokenizer) nextAdded(text string) (int, *addedToken) {
	for i := range len(text) {
		var best *addedToken
		for k := range t.added {
			a := &t.added[k]
			if strings.HasPrefix(text[i:], a.content) && (best == nil || len(a.content) > len(best.content)) {
				best = a
			}
		}
		if best != nil {
			return i, best
		}
	}
	return len(text), nil
}

// bpe appends the ids of one piece of text without added tokens.
func (t *Tokenizer) bpe(ids []int32, text string) []int32 {
	if text == "" {
		return ids
	}
	var syms []int32
	for _, r := range strings.ReplaceAll(text, " ", "▁") {
		if id, ok := t.vocab[string(r)]; ok {
			syms = append(syms, id)
			continue
		}
		for _, b := range []byte(string(r)) {
			syms = append(syms, t.bytes[b])
		}
	}
	// Merge the adjacent pair with the lowest rank (leftmost on ties) until none is left.
	for {
		at, best := -1, merge{rank: -1}
		for i := 0; i+1 < len(syms); i++ {
			if m, ok := t.merges[[2]int32{syms[i], syms[i+1]}]; ok && (at < 0 || m.rank < best.rank) {
				at, best = i, m
			}
		}
		if at < 0 {
			return append(ids, syms...)
		}
		syms[at] = best.id
		syms = append(syms[:at+1], syms[at+2:]...)
	}
}

// inputIDs prepares a search query the way the browser does: lowercase, at most 64
// tokens including <eos>, padded with <pad> (0) to 64.
func (t *Tokenizer) inputIDs(query string) ([]int64, error) {
	if query == "" || len(query) > maxQueryBytes {
		return nil, fmt.Errorf("%w: use 1 to %d bytes", errQuery, maxQueryBytes)
	}
	ids := t.Encode(strings.ToLower(query))
	if len(ids) > maxTokens {
		return nil, fmt.Errorf("%w: query is %d tokens; the limit is %d", errQuery, len(ids), maxTokens)
	}
	out := make([]int64, maxTokens)
	for i, id := range ids {
		out[i] = int64(id)
	}
	return out, nil
}
