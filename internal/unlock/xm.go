package unlock

import (
	"errors"
)

// 虾米 XM：魔数 ifmt + FE FE FE FE，类型串 ' WAV'/'FLAC'/' MP3'/' A4M'，
// key=data[0xf]，dataOffset=24 位小端，audioData[cur]=(audioData[cur]-key)^0xff。
// 非 XM 的 wav/mp3/flac/m4a 文件在此 raw 直通（与上游一致）。

var xmMagic = []byte{0x69, 0x66, 0x6D, 0x74} // "ifmt"

var xmFileType = map[string]string{
	" WAV": "wav",
	"FLAC": "flac",
	" MP3": "mp3",
	" A4M": "m4a",
}

// DecryptXM 解密 XM；非魔数时若 ext==xm 报错，否则 raw 直通（带嗅探）。
func DecryptXM(data []byte, ext string) ([]byte, string, error) {
	if len(data) < 12 || !bytesEqual(data[:4], xmMagic) || !bytesEqual(data[8:12], []byte{0xFE, 0xFE, 0xFE, 0xFE}) {
		if ext == "xm" {
			return nil, "", errors.New("此 XM 文件已损坏")
		}
		if e := SniffExt(data); e != "" {
			return data, e, nil
		}
		return nil, "", errors.New("无法识别文件格式")
	}
	typeText := string(data[4:8])
	target, ok := xmFileType[typeText]
	if !ok {
		return nil, "", errors.New("未知的 .xm 文件类型")
	}
	key := data[0xf]
	dataOffset := int(data[0xc]) | int(data[0xd])<<8 | int(data[0xe])<<16
	audio := data[0x10:]
	if dataOffset > len(audio) {
		return nil, "", errors.New("bad xm offset")
	}
	for cur := dataOffset; cur < len(audio); cur++ {
		audio[cur] = (audio[cur] - key) ^ 0xff
	}
	return audio, target, nil
}
