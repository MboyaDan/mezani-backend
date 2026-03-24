package router

import (
	"mezzani_backend/internal/handler"
	"mezzani_backend/internal/middleware"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

func SetupRouter(
	db *pgxpool.Pool,
	jwtSecret []byte,

	// Handlers
	authHandler *handler.AuthHandler,
	tableSessionHandler *handler.TableSessionHandler,
	customerHandler *handler.CustomerSessionHandler,
	cartHandler *handler.SharedCartHandler,
	orderHandler *handler.OrderHandler,
	wsHandler *handler.WSHandler,
	menuHandler *handler.MenuHandler,
	billingHandler *handler.BillingHandler,
	analyticsHandler *handler.AnalyticsHandler,
	staffHandler *handler.StaffHandler,
	tableHandler *handler.TableHandler,
	branchHandler *handler.BranchHandler,
	inventoryHandler *handler.InventoryHandler,
) *gin.Engine {

	r := gin.Default()

	// ========== PUBLIC ROUTES (No Auth) ==========
	public := r.Group("/api")
	{
		public.POST("/auth/register-owner", authHandler.RegisterOwner)
		public.POST("/auth/login", authHandler.Login)

		// Public menu viewing
		public.GET("/menus/:menu_id/full", menuHandler.GetFullMenu)
		public.GET("/menus/branch/:branch_id", menuHandler.GetBranchMenus)
		public.GET("/menus/:menu_id/categories", menuHandler.GetMenuCategories)
		public.GET("/categories/:category_id/items", menuHandler.GetCategoryItems)

		// WebSocket (handles its own auth via connection)
		r.GET("/ws/kitchen", wsHandler.HandleWS)
	}

	// ========== PROTECTED ROUTES (Auth Required) ==========
	protected := r.Group("/api")
	protected.Use(middleware.AuthMiddleware(jwtSecret))
	{
		// ===== TABLE SESSION MANAGEMENT =====
		table := protected.Group("/table-session")
		{
			table.POST("/start", tableSessionHandler.StartSession)
			table.POST("/:id/close", tableSessionHandler.CloseSession)
			table.POST("/heartbeat", tableSessionHandler.Heartbeat)
		}
		// ===== CUSTOMER FLOW (Public - QR scan, no JWT) =====
		public.POST("/customer/join", customerHandler.JoinTable)
		public.GET("/customer/table/:table_session_id", customerHandler.ListCustomers)
		public.GET("/customer/:id", customerHandler.GetCustomer)
		public.POST("/cart/create", cartHandler.CreateCart)
		public.POST("/cart/join", cartHandler.JoinCart)
		public.POST("/cart/add-item", cartHandler.AddItem)
		public.POST("/orders/submit", orderHandler.SubmitCart)

		// ===== ORDER MANAGEMENT =====
		orders := protected.Group("/orders")
		{
			orders.PATCH("/:id/status",
				middleware.RequirePermission("update_order_status"),
				orderHandler.UpdateStatus,
			)
		}

		// ===== BILLING =====
		billing := protected.Group("/billing")
		{
			billing.POST("/close",
				middleware.RequirePermission("close_bill"),
				billingHandler.CloseBill,
			)
		}

		// ===== MENU MANAGEMENT =====
		menu := protected.Group("/menu")
		menu.Use(middleware.RequirePermission("manage_menu"))
		{
			menu.POST("/", menuHandler.CreateMenu)
			menu.POST("/categories", menuHandler.CreateCategory)
			menu.POST("/items", menuHandler.CreateMenuItem)
			menu.PATCH("/items/:id/price", menuHandler.UpdatePrice)
			menu.PATCH("/items/:id/sold-out", menuHandler.SetItemSoldOut)
			menu.PATCH("/items/:id/available", menuHandler.SetItemAvailable)
			menu.PATCH("/items/:id/special", menuHandler.SetDailySpecial)
			//menu.PATCH("/items/:id/regular", menuHandler.UnsetDailySpecial)
		}

		// ===== INVENTORY MANAGEMENT =====
		inventory := protected.Group("/branches/:branch_id/inventory")
		inventory.Use(middleware.RequirePermission("manage_inventory"))
		{
			inventory.POST("/", inventoryHandler.CreateItem)
			inventory.GET("/", inventoryHandler.ListItems)
			inventory.GET("/:item_id", inventoryHandler.GetItem)
			inventory.PATCH("/low-stock", inventoryHandler.LowStockAlerts)
			inventory.PATCH("/:item_id/stock", inventoryHandler.SetStock)

		}

		// ===== KITCHEN DISPLAY =====
		kitchen := protected.Group("/kitchen")
		kitchen.Use(middleware.RequirePermission("view_kitchen_display"))
		{
			kitchen.PATCH("/orders/:id/status", orderHandler.UpdateStatus)
		}

		// ===== REPORTING =====
		reports := protected.Group("/reports")
		reports.Use(middleware.RequirePermission("view_reports"))
		{
			reports.GET("/analytics/dashboard", analyticsHandler.Dashboard)
		}

		// ===== STAFF MANAGEMENT =====
		staff := protected.Group("/staff")
		staff.Use(middleware.RequirePermission("manage_staff"))
		{
			staff.POST("/create", staffHandler.CreateStaff)
		}

		// ===== OWNER — BRANCHES & TABLES =====
		owner := protected.Group("/owner")
		owner.Use(middleware.RequirePermission("manage_branches"))
		{
			owner.POST("/branches", branchHandler.CreateBranch)
			owner.POST("/branches/:id/tables", tableHandler.CreateTable)
		}
	}

	return r
}
