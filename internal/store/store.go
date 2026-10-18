package store

import (
	"context"
	"errors"
	"sort"

	"github.com/skskuzan/rd-linkforge/internal/link"
)

var ErrDuplicateID = errors.New("store: error duplicate id")
var _ link.Store = (*MemoryStore)(nil)

type MemoryStore struct {
	byID   map[uint64]link.Link
	byCode map[string]link.Link
}

func New() *MemoryStore {
	return &MemoryStore{byID: make(map[uint64]link.Link)}
}

func (s *MemoryStore) Create(ctx context.Context, l link.Link) (link.Link, error) {
	_, presentID := s.byID[l.ID]
	_, presentCode := s.byCode[l.Code]

	if presentID {
		return link.Link{}, ErrDuplicateID
	} else if presentCode {
		return link.Link{}, link.ErrAliasTaken
	} else {
		s.byID[l.ID] = l
	}

	return l, nil
}

func (s *MemoryStore) Get(id uint64) (link.Link, bool) {
	i, ok := s.byID[id]

	return i, ok
}

func (s *MemoryStore) All() []link.Link {
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

func (s *MemoryStore) Count() int {
	return len(s.byID)
}

func (s *MemoryStore) ByCode(ctx context.Context, code string) (link.Link, error) {
	l, ok := s.byCode[code]
	if !ok {
		return link.Link{}, link.ErrNotFound
	}

	return l, nil
}

func (s *MemoryStore) ListByOwner(ctx context.Context, owner string, page link.Page) ([]link.Link, error) {
	var r []link.Link
	n := page.Normalize()
	pid, afterError := n.AfterID()

	if afterError != nil {
		return r, afterError
	}

	sorted := s.All()

	for _, v := range sorted {
		if v.OwnerID == owner && v.ID > pid {
			r = append(r, v)
		}

		if len(r) == n.Limit {
			break
		}
	}

	return r, nil
}

func (s *MemoryStore) Delete(ctx context.Context, id uint64) error {
	l, ok := s.Get(id)

	if !ok {
		return link.ErrNotFound
	}

	delete(s.byID, id)
	delete(s.byCode, l.Code)

	return nil
}
