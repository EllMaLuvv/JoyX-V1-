package inbound

import "testing"

func TestMatchAutoReply(t *testing.T) {
	cases := []struct {
		text string
		want string
		ok   bool
	}{
		{"ijin kompe", "Mode On Aktif DiNyalakan ", true},
		{"izin kompe dong", "Mode On Aktif DiNyalakan ", true},
		{"IZIN   kompe", "Mode On Aktif DiNyalakan ", true},
		{"tifosi", "", true},
		{"Tifosi nih", "", true},
		{"halo semua", "", false},
		{"", "", false},
	}
	for _, testCase := range cases {
		got, ok := matchAutoReply(testCase.text)
		if ok != testCase.ok {
			t.Fatalf("matchAutoReply(%q) ok = %v; want %v", testCase.text, ok, testCase.ok)
		}
		if testCase.want != "" && got != testCase.want {
			t.Fatalf("matchAutoReply(%q) = %q; want %q", testCase.text, got, testCase.want)
		}
		if !ok && got != "" {
			t.Fatalf("matchAutoReply(%q) = %q; want no reply", testCase.text, got)
		}
	}
}

func TestTifosiJokesAreNotEmpty(t *testing.T) {
	if len(tifosiJokes) == 0 {
		t.Fatal("Joy's tifosi jokes are missing")
	}
	for _, joke := range tifosiJokes {
		if joke == "" {
			t.Fatal("an empty tifosi joke")
		}
	}
}
