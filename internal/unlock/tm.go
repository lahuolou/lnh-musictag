package unlock

import "errors"

// 腾讯 TM（QQ 音乐 iOS .tm2/.tm6）：前 8 字节覆盖为 m4a 头，其余不动。
// 与上游 tm.ts 一致。

var tmHeader = []byte{0x00, 0x00, 0x00, 0x20, 0x66, 0x74, 0x79, 0x70}

// DecryptTM 处理 TM 文件，返回可播放的 m4a 数据。
func DecryptTM(data []byte) ([]byte, error) {
	if len(data) < 8 {
		return nil, errors.New("文件太小")
	}
	copy(data[:8], tmHeader)
	return data, nil
}
