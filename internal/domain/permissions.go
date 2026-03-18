package domain

var RolePermissions = map[Role][]string{

	RoleOwner: {
		"manage_branches",
		"manage_staff",
		"manage_menu",
		"view_reports",
		"manage_tables",
	},

	RoleManager: {
		"manage_menu",
		"view_reports",
		"manage_tables",
	},

	RoleWaiter: {
		"create_orders",
		"view_menu",
	},

	RoleKitchen: {
		"view_orders",
		"update_order_status",
	},

	RoleCashier: {
		"view_orders",
		"close_bill",
	},
}
