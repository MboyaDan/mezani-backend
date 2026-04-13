package domain

import "log"

func HasPermission(role Role, permission string) bool {

	permissions, ok := RolePermissions[role]

	log.Printf("HasPermission | role='%s' | ok=%v | permissions=%v | looking for='%s'",
		role, ok, permissions, permission)

	for _, p := range permissions {

		if p == permission {
			return true
		}
	}

	return false
}
