package repotree

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRepoIsCaseInsenitive(t *testing.T) {
	lower, err := namedTreeGen.Did("quorumcontrol/dgit-test")
	require.Nil(t, err)
	mixed, err := namedTreeGen.Did("quorumcontrol/DGIT-test")
	require.Nil(t, err)
	upper, err := namedTreeGen.Did("QUORUMCONTROL/DGIT-TEST")
	require.Nil(t, err)

	require.Equal(t, lower, mixed)
	require.Equal(t, lower, upper)
}
