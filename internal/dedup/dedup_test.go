package dedup

import (
	"testing"

	"LNH-musictag/internal/model"
)

func TestGroupBySameName(t *testing.T) {
	tracks := []*model.Track{
		{ID: "a", FileName: "广岛之恋.mp3", Ext: ".mp3", Bitrate: 192, SampleRate: 44100},
		{ID: "b", FileName: "广岛之恋.flac", Ext: ".flac", Bitrate: 1000, SampleRate: 96000},
		{ID: "c", FileName: "独奏曲.mp3", Ext: ".mp3", Bitrate: 320, SampleRate: 44100},
		{ID: "d", FileName: "独奏曲.ogg", Ext: ".ogg", Bitrate: 400, SampleRate: 48000},
	}
	groups := GroupBySameName(tracks)
	if len(groups) != 2 {
		t.Fatalf("期望 2 组，得到 %d", len(groups))
	}
	for _, g := range groups {
		if g.Method != "format" {
			t.Fatalf("method 应为 format: %v", g.Method)
		}
		var best *model.Track
		for _, tr := range tracks {
			for _, id := range g.IDs {
				if tr.ID == id {
					if best == nil || qualityScore(tr) > qualityScore(best) {
						best = tr
					}
				}
			}
		}
		if g.KeepID != best.ID {
			t.Fatalf("组 %q KeepID=%s 应为最佳 %s", g.Key, g.KeepID, best.ID)
		}
	}
	// 音质最佳者：flac 胜过 mp3
	for _, g := range groups {
		if g.Key == "广岛之恋" && g.KeepID != "b" {
			t.Fatalf("广岛之恋 应保留 flac(b)，得到 %s", g.KeepID)
		}
	}
}
