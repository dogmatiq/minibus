package minibus

import (
	"reflect"
	"sync"
)

type subscriptions struct {
	m         sync.Mutex
	functions map[*function]map[reflect.Type]struct{}
	types     map[reflect.Type]*subscriptionsForType
}

// subscriptionsForType is a collection of the functions that subscribe to a
// particular message type.
type subscriptionsForType struct {
	Members map[*function]struct{}

	// IsFinalized is set to true once the subscribers set has been updated to
	// include functions that receive this message type because they subscribe
	// to an interface that it implements, as opposed to subscribing to the
	// concrete message type directly.
	IsFinalized bool
}

func (s *subscriptions) Add(fn *function, t reflect.Type) {
	s.m.Lock()
	defer s.m.Unlock()

	if s.functions == nil {
		s.functions = map[*function]map[reflect.Type]struct{}{}
	}

	subs := s.forType(t)
	subs.Members[fn] = struct{}{}

	types, ok := s.functions[fn]
	if !ok {
		types = map[reflect.Type]struct{}{}
		s.functions[fn] = types
	}

	types[t] = struct{}{}
}

func (s *subscriptions) Remove(fn *function) {
	s.m.Lock()
	defer s.m.Unlock()

	for t := range s.functions[fn] {
		subs := s.forType(t)
		delete(subs.Members, fn)
	}

	delete(s.functions, fn)
}

func (s *subscriptions) Subscribers(t reflect.Type) map[*function]struct{} {
	s.m.Lock()
	defer s.m.Unlock()

	subs := s.forType(t)

	if !subs.IsFinalized {
		for subscribedType, subscribers := range s.types {
			if subscribedType.Kind() == reflect.Interface && t.Implements(subscribedType) {
				for f := range subscribers.Members {
					subs.Members[f] = struct{}{}
					s.functions[f][t] = struct{}{}
				}
			}
		}
		subs.IsFinalized = true
	}

	return subs.Members
}

func (s *subscriptions) forType(t reflect.Type) *subscriptionsForType {
	subs, ok := s.types[t]

	if !ok {
		subs = &subscriptionsForType{
			Members: map[*function]struct{}{},
		}

		if s.types == nil {
			s.types = map[reflect.Type]*subscriptionsForType{}
		}

		s.types[t] = subs
	}

	return subs
}
