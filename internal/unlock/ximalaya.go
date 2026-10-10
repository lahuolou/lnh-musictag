package unlock

import "errors"

// 喜马拉雅 X2M/X3M：前 1024 字节按 scramble 表重排并异或固定密钥。

var (
	x2mKey = []byte{0x78, 0x6d, 0x6c, 0x79} // "xmly"
	x3mKey = []byte{
		0x33, 0x39, 0x38, 0x39, 0x64, 0x31, 0x31, 0x31, 0x61, 0x61, 0x64, 0x35,
		0x36, 0x31, 0x33, 0x39, 0x34, 0x30, 0x66, 0x34, 0x66, 0x63, 0x34, 0x34,
		0x62, 0x36, 0x33, 0x39, 0x62, 0x32, 0x39, 0x32,
	}
)

// DecryptXimalaya 解密 X2M/X3M，返回音频数据。
func DecryptXimalaya(data []byte, ext string) ([]byte, error) {
	const headerSize = 1024
	if len(data) < headerSize {
		return nil, errors.New("文件太小")
	}
	head := append([]byte(nil), data[:headerSize]...)
	switch ext {
	case "x2m":
		for idx := 0; idx < headerSize; idx++ {
			data[idx] = head[x2mScrambleTable[idx]] ^ x2mKey[idx%len(x2mKey)]
		}
	case "x3m":
		for idx := 0; idx < headerSize; idx++ {
			data[idx] = head[x3mScrambleTable[idx]] ^ x3mKey[idx%len(x3mKey)]
		}
	default:
		return nil, errors.New("file type is incorrect")
	}
	return data, nil
}
