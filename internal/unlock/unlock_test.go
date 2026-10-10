package unlock

import (
	"os"
	"path/filepath"
	"testing"
)

var testdataDir = filepath.Join(`E:\项目\web`, "testdata")

func load(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(testdataDir, name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return b
}

func TestQmcDecryptKey(t *testing.T) {
	cases := []struct{ name, key, keyRaw string }{
		{"mflac_map", "mflac_map_key.bin", "mflac_map_key_raw.bin"},
		{"mflac_rc4", "mflac_rc4_key.bin", "mflac_rc4_key_raw.bin"},
		{"mflac0_rc4", "mflac0_rc4_key.bin", "mflac0_rc4_key_raw.bin"},
		{"mgg_map", "mgg_map_key.bin", "mgg_map_key_raw.bin"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			want := load(t, c.key)
			raw := load(t, c.keyRaw)
			got, err := qmcDecryptKey(raw)
			if err != nil {
				t.Fatalf("qmcDecryptKey: %v", err)
			}
			if !bytesEqual(got, want) {
				t.Fatalf("key mismatch: got %d bytes, want %d bytes", len(got), len(want))
			}
		})
	}
}

func TestQmcDecode(t *testing.T) {
	cases := []struct {
		name string
		ext  string
	}{
		{"mflac0_rc4", ".mflac0"},
		{"mflac_rc4", ".mflac"},
		{"mflac_map", ".mflac"},
		{"mgg_map", ".mgg"},
		{"qmc0_static", ".qmc0"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			raw := load(t, c.name+"_raw.bin")
			suffix := load(t, c.name+"_suffix.bin")
			cipher := append(append([]byte(nil), raw...), suffix...)
			want := load(t, c.name+"_target.bin")
			got, err := DecryptQMC(cipher, c.ext)
			if err != nil {
				t.Fatalf("DecryptQMC: %v", err)
			}
			if !bytesEqual(got, want) {
				t.Fatalf("decoded mismatch: got %d bytes, want %d bytes", len(got), len(want))
			}
		})
	}
}

func TestTencentTeaRoundTrip(t *testing.T) {
	key := []byte{0x33, 0x38, 0x36, 0x5A, 0x4A, 0x59, 0x21, 0x40, 0x23, 0x2A, 0x24, 0x25, 0x5E, 0x26, 0x29, 0x28}
	plain := []byte("Hello, unlock-music in Go! 0123456789")
	enc := encryptTencentTea(plain, key)
	dec, ok := decryptTencentTea(enc, key)
	if !ok {
		t.Fatal("decryptTencentTea failed")
	}
	if !bytesEqual(dec, plain) {
		t.Fatalf("round trip mismatch: %q != %q", dec, plain)
	}
}
