package constant

import "testing"

func TestPath2RelayModePlaygroundImagesGenerations(t *testing.T) {
	tests := []struct {
		name string
		path string
		want int
	}{
		{
			name: "playground images generations",
			path: "/pg/images/generations",
			want: RelayModeImagesGenerations,
		},
		{
			name: "playground chat completions",
			path: "/pg/chat/completions",
			want: RelayModeChatCompletions,
		},
		{
			name: "v1 images generations",
			path: "/v1/images/generations",
			want: RelayModeImagesGenerations,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Path2RelayMode(tt.path); got != tt.want {
				t.Fatalf("Path2RelayMode(%q) = %d, want %d", tt.path, got, tt.want)
			}
		})
	}
}
