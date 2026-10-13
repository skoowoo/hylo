package handler

import "testing"

func TestUserLang(t *testing.T) {
	cases := []struct {
		name string
		in   langSignals
		want string
	}{
		{"nothing", langSignals{}, ""},
		{"non-English system", langSignals{Languages: []string{"ja-JP", "en-US"}, Inputs: []string{"zh-Hans"}}, "ja-JP"},
		{"English only", langSignals{Languages: []string{"en-US"}, Locale: "en_US", Inputs: []string{"en"}}, "en-US"},
		{"English system, pinyin", langSignals{Languages: []string{"en-CN", "zh-Hans-CN"}, Locale: "en_CN", Inputs: []string{"en", "zh-Hans"}}, "zh-Hans-CN"},
		{"pinyin alone", langSignals{Languages: []string{"en-US"}, Locale: "en_US", Inputs: []string{"en", "zh-Hans"}}, "zh-Hans"},
		{"Windows layout", langSignals{Languages: []string{"en-US"}, Locale: "zh-CN", Inputs: []string{"en-US", "zh-CN"}}, "zh-CN"},
		{"preferred list decides", langSignals{Languages: []string{"en-US", "ko-KR"}, Inputs: []string{"ja", "ko"}}, "ko-KR"},
		{"region decides", langSignals{Languages: []string{"en-US"}, Locale: "en_TW", Inputs: []string{"zh-Hans", "zh-Hant"}}, "zh-Hant"},
		{"script matters", langSignals{Languages: []string{"en-US", "zh-Hant-TW"}, Inputs: []string{"zh-Hans"}}, "zh-Hans"},
	}
	for _, c := range cases {
		if got := userLang(c.in); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
