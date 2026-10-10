package unlock

import (
	"encoding/binary"
	"errors"
)

// 酷狗 KGM/VPR：与上游 kgm.ts / KgmWasm 一致。掩码表用 C++ 数学推导版
// （table1/table2 内嵌，无需外部 kgm.mask 文件）。

var (
	kgmVprHeader = []byte{0x05, 0x28, 0xBC, 0x96, 0xE9, 0xE4, 0x5A, 0x43, 0x91, 0xAA, 0xBD, 0xD0, 0x7A, 0xF5, 0x36, 0x31}
	kgmKgmHeader = []byte{0x7C, 0xD5, 0x32, 0xEB, 0x86, 0x02, 0x7F, 0x4B, 0xA8, 0xAF, 0xA6, 0x8E, 0x0F, 0xFF, 0x99, 0x14}
	// kgmVprMaskDiff 由生成脚本写入 kgm_tables.go
)

// kgmGetMask 推导掩码（与 C++ getMask 一致）。
func kgmGetMask(pos int) byte {
	offset := pos >> 4
	var value byte
	for offset >= 0x11 {
		value ^= kgmTable1[offset%272]
		offset >>= 4
		value ^= kgmTable2[offset%272]
		offset >>= 4
	}
	return kgmMaskV2PreDef[pos%272] ^ value
}

// DecryptKGM 解密 KGM/VPR 文件，返回音频数据。
func DecryptKGM(data []byte, isVpr bool) ([]byte, error) {
	hdr := kgmKgmHeader
	if isVpr {
		hdr = kgmVprHeader
	}
	if len(data) < 16 || !bytesEqual(data[:16], hdr) {
		if isVpr {
			return nil, errors.New("不是有效的 VPR 文件")
		}
		return nil, errors.New("不是有效的 KGM 文件")
	}
	if len(data) < 0x2c {
		return nil, errors.New("bad kgm header")
	}
	headerLen := int(binary.LittleEndian.Uint32(data[0x10:]))
	if headerLen < 0 || headerLen > len(data) {
		return nil, errors.New("bad kgm header length")
	}
	var key [17]byte
	copy(key[:], data[0x1c:0x2c])
	audio := data[headerLen:]
	for i := range audio {
		med8 := key[i%17] ^ audio[i]
		med8 ^= (med8 & 0xf) << 4
		msk8 := kgmGetMask(i)
		msk8 ^= (msk8 & 0xf) << 4
		audio[i] = med8 ^ msk8
		if isVpr {
			audio[i] ^= kgmVprMaskDiff[i%17]
		}
	}
	return audio, nil
}
