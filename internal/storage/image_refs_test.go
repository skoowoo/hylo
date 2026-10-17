package storage

import (
	"slices"
	"testing"
)

func TestParseLocalImages(t *testing.T) {
	raw := "---\nimage_url: \"https://cdn.example.com/og.jpg\"\n---\n\n" +
		"![[a.png|200]]\n" +
		"![b](/_assets/202610/b.jpg)\n" +
		"![c](/api/images/serve?name=c.webp)\n" +
		"![d](<my photo.png>)\n" +
		"![remote](https://cdn.example.com/r.png)\n" +
		"![doc](notes.pdf)\n" +
		"```md\n![code](code.png)\n```\n" +
		"![again](a.png)\n"
	want := []string{"a.png", "b.jpg", "c.webp", "my photo.png"}
	if got := ParseLocalImages([]byte(raw)); !slices.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}
