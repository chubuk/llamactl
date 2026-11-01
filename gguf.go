package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	ggufTypeArray = 9
	maxKV         = 1024
	maxStr        = 1 << 20
)

func readGGUFString(r io.Reader) (string, error) {
	var n uint64
	if err := binary.Read(r, binary.LittleEndian, &n); err != nil {
		return "", err
	}
	if n > maxStr {
		return "", fmt.Errorf("слишком длинная строка в GGUF: %d байт", n)
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", err
	}
	return string(buf), nil
}

func skipGGUFArray(r io.Reader) error {
	var elemType uint32
	var count uint64
	if err := binary.Read(r, binary.LittleEndian, &elemType); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &count); err != nil {
		return err
	}
	for i := uint64(0); i < count; i++ {
		if err := skipGGUFValue(r, elemType); err != nil {
			return err
		}
	}
	return nil
}

func skipGGUFValue(r io.Reader, typ uint32) error {
	switch typ {
	case 0, 1, 7: // u8, i8, bool
		_, err := io.CopyN(io.Discard, r, 1)
		return err
	case 2, 3: // u16, i16
		_, err := io.CopyN(io.Discard, r, 2)
		return err
	case 4, 5, 6: // u32, i32, f32
		_, err := io.CopyN(io.Discard, r, 4)
		return err
	case 10, 11, 12: // u64, i64, f64
		_, err := io.CopyN(io.Discard, r, 8)
		return err
	case 8: // string
		_, err := readGGUFString(r)
		return err
	case ggufTypeArray:
		return skipGGUFArray(r)
	default:
		return fmt.Errorf("неизвестный тип GGUF: %d", typ)
	}
}

// readGGUFScalar читает значение и возвращает его строкой; для массивов — skip и ok=false.
func readGGUFScalar(r io.Reader, typ uint32) (string, bool, error) {
	var err error
	v := ""
	switch typ {
	case 0:
		var x uint8
		err = binary.Read(r, binary.LittleEndian, &x)
		v = fmt.Sprintf("%d", x)
	case 1:
		var x int8
		err = binary.Read(r, binary.LittleEndian, &x)
		v = fmt.Sprintf("%d", x)
	case 2:
		var x uint16
		err = binary.Read(r, binary.LittleEndian, &x)
		v = fmt.Sprintf("%d", x)
	case 3:
		var x int16
		err = binary.Read(r, binary.LittleEndian, &x)
		v = fmt.Sprintf("%d", x)
	case 4:
		var x uint32
		err = binary.Read(r, binary.LittleEndian, &x)
		v = fmt.Sprintf("%d", x)
	case 5:
		var x int32
		err = binary.Read(r, binary.LittleEndian, &x)
		v = fmt.Sprintf("%d", x)
	case 6:
		var x float32
		err = binary.Read(r, binary.LittleEndian, &x)
		v = fmt.Sprintf("%g", x)
	case 7:
		var x uint8
		err = binary.Read(r, binary.LittleEndian, &x)
		v = fmt.Sprintf("%t", x != 0)
	case 8:
		v, err = readGGUFString(r)
	case 10:
		var x uint64
		err = binary.Read(r, binary.LittleEndian, &x)
		v = fmt.Sprintf("%d", x)
	case 11:
		var x int64
		err = binary.Read(r, binary.LittleEndian, &x)
		v = fmt.Sprintf("%d", x)
	case 12:
		var x float64
		err = binary.Read(r, binary.LittleEndian, &x)
		v = fmt.Sprintf("%g", x)
	case ggufTypeArray:
		err = skipGGUFArray(r)
		return "", false, err
	default:
		return "", false, fmt.Errorf("неизвестный тип GGUF: %d", typ)
	}
	return v, true, err
}

// ggufMeta читает скалярные/строковые KV-пары из заголовка GGUF (массивы пропускаются).
func ggufMeta(path string) ([]kv, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	magic := make([]byte, 4)
	if _, err := io.ReadFull(f, magic); err != nil {
		return nil, err
	}
	if string(magic) != "GGUF" {
		return nil, fmt.Errorf("не GGUF-файл (magic = %q)", magic)
	}
	var version uint32
	if err := binary.Read(f, binary.LittleEndian, &version); err != nil {
		return nil, err
	}
	if version < 2 || version > 3 {
		return nil, fmt.Errorf("неподдерживаемая версия GGUF: %d", version)
	}
	// Порядок заголовка GGUF: tensor_count, затем kv_count.
	var nTensors, nkv uint64
	if err := binary.Read(f, binary.LittleEndian, &nTensors); err != nil {
		return nil, err
	}
	if err := binary.Read(f, binary.LittleEndian, &nkv); err != nil {
		return nil, err
	}
	if nkv > maxKV {
		nkv = maxKV
	}
	out := make([]kv, 0, nkv)
	for i := uint64(0); i < nkv; i++ {
		key, err := readGGUFString(f)
		if err != nil {
			return nil, err
		}
		var typ uint32
		if err := binary.Read(f, binary.LittleEndian, &typ); err != nil {
			return nil, err
		}
		val, scalar, err := readGGUFScalar(f, typ)
		if err != nil {
			return nil, err
		}
		if scalar {
			if len(val) > 60 {
				val = val[:60] + "…"
			}
			out = append(out, kv{key, val})
		}
	}
	return out, nil
}

func formatGgufMeta(pairs []kv) string {
	var b strings.Builder
	for _, p := range pairs {
		fmt.Fprintf(&b, "%s = %s\n", p.key, p.value)
	}
	return strings.TrimSuffix(b.String(), "\n")
}
