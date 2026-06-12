package gemini

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVeoResolutionAliases(t *testing.T) {
	for name, tc := range map[string]struct {
		input string
		want  string
	}{
		"480 numeric": {"480", "480p"},
		"480 upper":   {"480P", "480p"},
		"720 lower":   {"720p", "720p"},
		"1080 upper":  {"1080P", "1080p"},
	} {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.want, SizeToVeoResolution(tc.input))
			require.Equal(t, tc.want, NormalizeVeoResolution(tc.input))
		})
	}

	require.Equal(t, "1080p", ResolveVeoResolution(map[string]any{"resolution": "480"}, "1080p", "720p"))
	require.Equal(t, "480p", ResolveVeoResolution(map[string]any{"resolution": "480"}, "", "1080p"))
	require.Equal(t, "720p", ResolveVeoResolution(nil, "", ""))
	require.Equal(t, "4k", NormalizeVeoResolution("4K"))
}
