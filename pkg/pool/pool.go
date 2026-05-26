// Package pool предоставляет обобщённый пул объектов, поддерживающих метод Reset().
//
// Пул построен поверх [sync.Pool] и автоматически сбрасывает состояние объекта
// перед его возвратом в пул через [Pool.Put].
//
// # Пример использования
//
//	type Buffer struct {
//	    data []byte
//	}
//	func (b *Buffer) Reset() { b.data = b.data[:0] }
//
//	p := pool.New(func() *Buffer { return &Buffer{data: make([]byte, 0, 64)} })
//	buf := p.Get()
//	buf.data = append(buf.data, "hello"...)
//	p.Put(buf) // вызывает buf.Reset() автоматически
package pool

import "sync"

// Resetter — ограничение для типов, которые могут быть помещены в пул.
// Тип должен реализовывать метод Reset(), очищающий состояние объекта.
type Resetter interface {
	Reset()
}

// Pool — обобщённый пул объектов типа T.
// T должен реализовывать интерфейс [Resetter].
// При возврате объекта в пул автоматически вызывается T.Reset().
type Pool[T Resetter] struct {
	p sync.Pool
}

// New создаёт и возвращает указатель на новый Pool.
// Параметр factory вызывается для создания нового объекта, когда пул пуст.
func New[T Resetter](factory func() T) *Pool[T] {
	return &Pool[T]{
		p: sync.Pool{
			New: func() any {
				return factory()
			},
		},
	}
}

// Get возвращает объект из пула. Если пул пуст, создаётся новый объект
// с помощью фабричной функции, переданной в [New].
func (p *Pool[T]) Get() T {
	return p.p.Get().(T)
}

// Put сбрасывает состояние объекта через Reset() и возвращает его в пул.
func (p *Pool[T]) Put(v T) {
	v.Reset()
	p.p.Put(v)
}
