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
	paymentHandler *handler.PaymentHandler,
	analyticsHandler *handler.AnalyticsHandler,
	staffHandler *handler.StaffHandler,
	tableHandler *handler.TableHandler,
	branchHandler *handler.BranchHandler,
	inventoryHandler *handler.InventoryHandler,
	passwordResetHandler *handler.PasswordResetHandler,
	aiHandler *handler.AIHandler,
	superAdminHandler *handler.SuperAdminHandler,
	subscriptionHandler *handler.SubscriptionHandler,
	getTenantSubscriptionExpiry func(ctx context.Context, tenantID uuid.UUID) (time.Time, error),
) *gin.Engine {

	r := gin.New()

	// ========== HEALTH CHECK (no middleware, no auth) ==========
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

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
		public.POST("/superadmin/login",
			middleware.StrictRateLimit(),
			superAdminHandler.Login,
		)
		public.POST("/superadmin/refresh",
			middleware.StrictRateLimit(),
			superAdminHandler.Refresh,
		)

		// ----- Public Menu Viewing -----
		public.GET("/menus/:menu_id/full", middleware.RelaxedRateLimit(), menuHandler.GetFullMenu)
		public.GET("/menus/branch/:branch_id", middleware.RelaxedRateLimit(), menuHandler.GetBranchMenus)
		public.GET("/menus/:menu_id/categories", middleware.RelaxedRateLimit(), menuHandler.GetMenuCategories)
		public.GET("/categories/:category_id/items", middleware.RelaxedRateLimit(), menuHandler.GetCategoryItems)
		public.GET("/table-sessions/:session_id/menu", middleware.RelaxedRateLimit(), menuHandler.GetMenuBySession)
		public.GET("/table-sessions/:session_id/info", middleware.RelaxedRateLimit(), menuHandler.GetSessionInfo)
		public.GET("/tables/:table_id/session-status", middleware.RelaxedRateLimit(), menuHandler.CheckSessionStatus)

		// ----- Customer Flow (QR scan — no JWT) -----
		public.POST("/customer/join", middleware.ModerateRateLimit(), customerHandler.JoinTable)
		public.GET("/customer/table/:table_session_id", middleware.ModerateRateLimit(), customerHandler.ListCustomers)
		public.GET("/customer/:id", middleware.ModerateRateLimit(), customerHandler.GetCustomer)
		public.POST("/cart/create", middleware.ModerateRateLimit(), cartHandler.CreateCart)
		public.POST("/cart/join", middleware.ModerateRateLimit(), cartHandler.JoinCart)
		public.POST("/cart/add-item", middleware.ModerateRateLimit(), cartHandler.AddItem)
		public.POST("/orders/submit", middleware.ModerateRateLimit(), orderHandler.SubmitCart)

		// ----- Paystack webhook (authenticated via HMAC signature, not JWT) -----
		public.POST("/webhooks/paystack", subscriptionHandler.Webhook)
	}

	// ========== PROTECTED ROUTES (JWT Required) ==========
	protected := r.Group("/api")
	protected.Use(middleware.AuthMiddleware(jwtSecret))
	protected.Use(middleware.RelaxedRateLimit())
	protected.Use(middleware.SubscriptionGuard(getTenantSubscriptionExpiry))
	{
		// ----- Manager — Tables (view + manage, no add) -----
		manager := protected.Group("/manager")
		manager.Use(middleware.RequirePermission("manage_tables"))
		{
			manager.GET("/tables", tableHandler.GetTablesWithSessions)
		}

		// ----- Waiter — Tables -----
		waiter := protected.Group("/waiter")
		waiter.Use(middleware.RequirePermission("create_orders"))
		{
			waiter.GET("/tables", tableHandler.GetTablesWithSessions)
		}

		// ----- Table Session Management -----
		table := protected.Group("/table-session")
		{
			table.POST("/start", tableSessionHandler.StartSession)
			table.POST("/:id/close", tableSessionHandler.CloseSession)
			table.POST("/heartbeat", tableSessionHandler.Heartbeat)
		}

		// ----- Order Management -----
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

		// ----- Payments -----
		payments := protected.Group("/payments")
		{
			payments.POST("/cash",
				middleware.RequirePermission("initiate_payment"),
				paymentHandler.InitiateCash,
			)
			payments.POST("/confirm",
				middleware.RequirePermission("confirm_payment"),
				paymentHandler.Confirm,
			)
			payments.GET("/pending/:branch_id",
				middleware.RequirePermission("view_payments"),
				middleware.TenantBranchGuard("branch_id", func(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
					branch, err := branchService.GetBranchByID(ctx, id)
					return branch.TenantID, err
				}),
				paymentHandler.Pending,
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
			menu.DELETE("/items/:id", menuHandler.DeleteMenuItem)
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
			inventory.GET("/low-stock", inventoryHandler.LowStockAlerts)
			inventory.PATCH("/:item_id/stock", inventoryHandler.SetStock)
			inventory.DELETE("/:item_id", inventoryHandler.DeleteItem)
		}

		// ----- Reporting -----
		reports := protected.Group("/reports")
		reports.Use(middleware.RequirePermission("view_reports"))
		{
			reports.GET("/analytics/dashboard", analyticsHandler.Dashboard)
		}

		// ----- AI Assistant (owner + manager via view_reports) -----
		ai := protected.Group("/ai")
		ai.Use(middleware.RequirePermission("view_reports"))
		{
			ai.POST("/chat", aiHandler.Chat)
		}

		// ----- Staff Management -----
		// GET  /staff/list   — owner + manager (view_staff)
		// POST /staff/create — owner only      (manage_staff)
		// DEL  /staff/:id   — owner only       (manage_staff)
		staff := protected.Group("/staff")
		{
			staff.GET("/list",
				middleware.RequirePermission("view_staff"),
				staffHandler.GetStaff,
			)
			staff.POST("/create",
				middleware.RequirePermission("manage_staff"),
				staffHandler.CreateStaff,
			)
			staff.DELETE("/:id",
				middleware.RequirePermission("manage_staff"),
				staffHandler.DeleteStaff,
			)
		}

		// ----- Owner — Branches & Tables -----
		owner := protected.Group("/owner")
		owner.Use(middleware.RequirePermission("manage_branches"))
		{
			owner.GET("/tables", tableHandler.GetTablesWithSessions)
			owner.POST("/branches", branchHandler.CreateBranch)
			owner.GET("/branches", branchHandler.ListBranches)
			owner.POST("/branches/:id/tables",
				middleware.TenantBranchGuard("id", func(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
					branch, err := branchService.GetBranchByID(ctx, id)
					return branch.TenantID, err
				}),
				tableHandler.CreateTable,
			)
			owner.DELETE("/branches/:id",
				middleware.TenantBranchGuard("id", func(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
					branch, err := branchService.GetBranchByID(ctx, id)
					return branch.TenantID, err
				}),
				branchHandler.DeleteBranch,
			)
		}
	}

	// ========== ACCOUNT ROUTES (JWT required, deliberately NOT
	// subscription-gated — an expired tenant must still be able to see
	// their own status and pay to renew) ==========
	account := r.Group("/api")
	account.Use(middleware.AuthMiddleware(jwtSecret))
	account.Use(middleware.RelaxedRateLimit())
	{
		account.GET("/me", staffHandler.Me)

		account.GET("/subscription/plans", subscriptionHandler.Plans)
		account.POST("/subscription/renew",
			middleware.RequirePermission("manage_branches"), // owner-level action
			subscriptionHandler.InitiateRenewal,
		)
		account.POST("/subscription/verify", subscriptionHandler.Verify)
	}

	// ========== SUPERADMIN ROUTES (separate token type, no tenant scope) ==========
	superadmin := r.Group("/api/superadmin")
	superadmin.Use(middleware.SuperAdminAuthMiddleware(jwtSecret))
	superadmin.Use(middleware.RelaxedRateLimit())
	{
		superadmin.GET("/overview",
			middleware.RequirePermission("view_platform_analytics"),
			superAdminHandler.Overview,
		)
		superadmin.GET("/tenants",
			middleware.RequirePermission("view_platform_analytics"),
			superAdminHandler.Tenants,
		)
	}

	return r
}
