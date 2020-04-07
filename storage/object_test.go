package storage

import (
	"testing"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/stretchr/testify/require"
)

func TestZlibBufferForObject(t *testing.T) {
	o := &plumbing.MemoryObject{}
	o.SetType(plumbing.BlobObject)
	o.SetSize(14)
	_, err := o.Write([]byte("Hello, World!\n"))
	require.Nil(t, err)
	require.Equal(t, o.Hash().String(), "8ab686eafeb1f44702738c8b0f24f2567c36da6d")

	buf, err := ZlibBufferForObject(o)
	require.Nil(t, err)

	require.Equal(t, buf.Bytes(), []byte{120, 156, 74, 202, 201, 79, 82, 48, 52, 97, 240, 72, 205, 201, 201, 215, 81, 8, 207, 47, 202, 73, 81, 228, 2, 4, 0, 0, 255, 255, 78, 21, 6, 152})
}
