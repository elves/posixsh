package eval

func each[X any, Y any](f func(X) Y, xs []X) []Y {
	ys := make([]Y, len(xs))
	for i, x := range xs {
		ys[i] = f(x)
	}
	return ys
}

type set[T comparable] map[T]struct{}

func (s set[T]) add(v T) { s[v] = struct{}{} }

func (s set[T]) has(v T) bool {
	_, ok := s[v]
	return ok
}
