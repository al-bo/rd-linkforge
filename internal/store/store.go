package store

import (
	"errors"
	"sort"

	"github.com/skskuzan/rd-linkforge/internal/link"
)

var ErrDuplicateID = errors.New("store: error duplicate id")

type Store struct {
	byID map[uint64]link.Link
}

func New() Store {
	return Store{byID: make(map[uint64]link.Link)}
}

func Add(s Store, l link.Link) error {
	_, present := s.byID[l.ID]

	if present {
		return ErrDuplicateID
	} else {
		s.byID[l.ID] = l
	}

	return nil
}

func Get(s Store, id uint64) (link.Link, bool) {
	i, ok := s.byID[id]

	return i, ok
}

func All(s Store) []link.Link {
	ids := make([]uint64, 0, len(s.byID))
	for id := range s.byID {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		return ids[i] < ids[j]
	})

	result := make([]link.Link, 0, len(s.byID))
	for _, id := range ids {
		result = append(result, s.byID[id])
	}

	return result
}

func Count(s Store) int {
	return len(s.byID)
}
