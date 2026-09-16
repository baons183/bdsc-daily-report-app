package handlers

import (
	"context"
	"database/sql"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"bdscdailyreport/internal/db"
	"bdscdailyreport/internal/models"
	"bdscdailyreport/internal/report"
	"bdscdailyreport/internal/window"
)

func (e *Env) ListCongViec(w http.ResponseWriter, r *http.Request) {
	rh, rm := db.ReportTime(e.DB, 8, 0)
	start, end := window.EntryWindow(time.Now(), rh, rm)
	out, err := report.FetchCongViec(r.Context(), e.DB, start, end)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, out)
}

func (e *Env) loadAttachments(r *http.Request, congViecId int) []models.Attachment {
	rows, err := e.DB.QueryContext(r.Context(), `SELECT Id, CongViecId, FileName, FilePath, ISNULL(ContentType,'') FROM dbo.CongViecAttachments WHERE CongViecId = ?`, congViecId)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := []models.Attachment{}
	for rows.Next() {
		var a models.Attachment
		if err := rows.Scan(&a.Id, &a.CongViecId, &a.FileName, &a.FilePath, &a.ContentType); err == nil {
			out = append(out, a)
		}
	}
	return out
}

// CreateCongViec expects multipart/form-data: text fields (thietBi, dm,
// noiDung, keHoach, tienDo, homNay, ngayMai, deXuat, luuY) plus zero or more
// files under the "photos" field.
func (e *Env) CreateCongViec(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(50 << 20); err != nil {
		writeErr(w, 400, "khong doc duoc du lieu form: "+err.Error())
		return
	}
	thietBi := r.FormValue("thietBi")
	if thietBi == "" {
		writeErr(w, 400, "thieu Thiet bi")
		return
	}
	var tienDo *float64
	if v := r.FormValue("tienDo"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			tienDo = &f
		}
	}
	now := time.Now()
	if !checkNotLocked(w, r, e.DB, now) {
		return
	}
	rh, rm := db.ReportTime(e.DB, 8, 0)
	start, end := window.EntryWindow(now, rh, rm)

	tx, err := e.DB.BeginTx(r.Context(), nil)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	defer tx.Rollback()

	tt, err := nextTTLocked(r.Context(), tx, "CongViec", start, end)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	user := currentUser(r)
	createdBy := ""
	if user != nil {
		createdBy = user.EmployeeCode
	}
	var newID int64
	err = tx.QueryRowContext(r.Context(), `
INSERT INTO dbo.CongViec (TT, ThietBi, DM, NoiDung, KeHoach, TienDo, HomNay, NgayMai, DeXuat, LuuY, NgayBaoCao, CreatedBy)
OUTPUT CAST(INSERTED.Id AS BIGINT)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		tt, thietBi, r.FormValue("dm"), r.FormValue("noiDung"), r.FormValue("keHoach"), tienDo,
		r.FormValue("homNay"), r.FormValue("ngayMai"), r.FormValue("deXuat"), r.FormValue("luuY"), now, createdBy).Scan(&newID)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}

	// Save uploaded photos to disk and record them, still inside the same
	// transaction so a mid-upload failure rolls back the CongViec row too.
	if err := saveCongViecPhotos(r.Context(), tx, e.UploadsDir, newID, r.MultipartForm.File["photos"]); err != nil {
		writeErr(w, 500, err.Error())
		return
	}

	if err := tx.Commit(); err != nil {
		writeErr(w, 500, err.Error())
		return
	}

	out := models.CongViec{
		Id: int(newID), TT: tt, ThietBi: thietBi, DM: r.FormValue("dm"), NoiDung: r.FormValue("noiDung"),
		KeHoach: r.FormValue("keHoach"), TienDo: tienDo, HomNay: r.FormValue("homNay"), NgayMai: r.FormValue("ngayMai"),
		DeXuat: r.FormValue("deXuat"), LuuY: r.FormValue("luuY"), NgayBaoCao: now, CreatedBy: createdBy,
	}
	out.Attachments = e.loadAttachments(r, out.Id)
	writeJSON(w, 201, out)
}

func (e *Env) DeleteCongViec(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		writeErr(w, 400, "id khong hop le")
		return
	}
	var ngayBaoCao time.Time
	var createdBy string
	if err := e.DB.QueryRowContext(r.Context(), `SELECT NgayBaoCao, ISNULL(CreatedBy,'') FROM dbo.CongViec WHERE Id = ?`, id).Scan(&ngayBaoCao, &createdBy); err != nil {
		writeErr(w, 404, "khong tim thay dong nay")
		return
	}
	if !checkOwnership(w, r, createdBy) {
		return
	}
	if !checkNotLocked(w, r, e.DB, ngayBaoCao) {
		return
	}
	// delete photo files on disk (CongViecAttachments rows cascade-delete in SQL)
	dir := filepath.Join(e.UploadsDir, "congviec", strconv.Itoa(id))
	os.RemoveAll(dir)
	if _, err := e.DB.ExecContext(r.Context(), `DELETE FROM dbo.CongViec WHERE Id = ?`, id); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	rh, rm := db.ReportTime(e.DB, 8, 0)
	start, end := window.EntryWindow(ngayBaoCao, rh, rm)
	if err := renumber(r.Context(), e.DB, "CongViec", start, end); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

// saveCongViecPhotos writes each uploaded file to uploads/congviec/<id>/ and
// records it in CongViecAttachments, inside the given transaction — shared
// by Create (new row) and Update (appending more photos to an existing row).
func saveCongViecPhotos(ctx context.Context, tx *sql.Tx, uploadsDir string, congViecId int64, files []*multipart.FileHeader) error {
	if len(files) == 0 {
		return nil
	}
	dir := filepath.Join(uploadsDir, "congviec", strconv.FormatInt(congViecId, 10))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, fh := range files {
		src, err := fh.Open()
		if err != nil {
			return err
		}
		destPath := filepath.Join(dir, fh.Filename)
		dst, err := os.Create(destPath)
		if err != nil {
			src.Close()
			return err
		}
		_, copyErr := io.Copy(dst, src)
		src.Close()
		dst.Close()
		if copyErr != nil {
			return copyErr
		}
		relPath := filepath.ToSlash(filepath.Join("congviec", strconv.FormatInt(congViecId, 10), fh.Filename))
		contentType := fh.Header.Get("Content-Type")
		if _, err := tx.ExecContext(ctx, `
INSERT INTO dbo.CongViecAttachments (CongViecId, FileName, FilePath, ContentType) VALUES (?, ?, ?, ?)`,
			congViecId, fh.Filename, relPath, contentType); err != nil {
			return err
		}
	}
	return nil
}

// UpdateCongViec expects the same multipart/form-data shape as
// CreateCongViec. Text fields replace the existing values; any new files
// under "photos" are appended to the existing attachments (nothing is
// removed here — deleting individual photos isn't supported yet).
func (e *Env) UpdateCongViec(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		writeErr(w, 400, "id khong hop le")
		return
	}
	var createdBy string
	var ngayBaoCao time.Time
	if err := e.DB.QueryRowContext(r.Context(), `SELECT ISNULL(CreatedBy,''), NgayBaoCao FROM dbo.CongViec WHERE Id = ?`, id).Scan(&createdBy, &ngayBaoCao); err != nil {
		writeErr(w, 404, "khong tim thay dong nay")
		return
	}
	if !checkEditOwnership(w, r, createdBy) {
		return
	}
	if !checkNotLocked(w, r, e.DB, ngayBaoCao) {
		return
	}
	if err := r.ParseMultipartForm(50 << 20); err != nil {
		writeErr(w, 400, "khong doc duoc du lieu form: "+err.Error())
		return
	}
	thietBi := r.FormValue("thietBi")
	if thietBi == "" {
		writeErr(w, 400, "thieu Thiet bi")
		return
	}
	var tienDo *float64
	if v := r.FormValue("tienDo"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			tienDo = &f
		}
	}

	tx, err := e.DB.BeginTx(r.Context(), nil)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(r.Context(), `
UPDATE dbo.CongViec SET ThietBi=?, DM=?, NoiDung=?, KeHoach=?, TienDo=?, HomNay=?, NgayMai=?, DeXuat=?, LuuY=?
WHERE Id=?`,
		thietBi, r.FormValue("dm"), r.FormValue("noiDung"), r.FormValue("keHoach"), tienDo,
		r.FormValue("homNay"), r.FormValue("ngayMai"), r.FormValue("deXuat"), r.FormValue("luuY"), id); err != nil {
		writeErr(w, 500, err.Error())
		return
	}

	files := []*multipart.FileHeader{}
	if r.MultipartForm != nil {
		files = r.MultipartForm.File["photos"]
	}
	if err := saveCongViecPhotos(r.Context(), tx, e.UploadsDir, int64(id), files); err != nil {
		writeErr(w, 500, err.Error())
		return
	}

	if err := tx.Commit(); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}
