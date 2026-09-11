// Package emailer sends the finished daily report PDF by POSTing it to a
// Power Automate Flow webhook — the same pattern MaintenanceReportApp and
// checksheet-app already use for their own notification emails, so no SMTP
// credentials need to live in this app's .env. The Flow itself owns
// recipients/subject/body; this side only ships the file.
package emailer

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type payload struct {
	ReportDate  string `json:"report_date"`
	FileName    string `json:"file_name"`
	FileContent string `json:"file_content"`
}

// SendReportPDF POSTs the given PDF (base64-encoded) to the Flow webhook.
// A blank flowURL is treated as "email disabled" and returns nil without
// doing anything.
func SendReportPDF(flowURL, pdfPath string, reportDate time.Time) error {
	if flowURL == "" {
		return nil
	}
	data, err := os.ReadFile(pdfPath)
	if err != nil {
		return fmt.Errorf("khong doc duoc file PDF: %w", err)
	}
	body, err := json.Marshal(payload{
		ReportDate:  reportDate.Format("02/01/2006"),
		FileName:    filepath.Base(pdfPath),
		FileContent: base64.StdEncoding.EncodeToString(data),
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, flowURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("khong goi duoc Power Automate Flow: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("Flow tra ve loi, status %d", resp.StatusCode)
	}
	return nil
}
