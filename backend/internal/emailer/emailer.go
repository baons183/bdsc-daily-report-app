// Package emailer notifies the finished daily report by POSTing a ready-made
// HTML summary to a Power Automate Flow webhook — the same pattern
// MaintenanceReportApp and checksheet-app already use for their own
// notification emails, so no SMTP credentials need to live in this app's
// .env. The Flow itself owns recipients/subject/body; this side only ships
// the content. The PDF is no longer attached — it lives in a
// SharePoint-synced folder and the HTML body links straight to it.
package emailer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type payload struct {
	ReportDate    string `json:"report_date"`
	EmailBodyHTML string `json:"email_body_html"`
}

// SendReportPDF POSTs the ready-to-use HTML email body (report.RenderEmailHTML,
// which already links to the report's SharePoint copy) to the Flow webhook —
// the Flow's "Send an email" action just needs its Body field set to
// email_body_html. A blank flowURL is treated as "email disabled" and
// returns nil without doing anything.
func SendReportPDF(flowURL, emailBodyHTML string, reportDate time.Time) error {
	if flowURL == "" {
		return nil
	}
	body, err := json.Marshal(payload{
		ReportDate:    reportDate.Format("02/01/2006"),
		EmailBodyHTML: emailBodyHTML,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, flowURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 180 * time.Second}
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
