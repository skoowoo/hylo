package storage

import (
	"testing"
)

func TestURLEmbedExtendAndPrune(t *testing.T) {
	v, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = v.Close() })
	const now = int64(1_000_000)
	rows := []URLEmbed{
		{URL: "https://a/ok-old", Kind: "audio", Status: "ok", FetchedAt: 1, ExpiresAt: 10},
		{URL: "https://a/ok-stale", Kind: "page", Status: "ok", FetchedAt: 1, ExpiresAt: 500_000},
		{URL: "https://a/err-expired", Status: "error", Error: "fetch_failed", FetchedAt: 1, ExpiresAt: 900_000},
		{URL: "https://a/err-live", Status: "error", Error: "fetch_failed", FetchedAt: 1, ExpiresAt: 2_000_000},
	}
	for _, r := range rows {
		if err := v.PutURLEmbed(r); err != nil {
			t.Fatal(err)
		}
	}
	if err := v.SetURLEmbedPosition("https://a/ok-old", 4200); err != nil {
		t.Fatal(err)
	}

	got, ok, err := v.ExtendURLEmbed("https://a/ok-old", now+100)
	if err != nil || !ok || got.ExpiresAt != now+100 || got.PositionMS != 4200 {
		t.Fatalf("extend ok = %+v %v %v", got, ok, err)
	}
	if _, ok, err := v.ExtendURLEmbed("https://a/err-expired", now+100); err != nil || ok {
		t.Fatalf("extend error row = %v %v", ok, err)
	}
	if _, ok, err := v.ExtendURLEmbed("https://a/missing", now+100); err != nil || ok {
		t.Fatalf("extend missing = %v %v", ok, err)
	}

	if err := v.PruneURLEmbeds(now, 100_000); err != nil {
		t.Fatal(err)
	}
	left, err := v.FreshURLEmbeds([]string{"https://a/ok-old", "https://a/ok-stale", "https://a/err-expired", "https://a/err-live"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"https://a/ok-old", "https://a/ok-stale", "https://a/err-live"} {
		if _, ok := left[want]; !ok {
			t.Errorf("%s pruned", want)
		}
	}
	if _, ok := left["https://a/err-expired"]; ok {
		t.Error("expired error row kept")
	}

	if err := v.PruneURLEmbeds(now, 600_000); err != nil {
		t.Fatal(err)
	}
	left, _ = v.FreshURLEmbeds([]string{"https://a/ok-stale"}, 0)
	if len(left) != 0 {
		t.Error("ok row past keepBefore kept")
	}
}
