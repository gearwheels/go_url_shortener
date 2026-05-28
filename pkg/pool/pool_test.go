package pool_test

import (
	"testing"

	"github.com/gearwheels/go_url_shortener/pkg/pool"
)

// buf — тестовая структура с методом Reset().
type buf struct {
	data  []byte
	count int
	name  string
}

func (b *buf) Reset() {
	b.data = b.data[:0]
	b.count = 0
	b.name = ""
}

// entry — вторая тестовая структура для проверки универсальности пула.
type entry struct {
	keys []string
	size int
}

func (e *entry) Reset() {
	e.keys = e.keys[:0]
	e.size = 0
}

func TestPool_GetReturnsObject(t *testing.T) {
	p := pool.New(func() *buf { return &buf{data: make([]byte, 0, 16)} })
	got := p.Get()
	if got == nil {
		t.Fatal("Get returned nil")
	}
}

func TestPool_PutResetsState(t *testing.T) {
	p := pool.New(func() *buf { return &buf{} })

	b := p.Get()
	b.data = append(b.data, 1, 2, 3)
	b.count = 42
	b.name = "test"

	p.Put(b)

	if b.count != 0 {
		t.Errorf("count after Put: want 0, got %d", b.count)
	}
	if b.name != "" {
		t.Errorf("name after Put: want \"\", got %q", b.name)
	}
	if len(b.data) != 0 {
		t.Errorf("data after Put: want len 0, got %d", len(b.data))
	}
	if cap(b.data) == 0 {
		t.Error("capacity should be preserved after Reset")
	}
}

func TestPool_GetAfterPutReturnsReset(t *testing.T) {
	p := pool.New(func() *buf { return &buf{} })

	b := p.Get()
	b.count = 99
	b.name = "dirty"
	p.Put(b)

	b2 := p.Get()
	if b2.count != 0 || b2.name != "" {
		t.Errorf("got dirty object from pool: count=%d name=%q", b2.count, b2.name)
	}
}

func TestPool_FactoryCalledWhenEmpty(t *testing.T) {
	calls := 0
	p := pool.New(func() *buf {
		calls++
		return &buf{}
	})

	_ = p.Get()
	_ = p.Get()

	if calls < 1 {
		t.Error("factory was never called")
	}
}

func TestPool_WorksWithDifferentTypes(t *testing.T) {
	p := pool.New(func() *entry { return &entry{keys: make([]string, 0, 4)} })

	e := p.Get()
	e.keys = append(e.keys, "a", "b")
	e.size = 10
	p.Put(e)

	if e.size != 0 || len(e.keys) != 0 {
		t.Error("entry not reset after Put")
	}
	if cap(e.keys) == 0 {
		t.Error("slice capacity should be preserved")
	}
}
