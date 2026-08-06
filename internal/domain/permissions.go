package domain

var RolePermissions = map[Role][]string{

	RoleOwner: {
		"view_staff",
		"manage_staff",
		"manage_branches",
		"manage_menu",
		"view_reports",
		"manage_tables",
		"manage_inventory",
		"create_orders",
		"update_order_status",
		"view_kitchen_display",
		"close_bill",
		"initiate_payment",
		"confirm_payment",
		"view_payments",
	},

	RoleManager: {
		"view_staff",
		"manage_menu",
		"view_reports",
		"manage_tables",
		"manage_inventory",
		"create_orders",
		"update_order_status",
		"view_kitchen_display",
		"close_bill",
		"initiate_payment",
		"confirm_payment",
		"view_payments",
	},

	RoleWaiter: {
		"create_orders",
		"view_menu",
		"initiate_payment",
	},

	RoleKitchen: {
		"view_orders",
		"update_order_status",
	},

	RoleCashier: {
		"view_orders",
		"close_bill",
		"initiate_payment",
		"confirm_payment",
		"view_payments",
	},

	RoleSuperAdmin: {
		"view_platform_analytics",
		"manage_tenants",
	},
}
