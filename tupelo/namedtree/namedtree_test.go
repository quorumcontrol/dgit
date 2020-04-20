package namedtree

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNamingIsCaseInsenitive(t *testing.T) {
	namedTreeGen := &Generator{Namespace: "Test"}
	lower, err := namedTreeGen.Did("quorumcontrol/dgit-test")
	require.Nil(t, err)
	mixed, err := namedTreeGen.Did("quorumcontrol/DGIT-test")
	require.Nil(t, err)
	upper, err := namedTreeGen.Did("QUORUMCONTROL/DGIT-TEST")
	require.Nil(t, err)

	require.Equal(t, lower, mixed)
	require.Equal(t, lower, upper)
}
