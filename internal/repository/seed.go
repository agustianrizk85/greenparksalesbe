package repository

import (
	"greenpark/sales/internal/domain"
	"greenpark/sales/internal/passwd"
)

// This file holds the bootstrap accounts for a fresh store. The dashboard data
// set itself starts EMPTY — all figures are populated by uploading the
// "DASHBOARD SALES_GREENPARK" workbook through the import pipeline
// (Upload XLSX → Validasi → Cleaning → Mapping → Preview → Approve).
//
// defaultTarget2026 is the only retained configuration figure: the annual akad
// target the executive gauge measures against. It is a setting, not data, so an
// empty dashboard still reads "0 of 500".
const defaultTarget2026 = 500

// seedUsers creates the default accounts. Change these immediately in any real
// deployment. Default credentials: admin/admin123 and viewer/viewer123.
func seedUsers() []storeUser {
	mk := func(id, username, name string, role domain.Role, password string) storeUser {
		salt := passwd.NewSalt()
		return storeUser{
			ID:           id,
			Username:     username,
			Name:         name,
			Role:         role,
			Salt:         salt,
			PasswordHash: passwd.Hash(password, salt),
		}
	}
	return []storeUser{
		mk("usr-admin", "admin", "Administrator", domain.RoleAdmin, "admin123"),
		mk("usr-viewer", "viewer", "Viewer", domain.RoleViewer, "viewer123"),
	}
}
