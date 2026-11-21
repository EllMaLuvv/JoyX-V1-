package inbound

import "testing"

func TestNormalizeDotCommand(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{".s", "/sticker"},
		{".s kucing", "/sticker kucing"},
		{".del", "/del"},
		{".reset", "/restart"},
		{".img", "/img"},
		{".rvo", "/rvo"},
		{".dd-Makalah Basis Data-07-10-2026", "/dd Makalah Basis Data 07-10-2026"},
		{".dd-Tugas-2026-10-07", "/dd Tugas 2026-10-07"},
		{"..", ".."},
		{".net", ".net"},
		{"halo .s", "halo .s"},
		{"/sticker", "/sticker"},
		{"", ""},
	}
	for _, testCase := range cases {
		if got := normalizeDotCommand(testCase.in); got != testCase.want {
			t.Fatalf("normalizeDotCommand(%q) = %q; want %q", testCase.in, got, testCase.want)
		}
	}
}

func TestParseRegisteredCommandAcceptsDotCommands(t *testing.T) {
	if _, cmd, ok := parseRegisteredCommand(".s"); !ok || cmd.Name != "sticker" {
		t.Fatalf(".s did not route to /sticker")
	}
	if _, cmd, ok := parseRegisteredCommand(".dd-Tugas-07-10-2026"); !ok || cmd.Name != "dd" {
		t.Fatalf(".dd-... did not route to /dd")
	}
	if _, _, ok := parseRegisteredCommand(".net"); ok {
		t.Fatal(".net must not be treated as a command")
	}
}
