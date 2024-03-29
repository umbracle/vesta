package taskrunner

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDirHook_ComposeMountData(t *testing.T) {
	cases := []struct {
		files []string
		res   []*mountPoint
	}{
		{
			[]string{
				"/a/b/c",
				"/a/b/c/d",
				"/a/b/e",
				"/b/c",
				"/a/d",
			},
			[]*mountPoint{
				{
					path: "/a",
					files: map[string]string{
						"/a/b/c":   "",
						"/a/b/c/d": "",
						"/a/b/e":   "",
						"/a/d":     "",
					},
				},
				{
					path: "/b",
					files: map[string]string{
						"/b/c": "",
					},
				},
			},
		},
	}

	for _, c := range cases {
		input := map[string]string{}
		for _, file := range c.files {
			input[file] = ""
		}
		found := composeMountData(input)

		// This does not seem to pass on ci with
		// require.Equal(c.res, found)
		if len(c.res) != len(found) {
			t.Fatal("wrong length")
		}
		for i := 0; i < len(c.res); i++ {
			require.Equal(t, c.res[i].files, found[i].files)
			require.Equal(t, c.res[i].path, found[i].path)
		}
	}

}
