package ui

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

func TestHelpPagesFit(t *testing.T) {
	var all []string
	for _, mode := range []string{"keyboard", "controller"} {
		for _, tiles := range []string{"Standard", "PC VGA"} {
			pages := helpPages(mode, tiles)
			for _, p := range pages {
				if len(p.Lines) > pageRows || len(p.Title) > pageWidth {
					t.Errorf("%s %s: %s has %d lines", mode, tiles, p.Title, len(p.Lines))
				}
				for _, l := range p.Lines {
					if len(l) > pageWidth {
						t.Errorf("%s: line too long: %q", p.Title, l)
					}
				}
				all = append(all, p.Lines...)
			}
			n := len(pages)
			if pages[n-1].Title != fmt.Sprintf("Status %d/%d", n, n) || !strings.Contains(pages[0].Title, fmt.Sprintf("1/%d", n)) {
				t.Errorf("%s: page numbering", mode)
			}
		}
	}
	text := strings.Join(all, "\n")
	if !strings.Contains(text, "pause menu") || !regexp.MustCompile(`(?i)auto\s*combat`).MatchString(text) {
		t.Error("help does not say where settings are")
	}
	join := func(mode, tiles string) string {
		var s []string
		for _, p := range helpPages(mode, tiles) {
			s = append(s, p.Lines...)
		}
		return strings.Join(s, "\n")
	}
	if !strings.Contains(join("keyboard", "Standard"), "Green: poisoned") || strings.Contains(join("keyboard", "Standard"), "G: good") {
		t.Error("Standard status")
	}
	if !strings.Contains(join("controller", "Apple II Mono"), "G: good") {
		t.Error("classic status")
	}
}
