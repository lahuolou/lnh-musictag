package unlock

import (
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// QQ 音乐 QMC 解密：与上游 QmcWasm（qmc.hpp / qmc_key.hpp / qmc_cipher.hpp）
// 及 qmc_cipher.ts 逐字节对齐。支持 v1（Static/Map/RC4）与 v2（EncV2 key、
// QTag 尾注、mflac/mgg 新格式）、QMC 缓存与 iOS(.tm) 直通。

// v2KeyPrefix 是 EncV2 密钥前缀 "QQMusic EncV2,Key:"。
var v2KeyPrefix = []byte{0x51, 0x51, 0x4D, 0x75, 0x73, 0x69, 0x63, 0x20, 0x45, 0x6E, 0x63, 0x56, 0x32, 0x2C, 0x4B, 0x65, 0x79, 0x3A}

var (
	mixKey1 = []byte{0x33, 0x38, 0x36, 0x5A, 0x4A, 0x59, 0x21, 0x40, 0x23, 0x2A, 0x24, 0x25, 0x5E, 0x26, 0x29, 0x28}
	mixKey2 = []byte{0x2A, 0x2A, 0x23, 0x21, 0x28, 0x23, 0x24, 0x25, 0x26, 0x5E, 0x61, 0x31, 0x63, 0x5A, 0x2C, 0x54}
)

// simpleMakeKey 生成 QMC v1 简单密钥（tan 派生），与上游一致（JS 语义）。
func simpleMakeKey(salt int, length int) []byte {
	key := make([]byte, length)
	for i := 0; i < length; i++ {
		tmp := math.Tan(float64(salt) + float64(i)*0.1)
		key[i] = byte(uint8(math.Abs(tmp) * 100.0))
	}
	return key
}

// decryptTencentTeaKey 是 decryptTencentTea 的无错误包装（失败返回 nil,false）。
func decryptTencentTeaKey(in, key []byte) ([]byte, bool) {
	return decryptTencentTea(in, key)
}

// decryptV2Key 处理 EncV2 密钥：前缀匹配则双重 TencentTea + base64。
// 返回 (是否 EncV2 路径, 结果, 失败)。
func decryptV2Key(key []byte) (bool, []byte, error) {
	if len(v2KeyPrefix) > len(key) {
		return false, nil, nil // 前缀不够长，走 v1
	}
	for i := 0; i < len(v2KeyPrefix); i++ {
		if key[i] != v2KeyPrefix[i] {
			return false, nil, nil // 不是 EncV2，走 v1
		}
	}
	tmp := key[len(v2KeyPrefix):]
	out1, ok := decryptTencentTeaKey(tmp, mixKey1)
	if !ok {
		return true, nil, errors.New("encv2 key decode failed")
	}
	out2, ok := decryptTencentTeaKey(out1, mixKey2)
	if !ok {
		return true, nil, errors.New("encv2 key decode failed")
	}
	decoded, err := base64.StdEncoding.DecodeString(string(out2))
	if err != nil {
		return true, nil, errors.New("encv2 key base64 decode failed")
	}
	if len(decoded) < 16 {
		return true, nil, errors.New("encv2 key size too small")
	}
	return true, decoded, nil
}

// qmcDecryptKey 从 base64 编码的密钥文本解密出真正的解密密钥。
func qmcDecryptKey(raw []byte) ([]byte, error) {
	rawDec, err := base64.StdEncoding.DecodeString(string(raw))
	if err != nil {
		return nil, err
	}
	if len(rawDec) < 16 {
		return nil, errors.New("key length is too short")
	}
	// EncV2：以 "QQMusic EncV2,Key:" 开头
	if encV2, out, e := decryptV2Key(rawDec); e != nil {
		return nil, e
	} else if encV2 {
		return out, nil
	}
	// v1：simpleKey 交错 + TencentTea 解密
	simpleKey := simpleMakeKey(106, 8)
	teaKey := make([]byte, 16)
	for i := 0; i < 8; i++ {
		teaKey[i<<1] = simpleKey[i]
		teaKey[(i<<1)+1] = rawDec[i]
	}
	out, ok := decryptTencentTeaKey(rawDec[8:], teaKey)
	if !ok {
		return nil, errors.New("v1 tea decrypt failed")
	}
	result := make([]byte, 8+len(out))
	copy(result, rawDec[:8])
	copy(result[8:], out)
	return result, nil
}

// qmcCipherType 解密类型。
type qmcCipherType int

const (
	qmcCipherInvalid qmcCipherType = iota
	qmcCipherStatic
	qmcCipherMap
	qmcCipherRC4
	qmcCipherCache
	qmcCipherIOS
)

// qmcDecode 保存一次 QMC 解密的解析状态。
type qmcDecode struct {
	cipherType qmcCipherType
	key        []byte // 解密后的密钥（Static 为 nil）
	keySize    int
	tailSize   int
	songID     string
}

// checkType 根据扩展名与文件尾部分辨类型（与 qmc.hpp checkType 一致）。
func (d *qmcDecode) checkType(data []byte, ext string) qmcCipherType {
	low := strings.ToLower(ext)
	if strings.Contains(low, ".qmc") || strings.Contains(low, ".m") {
		if len(data) >= 4 {
			tag := string(data[len(data)-4:])
			switch tag {
			case "QTag":
				if len(data) >= 8 {
					d.keySize = int(binary.BigEndian.Uint32(data[len(data)-8:]))
					d.cipherType = qmcCipherMap // 实际类型由 key 长度决定，占位
					d.tailSize = 8
					return qmcCipherMap
				}
				return qmcCipherInvalid
			case "STag":
				return qmcCipherInvalid
			default:
				d.keySize = int(binary.LittleEndian.Uint32(data[len(data)-4:]))
				if d.keySize < 0x400 {
					d.tailSize = 4
					d.cipherType = qmcCipherMap
					return qmcCipherMap
				}
				d.keySize = 0
				d.cipherType = qmcCipherStatic
				return qmcCipherStatic
			}
		}
		return qmcCipherInvalid
	}
	if strings.Contains(low, ".cache") {
		d.cipherType = qmcCipherCache
		return qmcCipherCache
	}
	if strings.Contains(low, ".tm") {
		d.cipherType = qmcCipherIOS
		return qmcCipherIOS
	}
	return qmcCipherInvalid
}

// parseRawKeyQTag 解析 QTag 内嵌密钥 "key,songId,mediaVer"（与 C++ 一致）：
// 截取第一个逗号前的部分作为密钥文本，songId 取第二段，mediaVer 校验为数字。
func (d *qmcDecode) parseRawKeyQTag(rawKeyBuf []byte) error {
	s := string(rawKeyBuf)
	i := strings.Index(s, ",")
	if i < 0 {
		return errors.New("cannot parse embedded key")
	}
	d.key = []byte(s[:i])
	rest := s[i+1:]
	j := strings.Index(rest, ",")
	if j < 0 {
		return errors.New("cannot parse embedded key")
	}
	d.songID = rest[:j]
	rest = rest[j+1:]
	if k := strings.Index(rest, ","); k >= 0 {
		rest = rest[:k]
	}
	if _, err := strconv.Atoi(rest); err != nil {
		return errors.New("cannot parse embedded key")
	}
	return nil
}

// preDecode 解析文件类型、读取并解密密钥；返回需跳过的尾部长度。
func (d *qmcDecode) preDecode(data []byte, ext string) (int, error) {
	ct := d.checkType(data, ext)
	if ct == qmcCipherInvalid {
		return -1, errors.New("file is invalid or not supported")
	}
	if ct == qmcCipherMap || ct == qmcCipherStatic {
		if d.keySize > 0 {
			if d.keySize > len(data) {
				return -1, errors.New("cannot read embedded key from file")
			}
			rawKeyBuf := append([]byte(nil), data[len(data)-d.tailSize-d.keySize:]...)
			rawKeyBuf = rawKeyBuf[:d.keySize]
			if d.tailSize == 8 { // QTag：key 前有 "key,songId,mediaVer"
				d.cipherType = qmcCipherMap
				if err := d.parseRawKeyQTag(rawKeyBuf); err != nil {
					return -1, err
				}
			} else {
				d.key = rawKeyBuf
			}
			dec, err := qmcDecryptKey(d.key)
			if err != nil {
				return -1, fmt.Errorf("cannot decrypt embedded key: %w", err)
			}
			d.key = dec
		}
		// Static 模式无密钥（keySize=0），无需解密
		if d.keySize == 0 {
			d.cipherType = qmcCipherStatic
		} else if len(d.key) > 300 {
			d.cipherType = qmcCipherRC4
		} else {
			d.cipherType = qmcCipherMap
		}
	}
	return d.keySize + d.tailSize, nil
}

// qmcStaticMask 返回 Static 模式掩码（表已生成 qmc_tables.go）。
func qmcStaticMask(offset int) byte {
	if offset > 0x7fff {
		offset %= 0x7fff
	}
	return qmcStaticBox[(offset*offset+27)&0xff]
}

// qmcMapMask 返回 Map 模式掩码。
// 注意：上游 rotate 是 (value<<r)|(value>>r)（非循环移位，与 C++/TS 逐位一致）。
func qmcMapMask(key []byte, offset int) byte {
	if offset > 0x7fff {
		offset %= 0x7fff
	}
	idx := (offset*offset + 71214) % len(key)
	rotate := ((idx & 0x7) + 4) % 8
	return ((key[idx] << rotate) | (key[idx] >> rotate)) & 0xff
}

// qmcRC4Cipher 实现 RC4 变体（分段解密）。
type qmcRC4Cipher struct {
	key []byte
	S   []byte
	hash uint64
}

func newQmcRC4Cipher(key []byte) *qmcRC4Cipher {
	c := &qmcRC4Cipher{key: key, S: make([]byte, len(key))}
	n := len(key)
	for i := 0; i < n; i++ {
		c.S[i] = byte(i)
	}
	j := 0
	for i := 0; i < n; i++ {
		j = (int(c.S[i]) + j + int(key[i%n])) % n
		c.S[i], c.S[j] = c.S[j], c.S[i]
	}
	// hash 基（TS 语义：>>>0 强制 uint32 截断；测试夹具基于 TS 生成）
	c.hash = 1
	for i := 0; i < n; i++ {
		v := key[i]
		if v == 0 {
			continue
		}
		next := uint32(c.hash) * uint32(v)
		if next == 0 || next <= uint32(c.hash) {
			break
		}
		c.hash = uint64(next)
	}
	return c
}

func (c *qmcRC4Cipher) getSegmentKey(id int) int {
	seed := c.key[id%len(c.key)]
	idx := (float64(c.hash) / float64((id+1)*int(seed))) * 100.0
	return int(idx) % len(c.key)
}

// decrypt 分段解密 buf（offset 为文件内绝对偏移）。
func (c *qmcRC4Cipher) decrypt(buf []byte, offset int) {
	const firstSeg = 0x80
	const segSize = 5120
	toProcess := len(buf)
	processed := 0

	postProcess := func(l int) bool {
		toProcess -= l
		processed += l
		offset += l
		return toProcess == 0
	}

	if offset < firstSeg {
		l := min(firstSeg-offset, len(buf))
		for i := 0; i < l; i++ {
			buf[i] ^= c.key[c.getSegmentKey(offset+i)]
		}
		if postProcess(l) {
			return
		}
	}
	if offset%segSize != 0 {
		l := min(segSize-(offset%segSize), toProcess)
		c.encASegment(buf[processed:processed+l], offset)
		if postProcess(l) {
			return
		}
	}
	for toProcess > segSize {
		c.encASegment(buf[processed:processed+segSize], offset)
		postProcess(segSize)
	}
	if toProcess > 0 {
		c.encASegment(buf[processed:], offset)
	}
}

func (c *qmcRC4Cipher) encASegment(buf []byte, offset int) {
	S := append([]byte(nil), c.S...)
	n := len(c.key)
	skipLen := (offset % 5120) + c.getSegmentKey(offset/5120)
	j, k := 0, 0
	for i := -skipLen; i < len(buf); i++ {
		j = (j + 1) % n
		k = (int(S[j]) + k) % n
		S[k], S[j] = S[j], S[k]
		if i >= 0 {
			buf[i] ^= S[(int(S[j])+int(S[k]))%n]
		}
	}
}

// decode 按已解析的类型解密整份数据（data 会被原地修改），
// offset 为解密起点（通常是 tailSize，即跳过密钥尾部）。
func (d *qmcDecode) decode(data []byte, offset int) ([]byte, error) {
	switch d.cipherType {
	case qmcCipherStatic:
		for i := 0; i < len(data); i++ {
			data[i] ^= qmcStaticMask(offset + i)
		}
	case qmcCipherMap:
		for i := 0; i < len(data); i++ {
			data[i] ^= qmcMapMask(d.key, offset+i)
		}
	case qmcCipherRC4:
		newQmcRC4Cipher(d.key).decrypt(data, offset)
	case qmcCipherCache:
		for i := 0; i < len(data); i++ {
			data[i] ^= 0xf4
			data[i] = ((data[i] & 0x3f) << 2) | (data[i] >> 6)
		}
	case qmcCipherIOS:
		tmHeader := []byte{0x00, 0x00, 0x00, 0x20, 0x66, 0x74, 0x79, 0x70}
		copy(data, tmHeader)
	default:
		return nil, errors.New("file is invalid or encryption type is not supported")
	}
	return data, nil
}

// DecryptQMC 解密 QQ 音乐系文件（qmc0/qmc2/qmc3/qmcflac/mflac/mgg/bkc*/tkm/
// cache/tm2/tm6 等）。返回解密后的音频数据（已去掉密钥尾部）。
func DecryptQMC(data []byte, ext string) ([]byte, error) {
	// checkType 依赖带点的扩展名（.qmc/.m/.cache/.tm），与上游一致
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	d := &qmcDecode{}
	tail, err := d.preDecode(data, ext)
	if err != nil {
		return nil, err
	}
	if d.cipherType == qmcCipherIOS {
		// iOS(.tm)：不裁尾部，整文件覆盖前 8 字节为 m4a 头（与上游一致）
		tmHeader := []byte{0x00, 0x00, 0x00, 0x20, 0x66, 0x74, 0x79, 0x70}
		copy(data[:min(8, len(data))], tmHeader)
		return data, nil
	}
	// 输出只保留解密后的音频部分（去掉密钥尾部），掩码从 0 开始（与 wasm 一致）
	if tail > 0 && tail <= len(data) {
		data = data[:len(data)-tail]
	}
	return d.decode(data, 0)
}
