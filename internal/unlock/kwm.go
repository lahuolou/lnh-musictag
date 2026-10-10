package unlock

import (
	"encoding/binary"
	"errors"
	"strconv"
	"strings"
)

// 酷我 KWM：魔数 yeelion-kuwo-tme，8 字节 fileKey 转十进制串派生 32 字节掩码，
// 音频从 0x400 起逐字节异或。与上游 kwm.ts 一致。

var kwmMagic = []byte{0x79, 0x65, 0x65, 0x6C, 0x69, 0x6F, 0x6E, 0x2D, 0x6B, 0x75, 0x77, 0x6F, 0x2D, 0x74, 0x6D, 0x65}

const kwmPreDefinedKey = "MoOtOiTvINGwd2E6n0E1i7L5t2IoOoNk"

// DecryptKWM 解密 KWM 文件，返回音频数据。非 KWM 但可嗅探为 aac 时返回 raw 直通。
func DecryptKWM(data []byte) ([]byte, error) {
	if len(data) < 16 || !bytesEqual(data[:16], kwmMagic) {
		if SniffExt(data) == "aac" {
			return data, nil
		}
		return nil, errors.New("不是有效的 KWM 文件")
	}
	if len(data) < 0x400 {
		return nil, errors.New("bad kwm file")
	}
	fileKey := binary.LittleEndian.Uint64(data[0x18:0x20])
	keyStr := strconv.FormatUint(fileKey, 10)
	if len(keyStr) > 32 {
		keyStr = keyStr[:32]
	} else if len(keyStr) < 32 {
		keyStr = strings.Repeat(keyStr, 32/len(keyStr)+1)[:32]
	}
	var mask [32]byte
	for i := 0; i < 32; i++ {
		mask[i] = kwmPreDefinedKey[i] ^ keyStr[i]
	}
	audio := data[0x400:]
	for i := range audio {
		audio[i] ^= mask[i%0x20]
	}
	return audio, nil
}
