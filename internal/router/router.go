package router

import (
	"mezzani_backend/internal/handler"
	"mezzani_backend/internal/middleware"
	"mezzani_backend/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

func SetupRouter(
	db *pgxpool.Pool,

	// Handlers
	authHandler *handler.AuthHandler,
	tableSessionHandler *handler.TableSessionHandler,
	customerHandler *handler.CustomerSessionHandler,
	cartHandler *handler.SharedCartHandler,
	orderHandler *handler.OrderHandler,
	wsHandler *handler.WSHandler,
	menuHandler *handler.MenuHandler,
) *gin.Engine {

	r := gin.Default()

	// JWT secret (should come from env variable)
	jwtSecret := []byte("super-secret-key")

	// ========== PUBLIC ROUTES (No Auth) ==========
	public := r.Group("/api")
	{
		// Auth
		public.POST("/auth/login", authHandler.Login)

		// Public menu viewing (customers can view menu without login)
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
		// ===== TABLE SESSION MANAGEMENT (Waiters/Staff) =====
		table := protected.Group("/table-session")
		{
			table.POST("/start", tableSessionHandler.StartSession)
			table.POST("/:id/close", tableSessionHandler.CloseSession)
			table.POST("/heartbeat", tableSessionHandler.Heartbeat)
		}

		// ===== CUSTOMER MANAGEMENT (All staff) =====
		customer := protected.Group("/customer")
		{
			customer.POST("/join", customerHandler.JoinTable)
			customer.GET("/table/:table_session_id", customerHandler.ListCustomers)
			customer.GET("/:id", customerHandler.GetCustomer)
		}

		// ===== CART OPERATIONS (Waiters/Customers) =====
		cart := protected.Group("/cart")
		{
			cart.POST("/create", cartHandler.CreateCart)
			cart.POST("/join", cartHandler.JoinCart)
			cart.POST("/add-item", cartHandler.AddItem)
		}

		// ===== ORDER MANAGEMENT =====
		orders := protected.Group("/orders")
		{
			// Waiters can create orders

			orders.POST("/submit",
				middleware.RequirePermission("create_orders"),
				orderHandler.SubmitCart,
			)

			// Kitchen staff can update status
			orders.PATCH("/:id/status",
				middleware.RequirePermission("update_order_status"),
				orderHandler.UpdateStatus,
			)

			billingService := service.NewBillingService(queries)
			billingHandler := handler.NewBillingHandler(billingService)

			api.POST(
				"/billing/close",
				middleware.RequirePermission("close_bill"),
				billingHandler.CloseBill,
			)
		}

		// ===== MENU MANAGEMENT (Managers/Owners only) =====
		menu := protected.Group("/menu")
		menu.Use(middleware.RequirePermission("manage_menu"))
		{
			// Menu CRUD
			menu.POST("/", menuHandler.CreateMenu)

			// Categories
			menu.POST("/categories", menuHandler.CreateCategory)

			// Items
			menu.POST("/items", menuHandler.CreateMenuItem)

			// Updates
			menu.PATCH("/items/:id/price", menuHandler.UpdatePrice)
			menu.PATCH("/items/:id/sold-out", menuHandler.SetItemSoldOut)
			menu.PATCH("/items/:id/available", menuHandler.SetItemAvailable)
			menu.PATCH("/items/:id/special", menuHandler.SetDailySpecial)
		}

		// ===== KITCHEN DISPLAY (Kitchen staff) =====
		kitchen := protected.Group("/kitchen")
		kitchen.Use(middleware.RequirePermission("view_kitchen_display"))
		{
			//update orde status
			kitchen.GET("/orders", orderHandler.UpdateStatus)
			// Add more kitchen endpoints as needed
		}

		// ===== REPORTING (Managers/Owners only) =====
		reports := protected.Group("/reports")
		reports.Use(middleware.RequirePermission("view_reports"))
		{
			// Add reporting endpoints here
			// reports.GET("/sales", reportHandler.GetSalesReport)
			// reports.GET("/popular-items", reportHandler.GetPopularItems)
		}
	}

	return r
}
