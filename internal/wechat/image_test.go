package wechat

import (
	"bytes"
	"crypto/aes"
	"encoding/base64"
	"testing"
)

func TestDecodeImage(t *testing.T) {
	key := bytes.Repeat([]byte{0x11}, 16)
	plain := []byte{0x89, 'P', 'N', 'G', 1, 2, 3, 4}
	body := encryptAES128ECB(t, plain, key)

	got, err := DecodeImage(ImageItem{AESKey: "11111111111111111111111111111111"}, body)
	if err != nil || !bytes.Equal(got, plain) {
		t.Fatalf("hex key: %q %v", got, err)
	}
	got, err = DecodeImage(ImageItem{Media: &CDNMedia{AESKey: base64.StdEncoding.EncodeToString(key)}}, body)
	if err != nil || !bytes.Equal(got, plain) {
		t.Fatalf("raw key: %q %v", got, err)
	}
	hexText := []byte("11111111111111111111111111111111")
	got, err = DecodeImage(ImageItem{Media: &CDNMedia{AESKey: base64.StdEncoding.EncodeToString(hexText)}}, body)
	if err != nil || !bytes.Equal(got, plain) {
		t.Fatalf("hex-in-base64: %q %v", got, err)
	}
	got, err = DecodeImage(ImageItem{}, plain)
	if err != nil || !bytes.Equal(got, plain) {
		t.Fatalf("plaintext: %q %v", got, err)
	}
}

func TestExtractTextSkipsImages(t *testing.T) {
	msg := Message{ItemList: []MessageItem{
		{Type: MessageItemTypeImage, ImageItem: &ImageItem{AESKey: "aa"}},
		{Type: MessageItemTypeText, TextItem: &TextItem{Text: "看这张"}},
	}}
	if got := ExtractText(msg); got != "看这张" {
		t.Fatalf("text %q", got)
	}
	if len(Images(msg)) != 1 {
		t.Fatalf("images %+v", Images(msg))
	}
	only := Message{ItemList: []MessageItem{{Type: MessageItemTypeImage, ImageItem: &ImageItem{}}}}
	if ExtractText(only) != "" || len(Images(only)) != 1 {
		t.Fatalf("image-only text %q images %d", ExtractText(only), len(Images(only)))
	}
}

func encryptAES128ECB(t *testing.T, plain, key []byte) []byte {
	t.Helper()
	n := aes.BlockSize - len(plain)%aes.BlockSize
	plain = append(append([]byte{}, plain...), bytes.Repeat([]byte{byte(n)}, n)...)
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]byte, len(plain))
	for i := 0; i < len(plain); i += aes.BlockSize {
		block.Encrypt(out[i:i+aes.BlockSize], plain[i:i+aes.BlockSize])
	}
	return out
}
