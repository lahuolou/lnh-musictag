package unlock

// 网易云 UC 缓存：整文件逐字节异或 0xA3（与上游 ncmcache.ts 一致）。
// 若异或后无法嗅探为音频则视为无效缓存。
func DecryptNCMCache(data []byte) ([]byte, error) {
	out := append([]byte(nil), data...)
	for i := range out {
		out[i] ^= 0xA3
	}
	if SniffExt(out) == "" {
		return nil, errNotAudio
	}
	return out, nil
}
