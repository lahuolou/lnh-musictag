package unlock

import "encoding/binary"

// TencentTea 是 QQ 音乐系（QMC）使用的 TEA 变体（CBC 链式、明文格式
// [PadLen(1)+Pad(0-7)+Salt(2)+Body+Zero(7)]）。与上游 QmcWasm 的
// TencentTea.hpp 逐字节对齐。

const teaDelta = 0x9e3779b9

const (
	teaSaltLen = 2
	teaZeroLen = 7
)

// teaDecryptECB 解密一个 8 字节块（大端）。
func teaDecryptECB(src, key []byte, rounds int) []byte {
	var y, z, sum uint32
	k := [4]uint32{}
	for i := 0; i < 4; i++ {
		k[i] = binary.BigEndian.Uint32(key[i*4:])
	}
	y = binary.BigEndian.Uint32(src[0:])
	z = binary.BigEndian.Uint32(src[4:])
	sum = teaDelta * uint32(rounds)
	for i := 0; i < rounds; i++ {
		z -= ((y << 4) + k[2]) ^ (y + sum) ^ ((y >> 5) + k[3])
		y -= ((z << 4) + k[0]) ^ (z + sum) ^ ((z >> 5) + k[1])
		sum -= teaDelta
	}
	out := make([]byte, 8)
	binary.BigEndian.PutUint32(out[0:], y)
	binary.BigEndian.PutUint32(out[4:], z)
	return out
}

// teaEncryptECB 加密一个 8 字节块（大端）。
func teaEncryptECB(src, key []byte, rounds int) []byte {
	var y, z, sum uint32
	k := [4]uint32{}
	for i := 0; i < 4; i++ {
		k[i] = binary.BigEndian.Uint32(key[i*4:])
	}
	y = binary.BigEndian.Uint32(src[0:])
	z = binary.BigEndian.Uint32(src[4:])
	sum = 0
	for i := 0; i < rounds; i++ {
		sum += teaDelta
		y += ((z << 4) + k[0]) ^ (z + sum) ^ ((z >> 5) + k[1])
		z += ((y << 4) + k[2]) ^ (y + sum) ^ ((y >> 5) + k[3])
	}
	out := make([]byte, 8)
	binary.BigEndian.PutUint32(out[0:], y)
	binary.BigEndian.PutUint32(out[4:], z)
	return out
}

// teaEncryptLen 计算给定明文长度需要的填充字节数。
func teaEncryptLen(inLen int) int {
	n := inLen + 1 + teaSaltLen + teaZeroLen
	if pad := n % 8; pad != 0 {
		return 8 - pad
	}
	return 0
}

// encryptTencentTea 加密（CBC），输出长度是 8 的倍数。
func encryptTencentTea(in, key []byte) []byte {
	padLen := teaEncryptLen(len(in))
	srcBuf := make([]byte, 8)
	ivPlain := make([]byte, 8)
	out := []byte{}
	ivCrypt := 0
	tmpIdx := 0

	// 第一个块：PadLen + 随机 Padding（此处用确定性伪随机即可，解密端不校验内容）
	srcBuf[0] = byte(padLen) & 0x07
	tmpIdx = 1
	for n := 0; n < padLen; n++ {
		srcBuf[tmpIdx] = byte(n*31 + 7)
		tmpIdx++
	}

	cryptBlock := func() {
		start := len(out)
		out = append(out, make([]byte, 8)...)
		for j := 0; j < 8; j++ {
			srcBuf[j] ^= out[ivCrypt+j]
		}
		enc := teaEncryptECB(srcBuf, key, 16)
		copy(out[start:], enc)
		for j := 0; j < 8; j++ {
			out[start+j] ^= ivPlain[j]
		}
		copy(ivPlain, srcBuf)
		tmpIdx = 0
		ivCrypt = start
	}

	for i := 1; i <= teaSaltLen; {
		if tmpIdx < 8 {
			srcBuf[tmpIdx] = byte(0x50 + i)
			tmpIdx++
			i++
		}
		if tmpIdx == 8 {
			cryptBlock()
		}
	}

	inPos := 0
	for inPos < len(in) {
		if tmpIdx < 8 {
			srcBuf[tmpIdx] = in[inPos]
			tmpIdx++
			inPos++
		}
		if tmpIdx == 8 {
			cryptBlock()
		}
	}

	for i := 1; i <= teaZeroLen; {
		if tmpIdx < 8 {
			srcBuf[tmpIdx] = 0
			tmpIdx++
			i++
		}
		if tmpIdx == 8 {
			cryptBlock()
		}
	}
	return out
}

// decryptTencentTea 解密，返回明文和是否通过 Zero 校验。
func decryptTencentTea(in, key []byte) ([]byte, bool) {
	if len(in)%8 != 0 || len(in) < 16 {
		return nil, false
	}
	tmpBuf := teaDecryptECB(in[0:8], key, 16)
	nPadLen := int(tmpBuf[0] & 0x7)
	outLen := len(in) - 1 - nPadLen - teaSaltLen - teaZeroLen
	if outLen < 0 {
		return nil, false
	}
	outBuf := make([]byte, outLen)

	ivPrev := make([]byte, 8)
	ivCur := append([]byte(nil), in[0:8]...)
	inBufPos := 8
	tmpIdx := 1 + nPadLen

	cryptBlock := func() {
		copy(ivPrev, ivCur)
		copy(ivCur, in[inBufPos:inBufPos+8])
		for j := 0; j < 8; j++ {
			tmpBuf[j] ^= ivCur[j]
		}
		tmpBuf = teaDecryptECB(tmpBuf, key, 16)
		inBufPos += 8
		tmpIdx = 0
	}

	for i := 1; i <= teaSaltLen; {
		if tmpIdx < 8 {
			tmpIdx++
			i++
		} else {
			cryptBlock()
		}
	}

	outPos := 0
	for outPos < outLen {
		if tmpIdx < 8 {
			outBuf[outPos] = tmpBuf[tmpIdx] ^ ivPrev[tmpIdx]
			outPos++
			tmpIdx++
		} else {
			cryptBlock()
		}
	}

	for i := 1; i <= teaZeroLen; i++ {
		if tmpBuf[i] != ivPrev[i] {
			return nil, false
		}
	}
	return outBuf, true
}
