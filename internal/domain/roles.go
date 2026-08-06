package domain

type Role string

const (
	RoleOwner      Role = "owner"
	RoleManager    Role = "manager"
	RoleWaiter     Role = "waiter"
	RoleKitchen    Role = "kitchen"
	RoleCashier    Role = "cashier"
	RoleSuperAdmin Role = "superadmin"
)
