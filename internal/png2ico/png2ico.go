package png2ico

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image/png"
)

// pngToICO преобразует иконку из формата PNG в ICO (32 бита, BGRA),
// необходимый для отображения в системном трее Windows.
func Png2ICO(pngData []byte) ([]byte, error) {
	img, err := png.Decode(bytes.NewReader(pngData))
	if err != nil {
		return nil, fmt.Errorf("декодирование PNG: %w", err)
	}
	rect := img.Bounds()
	w, h := rect.Dx(), rect.Dy()
	if w < 1 || h < 1 || w > 256 || h > 256 {
		return nil, fmt.Errorf("неподдерживаемый размер иконки: %dx%d", w, h)
	}

	// Размеры частей ICO-файла.
	const bmpHeaderSize = 40          // BITMAPINFOHEADER
	xorSize := w * h * 4              // пиксели BGRA
	andRowSize := ((w + 31) / 32) * 4 // строка AND-маски выровнена по 32 бита
	andSize := andRowSize * h         // AND-маска
	imageSize := bmpHeaderSize + xorSize + andSize

	out := new(bytes.Buffer)

	// ICONDIR — заголовок каталога иконок (6 байт).
	writeU16(out, 0) // reserved
	writeU16(out, 1) // тип: 1 — иконка
	writeU16(out, 1) // количество изображений

	// ICONDIRENTRY — описание единственного изображения (16 байт).
	dim := func(v int) byte {
		if v >= 256 {
			return 0 // 0 означает 256
		}
		return byte(v)
	}
	out.WriteByte(dim(w))            // ширина
	out.WriteByte(dim(h))            // высота
	out.WriteByte(0)                 // количество цветов палитры
	out.WriteByte(0)                 // зарезервировано
	writeU16(out, 1)                 // planes
	writeU16(out, 32)                // bit count
	writeU32(out, uint32(imageSize)) // размер данных изображения
	writeU32(out, 6+16)              // смещение до данных изображения (ICONDIR 6 + ICONDIRENTRY 16)

	// BITMAPINFOHEADER (40 байт).
	writeU32(out, 40)                      // biSize
	writeU32(out, uint32(w))               // biWidth
	writeU32(out, uint32(h*2))             // biHeight: XOR-часть + AND-маска
	writeU16(out, 1)                       // biPlanes
	writeU16(out, 32)                      // biBitCount
	writeU32(out, 0)                       // biCompression: BI_RGB
	writeU32(out, uint32(xorSize+andSize)) // biSizeImage
	writeI32(out, 0)                       // biXPelsPerMeter
	writeI32(out, 0)                       // biYPelsPerMeter
	writeU32(out, 0)                       // biClrUsed
	writeU32(out, 0)                       // biClrImportant

	// XOR-часть: пиксели снизу вверх в формате BGRA.
	for y := h - 1; y >= 0; y-- {
		for x := 0; x < w; x++ {
			r, g, b, a := img.At(rect.Min.X+x, rect.Min.Y+y).RGBA()
			out.WriteByte(byte(b >> 8))
			out.WriteByte(byte(g >> 8))
			out.WriteByte(byte(r >> 8))
			out.WriteByte(byte(a >> 8))
		}
	}

	// AND-маска: нули означают, что пиксели полностью непрозрачны.
	out.Write(make([]byte, andSize))

	return out.Bytes(), nil
}

func writeU16(buf *bytes.Buffer, v uint16) {
	_ = binary.Write(buf, binary.LittleEndian, v)
}

func writeU32(buf *bytes.Buffer, v uint32) {
	_ = binary.Write(buf, binary.LittleEndian, v)
}

func writeI32(buf *bytes.Buffer, v int32) {
	_ = binary.Write(buf, binary.LittleEndian, v)
}
