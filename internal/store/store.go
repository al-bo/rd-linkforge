package store

import (
	"context"
	"errors"
	"slices"

	"github.com/al-bo/rd-linkforge/internal/link"
)

var ErrDuplicateID = errors.New("store: error duplicate id")
var _ link.Store = (*Store)(nil)

type Store struct {
	byID   map[uint64]link.Link
	byCode map[string]link.Link
}

func New() *Store {
	return &Store{
		byID:   make(map[uint64]link.Link),
		byCode: make(map[string]link.Link),
	}
}

func (s *Store) Create(ctx context.Context, l link.Link) (link.Link, error) {
	_, presentID := s.byID[l.ID]
	_, presentCode := s.byCode[l.Code]

	if presentID {
		return link.Link{}, ErrDuplicateID
	} else if presentCode {
		return link.Link{}, link.ErrAliasTaken
	} else {
		s.byID[l.ID] = l
		s.byCode[l.Code] = l
	}

	return l, nil
}

func (s *Store) Get(id uint64) (link.Link, bool) {
	i, ok := s.byID[id]

	return i, ok
}

func (s *Store) All() []link.Link {
	ids := make([]uint64, 0, len(s.byID))
	for id := range s.byID {
		ids = append(ids, id)
	}
	slices.Sort(ids)

	result := make([]link.Link, 0, len(s.byID))
	for _, id := range ids {
		result = append(result, s.byID[id])
	}

	return result
}

func (s *Store) Count() int {
	return len(s.byID)
}

func (s *Store) ByCode(ctx context.Context, code string) (link.Link, error) {
	l, ok := s.byCode[code]
	if !ok {
		return link.Link{}, link.ErrNotFound
	}

	return l, nil
}

func (s *Store) ListByOwner(ctx context.Context, ownerID string, page link.Page) ([]link.Link, error) {
	p := page.Normalize()
	afterID, err := p.AfterID()
	if err != nil {
		return nil, err
	}

	var ids []uint64
	for id, l := range s.byID {
		if l.OwnerID == ownerID && id > afterID {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)

	if len(ids) == 0 {
		return nil, nil
	}
	if len(ids) > p.Limit {
		ids = ids[:p.Limit]
	}

	links := make([]link.Link, 0, len(ids))
	for _, id := range ids {
		links = append(links, s.byID[id])
	}
	return links, nil
}

func (s *Store) Delete(ctx context.Context, id uint64) error {
	l, ok := s.Get(id)

	if !ok {
		return link.ErrNotFound
	}

	delete(s.byID, id)
	delete(s.byCode, l.Code)

	return nil
}
