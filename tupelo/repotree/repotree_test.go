package repotree

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRepoIsCaseInsenitive(t *testing.T) {

	lower, err := Did("quorumcontrol/dgit-test")
	require.Nil(t, err)
	mixed, err := Did("quorumcontrol/DGIT-test")
	require.Nil(t, err)
	upper, err := Did("QUORUMCONTROL/DGIT-TEST")
	require.Nil(t, err)

	require.Equal(t, lower, mixed)
	require.Equal(t, lower, upper)
}
