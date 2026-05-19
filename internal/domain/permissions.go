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
