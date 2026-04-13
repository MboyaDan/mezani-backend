package domain

var RolePermissions = map[Role][]string{

	RoleOwner: {
		"manage_branches",
		"manage_staff",
		"manage_menu",
		"view_reports",
		"manage_tables",
		"manage_inventory",
		"create_orders",
		"update_order_status",
		"view_kitchen_display",
		"close_bill",
	},

	RoleManager: {
		"manage_menu",
		"view_reports",
		"manage_tables",
		//"manage_staff",
		// Managers can view staff but not manage th
		"manage_inventory",
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
