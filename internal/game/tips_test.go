package game

import (
	"reflect"
	"strings"
	"testing"
)

// wordWrap is a plain greedy wrap standing in for the UI's wrapText.
func wordWrap(text string, width, _ int) []string {
	var lines []string
	line := ""
	for _, word := range strings.Fields(text) {
		if line != "" && len(line)+1+len(word) > width {
			lines = append(lines, line)
			line = ""
		}
		if line != "" {
			line += " "
		}
		line += word
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}

func TestTipPagesSplitAndCut(t *testing.T) {
	tip := Tip{"Test", []string{"one two", "three four five six seven eight nine ten eleven"}}
	got := TipPages(tip, 12, 3, wordWrap)
	want := [][]string{{"one two"}, {"three four", "five six", "seven eight"}, {"nine ten", "eleven"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatal(got)
	}
}

func playForNudge(w *World) int {
	io := newFakeIO(w.Resources)
	io.Keys = append(io.Keys, " ", " ", "Q")
	NewGame(w, io, GameOptions{}).Run()
	return strings.Count(io.out(), TipsNudge)
}

func TestTipsNudgeOnceOnNewGame(t *testing.T) {
	for _, line := range strings.Split(TipsNudge, "\n") {
		if len(line) > 16 {
			t.Fatal(line)
		}
	}
	w := newWorld(t)
	if playForNudge(w) != 1 || !w.TipsNudged || playForNudge(w) != 0 {
		t.Fatal("nudge")
	}
}

func TestTipsNudgeRememberedWithGame(t *testing.T) {
	fresh := savedCopy(t, newWorld(t))
	if fresh.TipsNudged == nil || *fresh.TipsNudged {
		t.Fatal("fresh save")
	}
	older := fresh
	older.TipsNudged = nil
	again := newWorld(t)
	if !Restore(again, older) || playForNudge(again) != 0 {
		t.Fatal("older save nudged")
	}
	if !Restore(again, fresh) || playForNudge(again) != 1 {
		t.Fatal("fresh save not nudged")
	}
}
