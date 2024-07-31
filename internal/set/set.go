package set

// Set is a set of values of type T.
type Set[T comparable] struct {
	members map[T]struct{}
}

func (s *Set[T]) Add(v T) {
	if s.members == nil {
		s.members = map[T]struct{}{}
	}
	s.members[v] = struct{}{}
}

func (s *Set[T]) Remove(v T) {
	delete(s.members, v)
}

func (s *Set[T]) AddSet(src Immutable[T]) {
	for v := range src.Elements() {
		s.Add(v)
	}
}

func (s *Set[T]) RemoveSet(src Immutable[T]) {
	for v := range src.Elements() {
		s.Remove(v)
	}
}

func (s *Set[T]) LenX() int {
	return len(s.members)
}

func (s *Set[T]) IsEmpty() bool {
	return len(s.members) == 0
}

func (s *Set[T]) Elements() map[T]struct{} {
	return s.members
}

type Immutable[T comparable] interface {
	Elements() map[T]struct{}
}
