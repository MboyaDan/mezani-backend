package service

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"

	"mezzani_backend/internal/cache"
	db "mezzani_backend/internal/database/sqlc"

	"github.com/google/uuid"
)

type MenuService struct {
	Queries *db.Queries
	Cache   *cache.Cache
}

func NewMenuService(q *db.Queries, c *cache.Cache) *MenuService {
	return &MenuService{Queries: q, Cache: c}
}

func (s *MenuService) CreateMenu(ctx context.Context, branchID uuid.UUID) (db.Menu, error) {
	menu, err := s.Queries.CreateMenu(ctx, db.CreateMenuParams{
		ID:       uuid.New(),
		BranchID: branchID,
	})
	if err != nil {
		return db.Menu{}, err
	}
	// Invalidate branch menus cache
	s.Cache.Delete(ctx, cache.KeyBranchMenus(branchID.String()))
	return menu, nil
}

func (s *MenuService) GetBranchMenus(ctx context.Context, branchID uuid.UUID) ([]db.Menu, error) {
	key := cache.KeyBranchMenus(branchID.String())

	var menus []db.Menu
	err := s.Cache.Get(ctx, key, &menus)
	if err == nil {
		return menus, nil // cache hit
	}
	if !errors.Is(err, redis.Nil) {
		// log cache error but don't fail — fall through to DB
	}

	menus, err = s.Queries.GetBranchMenus(ctx, branchID)
	if err != nil {
		return nil, err
	}

	s.Cache.Set(ctx, key, menus, 30*time.Minute)
	return menus, nil
}

func (s *MenuService) CreateCategory(ctx context.Context, menuID uuid.UUID, name string, order int32) (db.MenuCategory, error) {
	category, err := s.Queries.CreateMenuCategory(ctx, db.CreateMenuCategoryParams{
		ID:           uuid.New(),
		MenuID:       menuID,
		Name:         name,
		DisplayOrder: order,
	})
	if err != nil {
		return db.MenuCategory{}, err
	}
	// Invalidate menu categories cache
	s.Cache.Delete(ctx, cache.KeyMenuCategories(menuID.String()))
	return category, nil
}

func (s *MenuService) GetMenuCategories(ctx context.Context, menuID uuid.UUID) ([]db.MenuCategory, error) {
	key := cache.KeyMenuCategories(menuID.String())

	var categories []db.MenuCategory
	err := s.Cache.Get(ctx, key, &categories)
	if err == nil {
		return categories, nil
	}

	categories, err = s.Queries.GetMenuCategories(ctx, menuID)
	if err != nil {
		return nil, err
	}

	s.Cache.Set(ctx, key, categories, 10*time.Minute)
	return categories, nil
}

func (s *MenuService) AddMenuItem(ctx context.Context, categoryID uuid.UUID, name string, description string, price float64) (db.MenuItem, error) {
	item, err := s.Queries.CreateMenuItem(ctx, db.CreateMenuItemParams{
		ID:          uuid.New(),
		CategoryID:  categoryID,
		Name:        name,
		Description: description,
		Price:       price,
	})
	if err != nil {
		return db.MenuItem{}, err
	}
	// Invalidate category items cache
	s.Cache.Delete(ctx, cache.KeyCategoryItems(categoryID.String()))
	return item, nil
}

func (s *MenuService) GetCategoryItems(ctx context.Context, categoryID uuid.UUID) ([]db.MenuItem, error) {
	key := cache.KeyCategoryItems(categoryID.String())

	var items []db.MenuItem
	err := s.Cache.Get(ctx, key, &items)
	if err == nil {
		return items, nil
	}

	items, err = s.Queries.GetCategoryItems(ctx, categoryID)
	if err != nil {
		return nil, err
	}

	s.Cache.Set(ctx, key, items, 10*time.Minute)
	return items, nil
}

func (s *MenuService) GetFullMenu(ctx context.Context, menuID uuid.UUID) ([]db.GetFullMenuRow, error) {
	key := cache.KeyFullMenu(menuID.String())

	var rows []db.GetFullMenuRow
	err := s.Cache.Get(ctx, key, &rows)
	if err == nil {
		return rows, nil
	}

	rows, err = s.Queries.GetFullMenu(ctx, menuID)
	if err != nil {
		return nil, err
	}

	s.Cache.Set(ctx, key, rows, 10*time.Minute)
	return rows, nil
}

func (s *MenuService) UpdateMenuItemPrice(ctx context.Context, id uuid.UUID, price float64) error {
	if err := s.Queries.UpdateMenuItemPrice(ctx, db.UpdateMenuItemPriceParams{ID: id, Price: price}); err != nil {
		return err
	}
	// Wipe all menu cache since we don't know which category/menu this item belongs to
	s.Cache.DeleteByPattern(ctx, "menu:*")
	return nil
}

func (s *MenuService) SetMenuItemSoldOut(ctx context.Context, id uuid.UUID, soldOut bool) error {
	if err := s.Queries.SetMenuItemSoldOut(ctx, db.SetMenuItemSoldOutParams{ID: id, SoldOut: soldOut}); err != nil {
		return err
	}
	s.Cache.DeleteByPattern(ctx, "menu:*")
	return nil
}

func (s *MenuService) SetMenuItemAvailability(ctx context.Context, id uuid.UUID, available bool) error {
	if err := s.Queries.SetMenuItemAvailability(ctx, db.SetMenuItemAvailabilityParams{ID: id, Available: available}); err != nil {
		return err
	}
	s.Cache.DeleteByPattern(ctx, "menu:*")
	return nil
}

func (s *MenuService) SetDailySpecial(ctx context.Context, id uuid.UUID, isSpecial bool) error {
	if err := s.Queries.SetDailySpecial(ctx, db.SetDailySpecialParams{ID: id, IsSpecial: isSpecial}); err != nil {
		return err
	}
	s.Cache.DeleteByPattern(ctx, "menu:*")
	return nil
}

func (s *MenuService) GetMenuBySession(ctx context.Context, sessionID uuid.UUID) ([]db.GetFullMenuRow, error) {

	sessionWithTable, err := s.Queries.GetTableSessionWithTable(ctx, sessionID)
	if err != nil {
		return nil, errors.New("session not found")
	}
	if sessionWithTable.Status != "active" {
		return nil, errors.New("table session is not active")
	}

	if sessionWithTable.ExpiresAt.Before(time.Now()) {
		return nil, errors.New("table session has expired")
	}

	menus, err := s.Queries.GetBranchMenus(ctx, sessionWithTable.BranchID)
	if err != nil || len(menus) == 0 {
		return nil, errors.New("no menu found for this branch")
	}

	return s.GetFullMenu(ctx, menus[0].ID)
}

type SessionInfo struct {
	TableNumber int32  `json:"table_number"`
	BranchID    string `json:"branch_id"`
	Status      string `json:"status"`
}

func (s *MenuService) GetSessionInfo(ctx context.Context, sessionID uuid.UUID) (SessionInfo, error) {
	row, err := s.Queries.GetTableSessionWithTable(ctx, sessionID)
	if err != nil {
		return SessionInfo{}, errors.New("session not found")
	}
	if row.Status != "active" {
		return SessionInfo{}, errors.New("session is not active")
	}
	return SessionInfo{
		TableNumber: row.TableNumber,
		BranchID:    row.BranchID.String(),
		Status:      row.Status,
	}, nil
}
