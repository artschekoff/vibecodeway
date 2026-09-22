package symbols

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// Recorded from: ctags --output-format=json --fields=+nl on a tiny repo.
// Non-tag lines (ptag pseudo-tags) and unparsable lines must be skipped.
const ctagsFixture = `{"_type": "ptag", "name": "JSON_OUTPUT_VERSION", "path": "1.0"}
{"_type": "tag", "name": "getUser", "path": "src/user.ts", "line": 12, "kind": "function", "language": "TypeScript"}
{"_type": "tag", "name": "UserRepo", "path": "src/repo.ts", "line": 3, "kind": "class", "language": "TypeScript"}
{"_type": "tag", "name": "getuser", "path": "legacy/u.js", "line": 7, "kind": "function", "language": "JavaScript"}
not json at all
`

func TestParseCtagsJSON(t *testing.T) {
	syms := parseCtagsJSON([]byte(ctagsFixture))
	require.Len(t, syms, 3)
	assert.Equal(t, Symbol{Name: "getUser", Kind: "function", Path: "src/user.ts", Language: "TypeScript", Line: 12}, syms[0])
}

func TestLookup_exactThenCaseInsensitive(t *testing.T) {
	s := NewService(zap.NewNop())
	s.putForTest("/wt", parseCtagsJSON([]byte(ctagsFixture)))

	exact, err := s.Lookup("/wt", "getUser")
	require.NoError(t, err)
	require.Len(t, exact, 1)
	assert.Equal(t, "src/user.ts", exact[0].Path)

	// No exact-case match -> case-insensitive fallback finds both spellings.
	ci, err := s.Lookup("/wt", "GETUSER")
	require.NoError(t, err)
	assert.Len(t, ci, 2)
}

func TestSearch_prefixFirstThenShorter(t *testing.T) {
	s := NewService(zap.NewNop())
	s.putForTest("/wt", []Symbol{
		{Name: "userRepository", Path: "a.ts", Line: 1},
		{Name: "getUser", Path: "b.ts", Line: 1},
		{Name: "user", Path: "c.ts", Line: 1},
	})
	got, err := s.Search("/wt", "user", 10)
	require.NoError(t, err)
	names := []string{got[0].Name, got[1].Name, got[2].Name}
	// Prefix matches ("user", "userRepository") before substring ("getUser");
	// within prefix matches, shorter first.
	assert.Equal(t, []string{"user", "userRepository", "getUser"}, names)
}
