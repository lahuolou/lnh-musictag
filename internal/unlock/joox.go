package unlock

import (
	"encoding/binary"
	"errors"
)

// JOOX（实验性）：能识别 "E!" 头与版本、校验长度字段，但密钥派生算法
// 上游 @unlock-music/joox-crypto 已从 npm 下架且源码无法获取，无法实现
// 完整解密。识别通过时返回明确的"需补算法"错误；E!99 版本直接拒绝。

var jooxMagic = []byte{'E', '!'}

// DecryptJOOX 实验性支持：仅做头识别与版本检测。
func DecryptJOOX(data []byte) error {
	if len(data) < 11 || !bytesEqual(data[:2], jooxMagic) {
		return errors.New("不是有效的 JOOX 文件")
	}
	ver := string(data[2:4])
	if ver == "99" {
		return errors.New("该文件使用了新的加密方式，暂时无法解锁")
	}
	if len(data) < 4+8 {
		return errors.New("bad joox file")
	}
	size := binary.BigEndian.Uint64(data[4:12])
	if size > uint64(len(data)-12) {
		return errors.New("bad joox length")
	}
	// 上游算法（@unlock-music/joox-crypto）已无法获取，无法解密
	return errors.New("JOOX 解密算法缺失（上游包已下架），该格式暂无法解锁")
}
