package styles

import "testing"

func TestFormatPrefixWithKey(t *testing.T) {
	tests := []struct {
		name    string
		project string
		section string
		taskKey string
		want    string
	}{
		{
			name:    "github short key appends number",
			project: "iruoy/fylla",
			taskKey: "fylla#42",
			want:    "iruoy/fylla#42: ",
		},
		{
			name:    "github owner repo key appends number",
			project: "iruoy/fylla",
			taskKey: "iruoy/fylla#42",
			want:    "iruoy/fylla#42: ",
		},
		{
			name:    "github key with section",
			project: "iruoy/fylla",
			section: "Backlog",
			taskKey: "fylla#42",
			want:    "iruoy/fylla#42/Backlog: ",
		},
		{
			name:    "non github project unchanged",
			project: "PROJ",
			taskKey: "PROJ-1",
			want:    "PROJ: ",
		},
		{
			name:    "malformed number unchanged",
			project: "iruoy/fylla",
			taskKey: "fylla#abc",
			want:    "iruoy/fylla: ",
		},
		{
			name:    "missing number unchanged",
			project: "iruoy/fylla",
			taskKey: "fylla",
			want:    "iruoy/fylla: ",
		},
		{
			name:    "already numbered project unchanged",
			project: "iruoy/fylla#42",
			taskKey: "fylla#42",
			want:    "iruoy/fylla#42: ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatPrefixWithKey(tt.project, tt.section, tt.taskKey); got != tt.want {
				t.Fatalf("FormatPrefixWithKey(%q,%q,%q) = %q, want %q", tt.project, tt.section, tt.taskKey, got, tt.want)
			}
		})
	}
}
