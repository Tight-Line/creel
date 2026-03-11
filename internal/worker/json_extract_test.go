package worker

import "testing"

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "plain JSON",
			input: `{"action":"ADD"}`,
			want:  `{"action":"ADD"}`,
		},
		{
			name:  "with whitespace",
			input: "  {\"action\":\"ADD\"}  \n",
			want:  `{"action":"ADD"}`,
		},
		{
			name:  "json code fence",
			input: "```json\n{\"action\":\"ADD\"}\n```",
			want:  `{"action":"ADD"}`,
		},
		{
			name:  "plain code fence",
			input: "```\n{\"action\":\"ADD\"}\n```",
			want:  `{"action":"ADD"}`,
		},
		{
			name:  "code fence with trailing whitespace",
			input: "```json\n{\"action\":\"ADD\"}\n```\n",
			want:  `{"action":"ADD"}`,
		},
		{
			name:  "no fences, just JSON",
			input: `[{"fact":"likes fishing"}]`,
			want:  `[{"fact":"likes fishing"}]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractJSON(tt.input)
			if got != tt.want {
				t.Errorf("extractJSON() = %q, want %q", got, tt.want)
			}
		})
	}
}
