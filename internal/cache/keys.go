package cache

import "fmt"

const (
	TTLMenu   = 10 * 60 * 1000000000 // 10 minutes
	TTLBranch = 30 * 60 * 1000000000 // 30 minutes
)

func KeyFullMenu(menuID string) string {
	return fmt.Sprintf("menu:full:%s", menuID)
}

func KeyBranchMenus(branchID string) string {
	return fmt.Sprintf("menu:branch:%s", branchID)
}

func KeyMenuCategories(menuID string) string {
	return fmt.Sprintf("menu:categories:%s", menuID)
}

func KeyCategoryItems(categoryID string) string {
	return fmt.Sprintf("menu:items:%s", categoryID)
}
