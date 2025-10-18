package pool

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func makeFunc(sz int) interface{} {
	return make([]int, 0, sz)
}

func TestPool(t *testing.T) {
	testPool := New(1, 8, 2, makeFunc)

	cases := []struct {
		size int
		want int
	}{
		{1, 1},
		{3, 4},
		{10, 10},
	}

	for _, c := range cases {
		ret := testPool.Get(c.size)
		require.Equal(t, c.want, cap(ret.([]int)))
		testPool.Put(ret)
	}
}
