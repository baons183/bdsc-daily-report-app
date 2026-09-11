package handlers

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

// validSQLIdent guards the one spot where a config value (the asset
// database's name) gets interpolated directly into SQL text, since SQL
// Server doesn't allow parameterizing a database name. Anything outside
// this pattern is rejected rather than sent to the server.
var validSQLIdent = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

type AssetHit struct {
	TagName         string `json:"tagName"`
	EquipDescription string `json:"equipDescription"`
	Area            string `json:"area"`
	Unit            string `json:"unit"`
}

// SearchAssets powers the Thiết bị autocomplete field, backed by the shared
// Asset_Master table (same equipment master MaintenanceReportApp/
// checksheet-app already use) living in a separate database on the same
// SQL Server instance/login.
func (e *Env) SearchAssets(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(q) < 2 {
		writeJSON(w, 200, []AssetHit{})
		return
	}
	if !validSQLIdent.MatchString(e.AssetDBName) {
		writeErr(w, 500, "cau hinh ASSET_DB_NAME khong hop le")
		return
	}
	query := fmt.Sprintf(`
SELECT TOP 20 TagName, ISNULL(EquipDescription,''), ISNULL(Area,''), ISNULL(Unit,'')
FROM %s.dbo.Asset_Master
WHERE TagName LIKE ?
ORDER BY TagName`, e.AssetDBName)
	rows, err := e.DB.QueryContext(r.Context(), query, q+"%")
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	defer rows.Close()
	out := []AssetHit{}
	for rows.Next() {
		var a AssetHit
		if err := rows.Scan(&a.TagName, &a.EquipDescription, &a.Area, &a.Unit); err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		out = append(out, a)
	}
	writeJSON(w, 200, out)
}
