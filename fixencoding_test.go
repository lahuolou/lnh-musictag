package main

import (
	"testing"

	"golang.org/x/text/encoding/simplifiedchinese"
)

func TestFixMojibake(t *testing.T) {
	// 已是正常中文 → 保持不动
	if got := fixMojibake("你好世界"); got != "你好世界" {
		t.Fatalf("正常中文被改动: %q", got)
	}
	// 情形 A：UTF-8 字节被按 GBK 读出（如"浣犲ソ"）→ 应还原为"你好"
	mojiA, _ := simplifiedchinese.GBK.NewDecoder().Bytes([]byte("你好"))
	if got := fixMojibake(string(mojiA)); got != "你好" {
		t.Fatalf("情形A失败: 输入%q 得到 %q", string(mojiA), got)
	}
	// 情形 B：GBK 字节被按 UTF-8 读出（如"ÄãºÃ"）→ 应还原为"你好"
	gbkBytes, _ := simplifiedchinese.GBK.NewEncoder().Bytes([]byte("你好"))
	if got := fixMojibake(string(gbkBytes)); got != "你好" {
		t.Fatalf("情形B失败: 输入%q 得到 %q", string(gbkBytes), got)
	}
	// 情形 C：UTF-8 字节被按 Latin-1 读出（如"ä½ å¥½"）→ 应还原为"你好"
	latin := ""
	for _, x := range []byte("你好") {
		latin += string(rune(x))
	}
	if got := fixMojibake(latin); got != "你好" {
		t.Fatalf("情形C失败: 输入%q 得到 %q", latin, got)
	}
}

func TestScriptConverter(t *testing.T) {
	trad := scriptConverter("trad")("广岛之恋 演唱会")
	if trad != "廣島之戀 演唱會" {
		t.Fatalf("简→繁失败: %q", trad)
	}
	simp := scriptConverter("simp")("廣島之戀 演唱會")
	if simp != "广岛之恋 演唱会" {
		t.Fatalf("繁→简失败: %q", simp)
	}
}
