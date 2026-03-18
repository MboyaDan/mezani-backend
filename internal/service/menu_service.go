package service

import (
	"context"

	db "mezzani_backend/internal/database/sqlc"

	"github.com/google/uuid"
)

type MenuService struct {
	Queries *db.Queries
}

func NewMenuService(q *db.Queries) *MenuService {
	return &MenuService{Queries: q}
}

//
// CREATE MENU (per branch)
//

func (s *MenuService) CreateMenu(
	ctx context.Context,
	branchID uuid.UUID,
) (db.Menu, error) {

	return s.Queries.CreateMenu(ctx, db.CreateMenuParams{
		ID:       uuid.New(),
		BranchID: branchID,
	})
}

//
// GET MENUS BY BRANCH
//

func (s *MenuService) GetBranchMenus(
	ctx context.Context,
	branchID uuid.UUID,
) ([]db.Menu, error) {

	return s.Queries.GetBranchMenus(ctx, branchID)
}

//
// CREATE MENU CATEGORY
//

func (s *MenuService) CreateCategory(
	ctx context.Context,
	menuID uuid.UUID,
	name string,
	order int32,
) (db.MenuCategory, error) {

	return s.Queries.CreateMenuCategory(ctx, db.CreateMenuCategoryParams{
		ID:           uuid.New(),
		MenuID:       menuID,
		Name:         name,
		DisplayOrder: order,
	})
}

//
// GET MENU CATEGORIES
//

func (s *MenuService) GetMenuCategories(
	ctx context.Context,
	menuID uuid.UUID,
) ([]db.MenuCategory, error) {

	return s.Queries.GetMenuCategories(ctx, menuID)
}

//
// ADD MENU ITEM
//

func (s *MenuService) AddMenuItem(
	ctx context.Context,
	categoryID uuid.UUID,
	name string,
	description string,
	price float64,
) (db.MenuItem, error) {

	return s.Queries.CreateMenuItem(ctx, db.CreateMenuItemParams{
		ID:          uuid.New(),
		CategoryID:  categoryID,
		Name:        name,
		Description: description,
		Price:       price,
	})
}

//
// GET CATEGORY ITEMS
//

func (s *MenuService) GetCategoryItems(
	ctx context.Context,
	categoryID uuid.UUID,
) ([]db.MenuItem, error) {

	return s.Queries.GetCategoryItems(ctx, categoryID)
}

//
// UPDATE MENU ITEM PRICE
//

func (s *MenuService) UpdateMenuItemPrice(
	ctx context.Context,
	id uuid.UUID,
	price float64,
) error {

	return s.Queries.UpdateMenuItemPrice(ctx, db.UpdateMenuItemPriceParams{
		ID:    id,
		Price: price,
	})
}

//
// SET ITEM SOLD OUT
//

func (s *MenuService) SetMenuItemSoldOut(
	ctx context.Context,
	id uuid.UUID,
	soldOut bool,
) error {

	return s.Queries.SetMenuItemSoldOut(ctx, db.SetMenuItemSoldOutParams{
		ID:      id,
		SoldOut: soldOut,
	})
}

//
// SET ITEM AVAILABILITY
//

func (s *MenuService) SetMenuItemAvailability(
	ctx context.Context,
	id uuid.UUID,
	available bool,
) error {

	return s.Queries.SetMenuItemAvailability(ctx, db.SetMenuItemAvailabilityParams{
		ID:        id,
		Available: available,
	})
}

//
// SET DAILY SPECIAL
//

func (s *MenuService) SetDailySpecial(
	ctx context.Context,
	id uuid.UUID,
	isSpecial bool,
) error {

	return s.Queries.SetDailySpecial(ctx, db.SetDailySpecialParams{
		ID:        id,
		IsSpecial: isSpecial,
	})
}

func (s *MenuService) GetFullMenu(
	ctx context.Context,
	menuID uuid.UUID,
) ([]db.GetFullMenuRow, error) {

	return s.Queries.GetFullMenu(ctx, menuID)
}
