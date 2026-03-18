package domain

func HasPermission(role Role, permission string) bool {

	permissions := RolePermissions[role]

	for _, p := range permissions {

		if p == permission {
			return true
		}
	}

	return false
}
