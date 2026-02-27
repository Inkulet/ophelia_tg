package app

type Permission string

const (
	PermEdit        Permission = "edit"
	PermDelete      Permission = "delete"
	PermMassTag     Permission = "mass_tag"
	PermBroadcast   Permission = "broadcast"
	PermImportDB    Permission = "import_db"
	PermWhitelist   Permission = "whitelist"
	PermViewChats   Permission = "view_chats"
	PermModerators  Permission = "moderators"
	PermCollections Permission = "collections"
	PermAudit       Permission = "audit"
)

var rolePermissions = map[string]map[Permission]bool{
	"moderator": {
		PermEdit:  true,
		PermAudit: true,
	},
	"editor": {
		PermEdit:        true,
		PermCollections: true,
	},
}

func hasPermission(userID int64, perm Permission) bool {
	if isAdmin(userID) {
		return true
	}
	if womanManager == nil {
		return false
	}
	role, ok := womanManager.GetModeratorRole(userID)
	if !ok {
		return false
	}
	if perms, ok := rolePermissions[role]; ok {
		return perms[perm]
	}
	return false
}
