package pool

import (
	"fmt"
	"reflect"
	"sync"
)

// Pool is a bucketed pool for variably sized slices
type Pool struct {
	buckets []sync.Pool
	sizes   []int
	// make is the function used to create an empty slice when none exist yet
	make func(int) interface{}
}

// New returns a new Pool with size buckets for minSize to maxSize
// increasing by the given factor.
func New(minSize, maxSize int, factor float64, makeFunc func(int) interface{}) *Pool {
	if minSize < 1 {
		panic("minSize must be greater than zero")
	}

	if maxSize < 1 {
		panic("maxSize must be greater than zero")
	}

	if factor < 0 {
		panic("factor must be greater than zero")
	}

	var sizes []int
	for s := minSize; s <= maxSize; s = int(float64(s) * factor) {
		sizes = append(sizes, s)
	}

	p := &Pool{
		buckets: make([]sync.Pool, len(sizes)),
		sizes:   sizes,
		make:    makeFunc,
	}

	return p
}

// Get returns a new byte slices that fits the given size.
func (p *Pool) Get(sz int) interface{} {
	for i, bkSize := range p.sizes {
		if sz < bkSize {
			continue
		}
		// 如果内存池中有可用内存，则从内存池中获取
		b := p.buckets[i].Get()
		if b == nil {
			// 创建新的内存
			b = p.make(bkSize)
		}
		return b
	}
	return p.make(sz)
}

// Put adds a slice to the right bucket in the pool
func (p *Pool) Put(s interface{}) {
	slice := reflect.ValueOf(s)

	if slice.Kind() != reflect.Slice {
		panic(fmt.Sprintf("%+v is not a slice", slice))
	}
	for i, size := range p.sizes {
		if slice.Cap() > size {
			continue
		}
		p.buckets[i].Put(slice.Slice(0, 0).Interface())
		return
	}
}
