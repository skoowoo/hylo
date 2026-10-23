package channels

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/hardhacker/hylo/internal/config"
	"github.com/hardhacker/hylo/internal/discord"
	"github.com/hardhacker/hylo/internal/mate"
	"github.com/hardhacker/hylo/internal/telegram"
	"github.com/hardhacker/hylo/internal/wechat"
)

var tinyPNG = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
	0xde, 0x00, 0x00, 0x00, 0x0c, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9c, 0x63, 0xf8, 0xcf, 0xc0, 0x00,
	0x00, 0x03, 0x01, 0x01, 0x00, 0xc9, 0xfe, 0x92, 0xef, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e,
	0x44, 0xae, 0x42, 0x60, 0x82,
}

func TestSaveChannelImage(t *testing.T) {
	dir := t.TempDir()
	abs, err := saveChannelImage(dir, tinyPNG, "", "shot.png")
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(abs)
	if err != nil || len(got) != len(tinyPNG) {
		t.Fatalf("saved %d %v", len(got), err)
	}
	if _, err := saveChannelImage(dir, []byte("not an image"), "image/heic", "a.heic"); err != errImageType {
		t.Fatalf("heic err %v", err)
	}
	if _, err := saveChannelImage(dir, []byte("not an image"), "", "a.png"); err != errImageFetch {
		t.Fatalf("garbage err %v", err)
	}
}

func TestHandleImageTurn(t *testing.T) {
	store, err := mate.Open(filepath.Join(t.TempDir(), "vault"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	m := &mate.Mate{Name: "A", AgentID: "pi", Enabled: true}
	if err := store.CreateMate(m); err != nil {
		t.Fatal(err)
	}
	chat := &fakeChat{}
	dir := t.TempDir()
	h := NewHub(store, t.TempDir(), dir, nil, config.Proxy{})
	h.SetChatter(chat)
	ch := mate.Channel{MateID: m.ID, Kind: mate.ChannelDiscord, OwnerUserID: "owner"}
	out := &fakeOut{}

	h.handle(context.Background(), ch, "owner", "owner", "", func(context.Context, string) ([]string, string, error) {
		return nil, "", errImageFetch
	}, out)
	if chat.calls != 0 || len(out.sent) != 1 || out.sent[0] != "The image didn't come through. Please send it again." {
		t.Fatalf("failed download ran the agent: calls %d sent %#v", chat.calls, out.sent)
	}

	h.handle(context.Background(), ch, "owner", "owner", "看图", func(_ context.Context, upload string) ([]string, string, error) {
		abs, err := saveChannelImage(upload, tinyPNG, "", "a.png")
		if err != nil {
			t.Fatal(err)
		}
		return []string{abs}, truncatedImageNotice, nil
	}, out)
	if chat.calls != 1 || chat.text != "看图" || len(chat.images) != 1 {
		t.Fatalf("deliver text %q images %v", chat.text, chat.images)
	}
	if len(out.sent) < 2 || out.sent[len(out.sent)-2] != truncatedImageNotice {
		t.Fatalf("notice %#v", out.sent)
	}
}

func TestCaptionlessImageWaitsForText(t *testing.T) {
	prev := imageCaptionWait
	imageCaptionWait = 40 * time.Millisecond
	t.Cleanup(func() { imageCaptionWait = prev })

	store, err := mate.Open(filepath.Join(t.TempDir(), "vault"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	chat := &fakeChat{}
	h := NewHub(store, t.TempDir(), t.TempDir(), nil, config.Proxy{})
	h.SetChatter(chat)
	ch := mate.Channel{MateID: "m", Kind: mate.ChannelWechat, OwnerUserID: "owner"}
	out := &fakeOut{}
	fetch := func(context.Context, string) ([]string, string, error) {
		return []string{"/vault/.hylo/uploads/a.png"}, "", nil
	}

	h.handle(context.Background(), ch, "owner", "owner", "", fetch, out)
	if calls, _, _ := chat.state(); calls != 0 {
		t.Fatal("a picture alone was submitted immediately")
	}
	h.handle(context.Background(), ch, "owner", "owner", "提取文字", nil, out)
	calls, text, images := chat.state()
	if calls != 1 || text != "提取文字" || len(images) != 1 || images[0] != "/vault/.hylo/uploads/a.png" {
		t.Fatalf("joined turn text %q images %v calls %d", text, images, calls)
	}
	time.Sleep(80 * time.Millisecond)
	if calls, _, _ := chat.state(); calls != 1 {
		t.Fatal("the wait submitted the picture again")
	}

	h.handle(context.Background(), ch, "other", "owner", "", fetch, out)
	time.Sleep(80 * time.Millisecond)
	calls, text, images = chat.state()
	if calls != 2 || text != "" || len(images) != 1 {
		t.Fatalf("expired picture text %q images %v calls %d", text, images, calls)
	}
}

// A held timer that fired just as a text took its batch must not send the
// next batch held under the same chat.
func TestStaleCaptionTimerLeavesTheNextBatch(t *testing.T) {
	prev := imageCaptionWait
	imageCaptionWait = time.Hour
	t.Cleanup(func() { imageCaptionWait = prev })

	store, err := mate.Open(filepath.Join(t.TempDir(), "vault"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	chat := &fakeChat{}
	h := NewHub(store, t.TempDir(), t.TempDir(), nil, config.Proxy{})
	h.SetChatter(chat)
	ch := mate.Channel{MateID: "m", Kind: mate.ChannelWechat, OwnerUserID: "owner"}
	out := &fakeOut{}
	key := pendingKey(ch, "owner")

	h.holdImages(context.Background(), ch, "owner", []string{"/a.png"}, out)
	stale := h.held[key]
	h.takeImages(ch, "owner")
	h.holdImages(context.Background(), ch, "owner", []string{"/b.png"}, out)
	h.flushImages(key, stale)
	if calls, _, _ := chat.state(); calls != 0 || h.held[key] == nil {
		t.Fatalf("stale timer sent the next batch: calls %d", calls)
	}
}

func TestDiscordFetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(tinyPNG)
	}))
	t.Cleanup(srv.Close)
	session, err := discordgo.New("Bot test")
	if err != nil {
		t.Fatal(err)
	}
	session.Client = srv.Client()
	link := &discordLink{session: session}
	paths, notice, err := link.fetchAttachments(context.Background(), t.TempDir(), []discord.Attachment{{
		URL: srv.URL + "/a.png", Name: "a.png", ContentType: "image/png", Size: len(tinyPNG),
	}})
	if err != nil || notice != "" || len(paths) != 1 {
		t.Fatalf("paths %v notice %q err %v", paths, notice, err)
	}
	_, _, err = link.fetchAttachments(context.Background(), t.TempDir(), []discord.Attachment{{
		URL: srv.URL + "/a.heic", Name: "a.heic", ContentType: "image/heic",
	}})
	if err != errImageType {
		t.Fatalf("heic err %v", err)
	}
}

func TestTelegramAlbumAndDownload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(r.URL.Path) >= 7 && r.URL.Path[len(r.URL.Path)-7:] == "getFile" {
			_, _ = w.Write([]byte(`{"ok":true,"result":{"file_id":"f1","file_path":"photos/a.png","file_size":69}}`))
			return
		}
		_, _ = w.Write(tinyPNG)
	}))
	t.Cleanup(srv.Close)
	client := telegram.NewClient("secret-token", config.Proxy{})
	client.SetBase(srv.URL)

	prev := albumWait
	albumWait = 20 * time.Millisecond
	t.Cleanup(func() { albumWait = prev })

	done := make(chan error, 1)
	link := &telegramLink{
		client: client,
		albums: map[string]*telegramAlbum{},
		on: func(_, _, text string, fetch imageFetch) {
			if text != "两张" || fetch == nil {
				done <- errImageFetch
				return
			}
			paths, notice, err := fetch(context.Background(), t.TempDir())
			if err != nil || notice != "" || len(paths) != 2 {
				done <- errImageFetch
				return
			}
			done <- nil
		},
	}
	person := &telegram.User{ID: 7}
	chat := telegram.Chat{ID: 7, Type: "private"}
	link.enqueue(&telegram.Message{
		From: person, Chat: chat, MediaGroupID: "g", Caption: "两张",
		Photo: []telegram.PhotoSize{{FileID: "small", FileSize: 10}, {FileID: "big", FileSize: 20}},
	})
	link.enqueue(&telegram.Message{
		From: person, Chat: chat, MediaGroupID: "g",
		Photo: []telegram.PhotoSize{{FileID: "second", FileSize: 30}},
	})
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("album did not flush")
	}
}

func TestTelegramCaptionJoinsAlbum(t *testing.T) {
	prev := albumWait
	albumWait = time.Hour
	t.Cleanup(func() { albumWait = prev })

	var got []string
	link := &telegramLink{
		albums: map[string]*telegramAlbum{},
		on: func(_, _, text string, fetch imageFetch) {
			got = append(got, text)
			if fetch == nil {
				t.Errorf("caption turned into a text-only turn")
			}
		},
	}
	person := &telegram.User{ID: 7}
	chat := telegram.Chat{ID: 7, Type: "private"}
	link.enqueue(&telegram.Message{
		From: person, Chat: chat, MediaGroupID: "g",
		Photo: []telegram.PhotoSize{{FileID: "big", FileSize: 20}},
	})
	link.enqueue(&telegram.Message{From: person, Chat: chat, Text: "提取文字"})
	if len(got) != 1 || got[0] != "提取文字" || len(link.albums) != 0 {
		t.Fatalf("got %v albums %d", got, len(link.albums))
	}
}

func TestTelegramCaptionDoesNotSwallowNextPhoto(t *testing.T) {
	prev := albumWait
	albumWait = time.Hour
	t.Cleanup(func() { albumWait = prev })

	var got []string
	link := &telegramLink{
		albums: map[string]*telegramAlbum{},
		on:     func(_, _, text string, _ imageFetch) { got = append(got, text) },
	}
	person := &telegram.User{ID: 7}
	chat := telegram.Chat{ID: 7, Type: "private"}
	link.enqueue(&telegram.Message{
		From: person, Chat: chat, MediaGroupID: "g",
		Photo: []telegram.PhotoSize{{FileID: "album", FileSize: 20}},
	})
	link.enqueue(&telegram.Message{
		From: person, Chat: chat, Caption: "单独这张",
		Photo: []telegram.PhotoSize{{FileID: "solo", FileSize: 20}},
	})
	if len(got) != 1 || got[0] != "单独这张" || len(link.albums) != 1 {
		t.Fatalf("got %v albums %d", got, len(link.albums))
	}
}

func TestTelegramCaptionedAlbumKeepsFollowingText(t *testing.T) {
	prev := albumWait
	albumWait = time.Hour
	t.Cleanup(func() { albumWait = prev })

	var got []string
	link := &telegramLink{
		albums: map[string]*telegramAlbum{},
		on:     func(_, _, text string, _ imageFetch) { got = append(got, text) },
	}
	person := &telegram.User{ID: 7}
	chat := telegram.Chat{ID: 7, Type: "private"}
	link.enqueue(&telegram.Message{
		From: person, Chat: chat, MediaGroupID: "g", Caption: "两张",
		Photo: []telegram.PhotoSize{{FileID: "big", FileSize: 20}},
	})
	link.enqueue(&telegram.Message{From: person, Chat: chat, Text: "下一句"})
	if len(got) != 1 || got[0] != "下一句" || len(link.albums) != 1 {
		t.Fatalf("got %v albums %d", got, len(link.albums))
	}
}

func TestTelegramNewDropsAlbum(t *testing.T) {
	prev := albumWait
	albumWait = time.Hour
	t.Cleanup(func() { albumWait = prev })

	var got []string
	link := &telegramLink{
		albums: map[string]*telegramAlbum{},
		on:     func(_, _, text string, fetch imageFetch) { got = append(got, text) },
	}
	person := &telegram.User{ID: 7}
	chat := telegram.Chat{ID: 7, Type: "private"}
	link.enqueue(&telegram.Message{
		From: person, Chat: chat, MediaGroupID: "g",
		Photo: []telegram.PhotoSize{{FileID: "big", FileSize: 20}},
	})
	link.enqueue(&telegram.Message{From: person, Chat: chat, Text: "/new"})
	if len(got) != 1 || got[0] != "/new" || len(link.albums) != 0 {
		t.Fatalf("got %v albums %d", got, len(link.albums))
	}
}

func TestWechatImageDownload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("encrypted_query_param") == "" {
			http.Error(w, "missing", http.StatusBadRequest)
			return
		}
		_, _ = w.Write(tinyPNG)
	}))
	t.Cleanup(srv.Close)
	link := &wechatLink{client: &wechat.Client{CDNBase: srv.URL}}
	paths, _, err := link.fetchImages(context.Background(), t.TempDir(), []wechat.ImageItem{{
		Media: &wechat.CDNMedia{EncryptQueryParam: "abc"},
	}})
	if err != nil || len(paths) != 1 {
		t.Fatalf("paths %v err %v", paths, err)
	}
}
