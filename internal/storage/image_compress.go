package storage

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/draw"
	"image/jpeg"
	"image/png"
)

const (
	// Twice the widest an image is shown (the 1080px lightbox), with room to spare.
	maxImageEdge = 2560
	jpegQuality  = 85
	// Decoding is width*height*4 bytes; past this an upload is left as it came.
	maxCompressPixels = 40 << 20
	// A photo has far more distinct colours than a screenshot or diagram.
	graphicMaxColors = 8192
)

// compressImage re-encodes a JPEG or PNG so the vault doesn't keep camera-sized
// files: the long edge is capped, a photo is saved as JPEG, and a graphic
// (transparency or few colours, like a screenshot) stays PNG so text keeps sharp
// edges. A camera's EXIF orientation is applied and its metadata (location
// included) dropped. Anything it can't make meaningfully smaller comes back as it was.
func compressImage(data []byte, ext string) ([]byte, string) {
	isJPEG := ext == ".jpg" || ext == ".jpeg"
	if !isJPEG && ext != ".png" {
		return data, ext
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width*cfg.Height > maxCompressPixels {
		return data, ext
	}
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return data, ext
	}
	img := toRGBA(src)
	turned := false
	if isJPEG {
		if o := jpegOrientation(data); o > 1 && o <= 8 {
			img, turned = orient(img, o), true
		}
	}
	img, resized := shrink(img, maxImageEdge)

	var buf bytes.Buffer
	outExt := ".jpg"
	if !isJPEG && isGraphic(img) {
		outExt = ".png"
		err = png.Encode(&buf, img)
	} else {
		err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: jpegQuality})
	}
	if err != nil {
		return data, ext
	}
	// Re-encoding a same-size image costs a little quality, so it has to pay for it.
	if !resized && !turned && buf.Len() > len(data)*9/10 {
		return data, ext
	}
	return buf.Bytes(), outExt
}

func toRGBA(src image.Image) *image.RGBA {
	if m, ok := src.(*image.RGBA); ok && m.Rect.Min == (image.Point{}) {
		return m
	}
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Rect, src, b.Min, draw.Src)
	return dst
}

// isGraphic reports transparency or a small palette, judged on an even sample.
func isGraphic(img *image.RGBA) bool {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	step := 1
	for (w/step)*(h/step) > 1<<16 {
		step++
	}
	seen := make(map[uint32]struct{}, graphicMaxColors)
	for y := 0; y < h; y += step {
		row := img.Pix[y*img.Stride:]
		for x := 0; x < w; x += step {
			p := row[x*4 : x*4+4]
			if p[3] != 0xff {
				return true
			}
			seen[uint32(p[0])<<16|uint32(p[1])<<8|uint32(p[2])] = struct{}{}
			if len(seen) > graphicMaxColors {
				return false
			}
		}
	}
	return true
}

// shrink scales img so its long edge is at most edge, averaging each output
// pixel over the source area it covers (sharp, and no aliasing when downscaling).
func shrink(img *image.RGBA, edge int) (*image.RGBA, bool) {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	if w <= edge && h <= edge {
		return img, false
	}
	nw, nh := edge, h*edge/w
	if h > w {
		nw, nh = w*edge/h, edge
	}
	nw, nh = max(nw, 1), max(nh, 1)
	return resampleY(resampleX(img, nw), nh), true
}

// resampleX and resampleY are one pass each of the area average.
func resampleX(src *image.RGBA, nw int) *image.RGBA {
	w, h := src.Rect.Dx(), src.Rect.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, nw, h))
	spans := areaSpans(w, nw)
	for y := 0; y < h; y++ {
		in, out := src.Pix[y*src.Stride:], dst.Pix[y*dst.Stride:]
		for x, sp := range spans {
			var acc [4]float64
			for _, t := range sp {
				p := in[t.i*4 : t.i*4+4]
				for c := range acc {
					acc[c] += float64(p[c]) * t.w
				}
			}
			for c := range acc {
				out[x*4+c] = uint8(acc[c] + 0.5)
			}
		}
	}
	return dst
}

func resampleY(src *image.RGBA, nh int) *image.RGBA {
	w, h := src.Rect.Dx(), src.Rect.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, nh))
	spans := areaSpans(h, nh)
	acc := make([]float64, w*4)
	for y, sp := range spans {
		clear(acc)
		for _, t := range sp {
			in := src.Pix[t.i*src.Stride : t.i*src.Stride+w*4]
			for i, v := range in {
				acc[i] += float64(v) * t.w
			}
		}
		out := dst.Pix[y*dst.Stride:]
		for i, v := range acc {
			out[i] = uint8(v + 0.5)
		}
	}
	return dst
}

type tap struct {
	i int
	w float64
}

// areaSpans gives, for each of n output cells, the source indexes it covers
// and how much of each, weights summing to 1.
func areaSpans(size, n int) [][]tap {
	scale := float64(size) / float64(n)
	spans := make([][]tap, n)
	for o := range spans {
		lo, hi := float64(o)*scale, float64(o+1)*scale
		for i := int(lo); i < size && float64(i) < hi; i++ {
			cover := min(hi, float64(i+1)) - max(lo, float64(i))
			if cover > 0 {
				spans[o] = append(spans[o], tap{i, cover / scale})
			}
		}
	}
	return spans
}

// orient turns a camera image upright for EXIF orientation o (2–8).
func orient(src *image.RGBA, o int) *image.RGBA {
	w, h := src.Rect.Dx(), src.Rect.Dy()
	dw, dh := w, h
	if o >= 5 {
		dw, dh = h, w
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var dx, dy int
			switch o {
			case 2:
				dx, dy = w-1-x, y
			case 3:
				dx, dy = w-1-x, h-1-y
			case 4:
				dx, dy = x, h-1-y
			case 5:
				dx, dy = y, x
			case 6:
				dx, dy = h-1-y, x
			case 7:
				dx, dy = h-1-y, w-1-x
			case 8:
				dx, dy = y, w-1-x
			}
			dst.SetRGBA(dx, dy, src.RGBAAt(x, y))
		}
	}
	return dst
}

// jpegOrientation reads the EXIF orientation tag, 1 (upright) when absent.
func jpegOrientation(data []byte) int {
	for i := 2; i+4 <= len(data); {
		if data[i] != 0xff {
			return 1
		}
		marker := data[i+1]
		if marker == 0xda || marker == 0xd9 { // image data starts: no EXIF before it
			return 1
		}
		size := int(binary.BigEndian.Uint16(data[i+2:]))
		if size < 2 {
			return 1
		}
		seg := data[i+4 : min(len(data), i+2+size)]
		if marker == 0xe1 && len(seg) > 14 && string(seg[:6]) == "Exif\x00\x00" {
			return tiffOrientation(seg[6:])
		}
		i += 2 + size
	}
	return 1
}

func tiffOrientation(t []byte) int {
	var bo binary.ByteOrder
	switch string(t[:2]) {
	case "II":
		bo = binary.LittleEndian
	case "MM":
		bo = binary.BigEndian
	default:
		return 1
	}
	ifd := int(bo.Uint32(t[4:]))
	if ifd+2 > len(t) {
		return 1
	}
	n := int(bo.Uint16(t[ifd:]))
	for e := 0; e < n; e++ {
		at := ifd + 2 + e*12
		if at+12 > len(t) {
			return 1
		}
		if bo.Uint16(t[at:]) == 0x0112 {
			return int(bo.Uint16(t[at+8:]))
		}
	}
	return 1
}
