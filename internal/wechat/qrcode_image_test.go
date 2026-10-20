package wechat

import (
	"testing"
)

func TestQrcodeScanPayloadPrefersImgContent(t *testing.T) {
	got := QrcodeScanPayload(&QrcodeResp{
		Qrcode:           "abc",
		QrcodeImgContent: "https://example.com/scan",
	})
	if got != "https://example.com/scan" {
		t.Fatalf("payload = %q", got)
	}
}

func TestQrcodeDisplayFromRespEmpty(t *testing.T) {
	_, err := QrcodeDisplayFromResp(&QrcodeResp{})
	if err == nil {
		t.Fatal("expected error for empty payload")
	}
}
