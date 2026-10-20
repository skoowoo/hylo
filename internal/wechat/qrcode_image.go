package wechat

import (
	"strings"

	"github.com/hardhacker/hylo/internal/qrpng"
)

// QrcodeScanPayload returns the string WeChat expects encoded in the QR image.
func QrcodeScanPayload(qr *QrcodeResp) string {
	if qr == nil {
		return ""
	}
	if s := strings.TrimSpace(qr.QrcodeImgContent); s != "" {
		return s
	}
	return strings.TrimSpace(qr.Qrcode)
}

// QrcodeDisplayFromResp builds a PNG data URL from a get_bot_qrcode response.
func QrcodeDisplayFromResp(qr *QrcodeResp) (string, error) {
	return qrpng.DataURL(QrcodeScanPayload(qr))
}
