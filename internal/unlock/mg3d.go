package unlock

import (
	"encoding/binary"
	"errors"
)

// 咪咕 MG3D：无固定魔数，在 0x20..0x400 区间按 0x20 段探测密钥
// （全大写 hex），用候选密钥解密头部验证 RIFF/WAVEfmt/fmtSize/chunk 名，
// 取第一个候选对整个文件解密。与上游 mg3d.ts 一致。

const mg3dSegmentSize = 0x20

func isUpperHexChar(ch byte) bool {
	return (ch >= '0' && ch <= '9') || (ch >= 'A' && ch <= 'F')
}

func isPrintableAscii(ch byte) bool {
	return ch >= 0x20 && ch <= 0x7E
}

func mg3dDecryptSegment(data, key []byte) []byte {
	out := append([]byte(nil), data...)
	for i := range out {
		out[i] -= key[i%mg3dSegmentSize]
	}
	return out
}

// DecryptMG3D 探测并解密 MG3D 文件，返回解密后的数据。
func DecryptMG3D(data []byte) ([]byte, error) {
	header := data[:min(len(data), 0x100)]
	if len(header) < 0x100 {
		return nil, errors.New("文件太小")
	}
	var candidates [][]byte
	for i := mg3dSegmentSize; i < mg3dSegmentSize*20; i += mg3dSegmentSize {
		if i+mg3dSegmentSize > len(data) {
			break
		}
		key := data[i : i+mg3dSegmentSize]
		allHex := true
		for _, b := range key {
			if !isUpperHexChar(b) {
				allHex = false
				break
			}
		}
		if !allHex {
			continue
		}
		temp := mg3dDecryptSegment(header, key)
		if !bytesEqual(temp[:4], []byte("RIFF")) {
			continue
		}
		if !bytesEqual(temp[8:16], []byte("WAVEfmt ")) {
			continue
		}
		fmtSize := binary.LittleEndian.Uint32(temp[0x10:])
		if fmtSize != 16 && fmtSize != 18 && fmtSize != 40 {
			continue
		}
		firstOff := 0x14 + int(fmtSize)
		if firstOff+4 > len(temp) {
			continue
		}
		chunkName := temp[firstOff : firstOff+4]
		if !allBytes(chunkName, isPrintableAscii) {
			continue
		}
		if firstOff+8 <= len(header) {
			secondOff := firstOff + 8 + int(binary.LittleEndian.Uint32(temp[firstOff+4:]))
			if secondOff+4 <= len(header) {
				if !allBytes(temp[secondOff:secondOff+4], isPrintableAscii) {
					continue
				}
			}
		}
		candidates = append(candidates, append([]byte(nil), key...))
	}
	if len(candidates) == 0 {
		return nil, errors.New("未探测到合适的 MG3D 密钥")
	}
	key := candidates[0]
	out := append([]byte(nil), data...)
	for i := range out {
		out[i] -= key[i%mg3dSegmentSize]
	}
	return out, nil
}

func allBytes(b []byte, f func(byte) bool) bool {
	for _, x := range b {
		if !f(x) {
			return false
		}
	}
	return true
}
