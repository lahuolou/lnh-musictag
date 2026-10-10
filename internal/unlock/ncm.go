package unlock

import (
	"crypto/aes"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"strings"
)

// 网易云 NCM：与上游 ncm.ts 逐字节对齐。密钥/元数据区 AES-128-ECB(PKCS7)，
// 音频用 RC4 变体 keyBox 逐字节异或。

var ncmCoreKey, _ = hex.DecodeString("687a4852416d736f356b496e62617857")
var ncmMetaKey, _ = hex.DecodeString("2331346C6A6B5F215C5D2630553C2728")

var ncmMagic = []byte("CTENFDAM")

// NcmMeta 是 NCM 内嵌元数据（oriMeta / mainMusic）。
type NcmMeta struct {
	MusicName   string   `json:"musicName"`
	Artist      []string `json:"artist"`
	Album       string   `json:"album"`
	AlbumPic    string   `json:"albumPic"`
	MvID        string   `json:"mvId"`
	Format      string   `json:"format"`
	Bitrate     int64    `json:"bitrate"`
	Duration    int64    `json:"duration"`
	Alias       []string `json:"alias"`
	TransNames  []string `json:"transNames"`
	AlbumArtist string   `json:"albumArtist"`
}

// aesECBDecrypt 实现 AES-128-ECB + PKCS7 去填充（Go 标准库无 ECB）。
func aesECBDecrypt(cipherText, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(cipherText) == 0 || len(cipherText)%block.BlockSize() != 0 {
		return nil, errors.New("invalid aes block size")
	}
	out := make([]byte, len(cipherText))
	for i := 0; i < len(cipherText); i += block.BlockSize() {
		block.Decrypt(out[i:i+16], cipherText[i:i+16])
	}
	// PKCS7 unpad
	pad := int(out[len(out)-1])
	if pad == 0 || pad > 16 || pad > len(out) {
		return nil, errors.New("invalid aes padding")
	}
	for _, b := range out[len(out)-pad:] {
		if int(b) != pad {
			return nil, errors.New("invalid aes padding")
		}
	}
	return out[:len(out)-pad], nil
}

// ncmParse 保存 NCM 解析中间状态。
type ncmParse struct {
	offset int
	data   []byte
	keyBox []byte
	meta   *NcmMeta
}

func (p *ncmParse) readU32() uint32 {
	v := binary.LittleEndian.Uint32(p.data[p.offset:])
	p.offset += 4
	return v
}

// getKeyBox 解析密钥区并生成 keyBox。
func (p *ncmParse) getKeyBox() ([]byte, error) {
	keyLen := int(p.readU32())
	if keyLen <= 0 || p.offset+keyLen > len(p.data) {
		return nil, errors.New("bad ncm key length")
	}
	cipherText := p.data[p.offset : p.offset+keyLen]
	p.offset += keyLen
	// 密钥区直接 AES-128-ECB（非 base64）
	plain, err := aesECBDecrypt(cipherText, ncmCoreKey)
	if err != nil {
		return nil, errors.New("bad ncm key")
	}
	keyData := plain[17:] // 去掉前 17 字节
	if len(keyData) == 0 {
		return nil, errors.New("bad ncm key")
	}
	box := make([]byte, 256)
	for i := range box {
		box[i] = byte(i)
	}
	j := 0
	for i := 0; i < 256; i++ {
		j = (int(box[i]) + j + int(keyData[i%len(keyData)])) & 0xff
		box[i], box[j] = box[j], box[i]
	}
	out := make([]byte, 256)
	for i := 0; i < 256; i++ {
		ii := (i + 1) & 0xff
		si := box[ii]
		sj := box[(ii+int(si))&0xff]
		out[i] = box[(int(si)+int(sj))&0xff]
	}
	return out, nil
}

// getMeta 解析元数据区（xor 0x63 → base64 → AES-128-ECB(META_KEY) → JSON）。
func (p *ncmParse) getMeta() (*NcmMeta, error) {
	metaLen := int(p.readU32())
	if metaLen == 0 {
		return nil, nil
	}
	if p.offset+metaLen > len(p.data) {
		return nil, errors.New("bad ncm meta length")
	}
	raw := p.data[p.offset : p.offset+metaLen]
	p.offset += metaLen
	for i := range raw {
		raw[i] ^= 0x63
	}
	// 前 22 字节跳过，其余按 base64 文本解码
	if len(raw) <= 22 {
		return nil, nil
	}
	b64 := string(raw[22:])
	enc, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b64))
	if err != nil {
		return nil, nil // 元数据损坏时静默跳过（音频仍可解锁）
	}
	plain, err := aesECBDecrypt(enc, ncmMetaKey)
	if err != nil {
		return nil, nil
	}
	text := string(plain)
	if idx := strings.IndexByte(text, ':'); idx >= 0 {
		text = text[idx+1:]
	}
	meta := &NcmMeta{}
	_ = jsonUnmarshal([]byte(text), meta)
	return meta, nil
}

// DecryptNCM 解密 NCM/UC 缓存文件，返回音频数据与元数据。
func DecryptNCM(data []byte) ([]byte, *NcmMeta, error) {
	if len(data) < 10 || !bytesEqual(data[:8], ncmMagic) {
		return nil, nil, errors.New("不是有效的 NCM 文件")
	}
	p := &ncmParse{data: data, offset: 10}
	box, err := p.getKeyBox()
	if err != nil {
		return nil, nil, err
	}
	meta, _ := p.getMeta()
	// 音频偏移：CRC32(4) + flag(1) + mediaOffset(4, LE) + 13 字节
	if p.offset+9 > len(data) {
		return nil, nil, errors.New("bad ncm header")
	}
	mediaOff := int(binary.LittleEndian.Uint32(data[p.offset+5:])) + 13
	audio := data[p.offset+mediaOff:]
	for i := range audio {
		audio[i] ^= box[i&0xff]
	}
	return audio, meta, nil
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
