package search

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/blevesearch/bleve/v2/analysis"
	"github.com/blevesearch/bleve/v2/registry"
	"github.com/yanyiwu/gojieba"
)

const jiebaTokenizerName = "jieba"

// jiebaDictFS embeds the cppjieba dictionary files into the Hylo binary.
//
// gojieba.NewJieba() with no arguments resolves dictionary paths relative to
// the source file location recorded at *compile* time (via runtime.Caller),
// which points into the Go module cache of the machine that built the
// binary (e.g. /Users/<builder>/go/pkg/mod/...). That path doesn't exist on
// any other machine, so the search plugin panics for every user except the
// one who compiled it. Embedding the dicts and extracting them to a stable
// per-user directory at runtime makes dictionary resolution independent of
// where or by whom the binary was built.
//
//go:embed jiebadict/*.utf8
var jiebaDictFS embed.FS

// jiebaHandle is a package-level singleton. Loading the jieba dictionaries
// is expensive (~1s, ~11MB), so we do it once and protect concurrent
// Tokenize calls with jiebaMu (gojieba is not goroutine-safe).
var (
	jiebaOnce    sync.Once
	jiebaHandle  *gojieba.Jieba
	jiebaInitErr error
	jiebaMu      sync.Mutex
)

func initJieba() error {
	jiebaOnce.Do(func() {
		dictPaths, err := extractJiebaDicts()
		if err != nil {
			jiebaInitErr = fmt.Errorf("search: extract jieba dictionaries: %w", err)
			return
		}
		jiebaHandle = gojieba.NewJieba(dictPaths[:]...)
	})
	return jiebaInitErr
}

// extractJiebaDicts writes the embedded dictionary files to a stable
// directory under the user's home (~/.hylo/jieba-dict/) and returns their
// paths in the order gojieba.NewJieba expects:
// dict, hmm, user dict, idf, stop words. Files already present with the
// expected size are left untouched.
func extractJiebaDicts() ([5]string, error) {
	var paths [5]string

	home, err := os.UserHomeDir()
	if err != nil {
		return paths, err
	}
	destDir := filepath.Join(home, ".hylo", "jieba-dict")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return paths, err
	}

	names := [5]string{
		"jieba.dict.utf8",
		"hmm_model.utf8",
		"user.dict.utf8",
		"idf.utf8",
		"stop_words.utf8",
	}

	for i, name := range names {
		src := "jiebadict/" + name
		dest := filepath.Join(destDir, name)
		paths[i] = dest

		data, err := jiebaDictFS.ReadFile(src)
		if err != nil {
			return paths, fmt.Errorf("read embedded %s: %w", name, err)
		}

		if info, err := os.Stat(dest); err == nil && info.Size() == int64(len(data)) {
			continue
		}

		tmp := dest + ".tmp"
		if err := os.WriteFile(tmp, data, 0o644); err != nil {
			return paths, fmt.Errorf("write %s: %w", dest, err)
		}
		if err := os.Rename(tmp, dest); err != nil {
			return paths, fmt.Errorf("rename %s: %w", dest, err)
		}
	}

	return paths, nil
}

// JiebaTokenizer is a bleve analysis.Tokenizer backed by gojieba.
// It uses SearchMode, which emits both full compound words and their
// sub-components (e.g. "中华人民" also produces "中华" and "人民"),
// giving better search recall than DefaultMode.
type JiebaTokenizer struct{}

func newJiebaTokenizer(_ map[string]interface{}, _ *registry.Cache) (analysis.Tokenizer, error) {
	if err := initJieba(); err != nil {
		return nil, err
	}
	return &JiebaTokenizer{}, nil
}

// Tokenize implements analysis.Tokenizer.
// gojieba.Word.Start and .End are already byte offsets into the input string,
// so no rune→byte conversion is needed.
func (t *JiebaTokenizer) Tokenize(input []byte) analysis.TokenStream {
	text := string(input)

	jiebaMu.Lock()
	words := jiebaHandle.Tokenize(text, gojieba.SearchMode, true)
	jiebaMu.Unlock()

	tokens := make(analysis.TokenStream, 0, len(words))
	for i, w := range words {
		if strings.TrimSpace(w.Str) == "" {
			continue
		}
		tokens = append(tokens, &analysis.Token{
			Term:     []byte(w.Str),
			Start:    w.Start,
			End:      w.End,
			Position: i + 1,
			Type:     analysis.AlphaNumeric,
		})
	}
	return tokens
}

func init() {
	if err := registry.RegisterTokenizer(jiebaTokenizerName, newJiebaTokenizer); err != nil {
		panic("search: register jieba tokenizer: " + err.Error())
	}
}
