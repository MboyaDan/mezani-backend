package router

import (
	"context"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"mezzani_backend/internal/handler"
	"mezzani_backend/internal/middleware"
	"mezzani_backend/internal/notifications"
	"mezzani_backend/internal/service"
)

func SetupRouter(
	jwtSecret []byte,
	allowedOrigins []string,
	branchService *service.BranchService,
	alerts *notifications.AlertService,

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
	passwordResetHandler *handler.PasswordResetHandler,
) *gin.Engine {

	r := gin.New()

	// ========== GLOBAL MIDDLEWARE ==========
	r.Use(middleware.RecoveryWithAlerts(alerts))
	r.Use(middleware.Logger())
	r.Use(cors.New(cors.Config{
		AllowOrigins: allowedOrigins,
		AllowMethods: []string{"GET", "POST", "PATCH", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders: []string{
			"Origin",
			"Content-Type",
			"Authorization",
			"Accept",
		},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// ----- WebSocket -----
	// Registered in the public group so CORS middleware applies.
	// The handler performs its own connection-level auth.
	r.GET("/ws/kitchen", wsHandler.HandleWS)
	// ========== PUBLIC ROUTES (No Auth Required) ==========

	public := r.Group("/api")
	{
		// ----- Auth -----
		public.POST("/auth/register-owner",
			middleware.StrictRateLimit(),
			authHandler.RegisterOwner,
		)
		public.POST("/auth/login",
			middleware.StrictRateLimit(),
			authHandler.Login,
		)
		public.POST("/auth/refresh", authHandler.RefreshToken)
		public.POST("/auth/logout", authHandler.Logout)
		public.POST("/auth/forgot-password",
			middleware.StrictRateLimit(),
			passwordResetHandler.ForgotPassword,
		)
		public.POST("/auth/reset-password",
			middleware.StrictRateLimit(),
			passwordResetHandler.ResetPassword,
		)

		// ----- Public Menu Viewing -----
		public.GET("/menus/:menu_id/full", middleware.RelaxedRateLimit(), menuHandler.GetFullMenu)
		public.GET("/menus/branch/:branch_id", middleware.RelaxedRateLimit(), menuHandler.GetBranchMenus)
		public.GET("/menus/:menu_id/categories", middleware.RelaxedRateLimit(), menuHandler.GetMenuCategories)
		public.GET("/categories/:category_id/items", middleware.RelaxedRateLimit(), menuHandler.GetCategoryItems)
		public.GET("/table-sessions/:session_id/menu", middleware.RelaxedRateLimit(), menuHandler.GetMenuBySession)
		public.GET("/table-sessions/:session_id/info", middleware.RelaxedRateLimit(), menuHandler.GetSessionInfo)

		// ----- Customer Flow (QR scan — no JWT) -----
		// These are intentionally public. Customers join via QR code links.
		public.POST("/customer/join", middleware.ModerateRateLimit(), customerHandler.JoinTable)
		public.GET("/customer/table/:table_session_id", middleware.ModerateRateLimit(), customerHandler.ListCustomers)
		public.GET("/customer/:id", middleware.ModerateRateLimit(), customerHandler.GetCustomer)
		public.POST("/cart/create", middleware.ModerateRateLimit(), cartHandler.CreateCart)
		public.POST("/cart/join", middleware.ModerateRateLimit(), cartHandler.JoinCart)
		public.POST("/cart/add-item", middleware.ModerateRateLimit(), cartHandler.AddItem)
		public.POST("/orders/submit", middleware.ModerateRateLimit(), orderHandler.SubmitCart)

	}

	// ========== PROTECTED ROUTES (JWT Required) ==========
	protected := r.Group("/api")
	protected.Use(middleware.AuthMiddleware(jwtSecret))
	protected.Use(middleware.RelaxedRateLimit())
	{
		// ----- Table Session Management -----
		table := protected.Group("/table-session")
		{
			table.POST("/start", tableSessionHandler.StartSession)
			table.POST("/:id/close", tableSessionHandler.CloseSession)
			table.POST("/heartbeat", tableSessionHandler.Heartbeat)
		}

		// ----- Order Management -----
		// Kitchen display uses the same status update — one route, one permission.
		orders := protected.Group("/orders")
		{
			orders.PATCH("/:id/status",
				middleware.RequirePermission("update_order_status"),
				orderHandler.UpdateStatus,
			)
			orders.GET("/recent", orderHandler.GetRecentOrders)
		}

		// ----- Kitchen Display -----
		kitchen := protected.Group("/kitchen")
		kitchen.Use(middleware.RequirePermission("view_kitchen_display"))
		{

			//I will replace with a proper ListKitchenOrders handler when available
			kitchen.PATCH("/orders/:id/status", orderHandler.UpdateStatus)
		}

		// ----- Billing -----
		billing := protected.Group("/billing")
		{
			billing.POST("/close",
				middleware.RequirePermission("close_bill"),
				billingHandler.CloseBill,
			)
		}

		// ----- Menu Management -----
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
		}

		// ----- Inventory Management -----
		inventory := protected.Group("/branches/:branch_id/inventory")
		inventory.Use(middleware.RequirePermission("manage_inventory"))
		inventory.Use(middleware.TenantBranchGuard("branch_id", func(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
			branch, err := branchService.GetBranchByID(ctx, id)
			return branch.TenantID, err
		}))
		{
			inventory.POST("/", inventoryHandler.CreateItem)
			inventory.GET("/", inventoryHandler.ListItems)
			inventory.GET("/:item_id", inventoryHandler.GetItem)
			inventory.PATCH("/low-stock", inventoryHandler.LowStockAlerts)
			inventory.PATCH("/:item_id/stock", inventoryHandler.SetStock)
		}

		// ----- Reporting -----
		reports := protected.Group("/reports")
		reports.Use(middleware.RequirePermission("view_reports"))
		{
			reports.GET("/analytics/dashboard", analyticsHandler.Dashboard)
		}

		// ----- Staff Management -----
		staff := protected.Group("/staff")
		staff.Use(middleware.RequirePermission("manage_staff"))
		{
			staff.POST("/create", staffHandler.CreateStaff)
		}

		// ----- Owner — Branches & Tables -----
		owner := protected.Group("/owner")
		owner.Use(middleware.RequirePermission("manage_branches"))
		{
			owner.POST("/branches", branchHandler.CreateBranch)
			owner.GET("/branches", branchHandler.ListBranches)
			owner.POST("/branches/:id/tables",
				middleware.TenantBranchGuard("id", func(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
					branch, err := branchService.GetBranchByID(ctx, id)
					return branch.TenantID, err
				}),
				tableHandler.CreateTable,
			)
		}
	}

	return r
}
