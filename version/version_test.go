package version

import "testing"

func TestString_ReturnsFormattedVersionString(t *testing.T) {
	// Save originals and restore after test
	origVersion, origCommit, origDate := Version, Commit, Date
	t.Cleanup(func() {
		Version = origVersion
		Commit = origCommit
		Date = origDate
	})

	tests := []struct {
		name    string
		version string
		commit  string
		date    string
		want    string
	}{
		{
			name:    "default dev values",
			version: "dev",
			commit:  "none",
			date:    "unknown",
			want:    "dev (none) unknown",
		},
		{
			name:    "release values",
			version: "1.2.3",
			commit:  "abc1234",
			date:    "2025-01-15",
			want:    "1.2.3 (abc1234) 2025-01-15",
		},
		{
			name:    "empty values produce parens and spaces",
			version: "",
			commit:  "",
			date:    "",
			want:    " () ",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			Version = tc.version
			Commit = tc.commit
			Date = tc.date

			got := String()
			if got != tc.want {
				t.Errorf("String() = %q, want %q", got, tc.want)
			}
		})
	}
}
