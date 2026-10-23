package wechat

import (
	"strings"
	"testing"
)

func TestExtractTextPlain(t *testing.T) {
	msg := Message{ItemList: []MessageItem{{
		Type:     MessageItemTypeText,
		TextItem: &TextItem{Text: "hello"},
	}}}
	if got := ExtractText(msg); got != "hello" {
		t.Fatalf("got %q", got)
	}
}

func TestExtractTextQuote(t *testing.T) {
	msg := Message{ItemList: []MessageItem{{
		Type:     MessageItemTypeText,
		TextItem: &TextItem{Text: "reply"},
		RefMsg: &RefMessage{
			Title: "title",
			MessageItem: &MessageItem{
				TextItem: &TextItem{Text: "quoted"},
			},
		},
	}}}
	got := ExtractText(msg)
	if !strings.Contains(got, "[引用:") || !strings.Contains(got, "reply") {
		t.Fatalf("got %q", got)
	}
}
