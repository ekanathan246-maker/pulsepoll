package database

import "testing"

func TestVoteIndexNeedsReplacement(t *testing.T) {
	tests := []struct {
		name      string
		indexName string
		unique    bool
		want      bool
	}{
		{
			name:      "legacy generated index is replaced",
			indexName: "poll_id_1_voter_id_1",
			unique:    false,
			want:      true,
		},
		{
			name:      "already unique index is retained",
			indexName: "poll_id_1_voter_id_1",
			unique:    true,
			want:      false,
		},
		{
			name:      "unrelated index is retained",
			indexName: "created_at_1",
			unique:    false,
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := voteIndexNeedsReplacement(tt.indexName, tt.unique); got != tt.want {
				t.Fatalf("voteIndexNeedsReplacement(%q, %v) = %v, want %v", tt.indexName, tt.unique, got, tt.want)
			}
		})
	}
}
