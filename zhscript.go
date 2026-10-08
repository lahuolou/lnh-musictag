// 简繁体转换：对选中曲目的文本标签做简体⇄繁体转换（基于 OpenCC 的 Go 移植 gocc）。
package main

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/liuzl/gocc"

	"LNH-musictag/internal/dedup"
	"LNH-musictag/internal/taglibx"
)

// scriptConverter builds a simp<->trad converter for the given target.
// to == "trad" converts Simplified -> Traditional; otherwise Traditional -> Simplified.
func scriptConverter(to string) func(string) string {
	conv := "s2t"
	if to != "trad" {
		conv = "t2s"
	}
	cc, err := gocc.New(conv)
	if err != nil {
		return func(s string) string { return s }
	}
	return func(s string) string {
		o, err := cc.Convert(s)
		if err != nil {
			return s
		}
		return o
	}
}

// convertScriptHandler converts the text fields of selected tracks between
// simplified and traditional Chinese.
func convertScriptHandler(s *store, fp dedup.Fingerprinter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			IDs []string `json:"ids"`
			To  string   `json:"to"` // "simp" | "trad"
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		conv := scriptConverter(req.To)
		list := resolveTracks(s, req.IDs)
		type res struct {
			ID       string `json:"id"`
			FileName string `json:"fileName"`
			Changed  int    `json:"changed"`
		}
		results := []res{}
		for _, t := range list {
			m := map[string][]string{}
			changed := 0
			for _, k := range textFields {
				v := strings.TrimSpace(t.Tags[k])
				if v == "" {
					continue
				}
				c := conv(v)
				if c != v {
					m[k] = []string{c}
					changed++
				}
			}
			if len(m) > 0 {
				if taglibx.WriteTags(t.Path, m, false) == nil {
					refreshTrack(s, t.Path, fp)
				}
			}
			results = append(results, res{ID: t.ID, FileName: t.FileName, Changed: changed})
		}
		writeJSON(w, http.StatusOK, map[string]any{"total": len(results), "results": results})
	}
}
