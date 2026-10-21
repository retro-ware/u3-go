package ui

import (
	"encoding/binary"
	"errors"
	"image"
)

// decodePICT reads a QuickDraw PICT version 2 holding one DirectBitsRect of
// 32-bit pixels, packed a row at a time (packType 4: each component's bytes
// in a plane of their own, PackBits-compressed). That is how LairWare 2.0
// shipped its "Apple II Mono-Tiles.pict"; nothing else is supported.
func decodePICT(b []byte) (image.Image, error) {
	bad := errors.New("unsupported PICT")
	if len(b) < 512+10+4 {
		return nil, bad
	}
	p := b[512+10:] // past the 512-byte file header, the size and the frame
	be := binary.BigEndian
	at := 0
	word := func() int {
		if at+2 > len(p) {
			at = len(p)
			return -1
		}
		v := int(be.Uint16(p[at:]))
		at += 2
		return v
	}
	for at < len(p) {
		switch op := word(); op {
		case 0x0000, 0x001e: // NOP, DefHilite
		case 0x0011: // version
			at += 2
		case 0x0c00: // header
			at += 24
		case 0x0001: // clip region: its size counts itself
			n := word()
			at += n - 2
		case 0x00a0: // short comment
			at += 2
		case 0x00a1: // long comment
			at += 2
			at += word()
		case 0x009a, 0x009b:
			return directBits(p[at:])
		default:
			return nil, bad
		}
		at += at & 1 // opcodes are word aligned
	}
	return nil, bad
}

// directBits decodes the body of a DirectBitsRect opcode.
func directBits(p []byte) (image.Image, error) {
	be := binary.BigEndian
	if len(p) < 4+46+18 {
		return nil, errors.New("short PICT")
	}
	pm := p[4:] // past baseAddr
	rowBytes := int(be.Uint16(pm[0:]) & 0x3fff)
	top, left := int(int16(be.Uint16(pm[2:]))), int(int16(be.Uint16(pm[4:])))
	bottom, right := int(int16(be.Uint16(pm[6:]))), int(int16(be.Uint16(pm[8:])))
	packType := be.Uint16(pm[12:])
	pixelSize := be.Uint16(pm[28:])
	cmpCount := int(be.Uint16(pm[30:]))
	w, h := right-left, bottom-top
	if packType != 4 || pixelSize != 32 || (cmpCount != 3 && cmpCount != 4) || w <= 0 || h <= 0 {
		return nil, errors.New("unsupported PICT pixels")
	}
	data := pm[46+18:] // past the pixmap, srcRect, dstRect and mode
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	row := make([]byte, 0, cmpCount*w)
	for y := range h {
		var n int
		if rowBytes > 250 {
			if len(data) < 2 {
				return nil, errors.New("short PICT")
			}
			n, data = int(be.Uint16(data)), data[2:]
		} else {
			if len(data) < 1 {
				return nil, errors.New("short PICT")
			}
			n, data = int(data[0]), data[1:]
		}
		if n > len(data) {
			return nil, errors.New("short PICT")
		}
		row = unpackBits(data[:n], row[:0])
		data = data[n:]
		if len(row) < cmpCount*w {
			return nil, errors.New("short PICT row")
		}
		planes := row[(cmpCount-3)*w:] // an alpha plane, if any, comes first and is ignored
		for x := range w {
			i := img.PixOffset(x, y)
			img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = planes[x], planes[w+x], planes[2*w+x], 255
		}
	}
	return img, nil
}

// unpackBits expands PackBits data onto out.
func unpackBits(in, out []byte) []byte {
	for i := 0; i < len(in); {
		n := int(int8(in[i]))
		i++
		switch {
		case n >= 0:
			end := min(len(in), i+n+1)
			out = append(out, in[i:end]...)
			i = end
		case n != -128:
			if i < len(in) {
				for range 1 - n {
					out = append(out, in[i])
				}
			}
			i++
		}
	}
	return out
}
