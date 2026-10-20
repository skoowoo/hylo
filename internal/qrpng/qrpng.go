// Package qrpng renders QR codes for the pages that link a phone app.
package qrpng

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/skip2/go-qrcode"
)

const size = 256

// DataURL renders payload as a PNG data URL for HTML <img src>.
func DataURL(payload string) (string, error) {
	payload = strings.TrimSpace(payload)
	if payload == "" {
		return "", fmt.Errorf("qrpng: empty payload")
	}
	png, err := qrcode.Encode(payload, qrcode.Medium, size)
	if err != nil {
		return "", fmt.Errorf("qrpng: encode: %w", err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(png), nil
}
