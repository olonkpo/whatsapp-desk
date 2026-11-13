package main

import (
	"bytes"
	"database/sql"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func campaignDB(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err = db.Exec("CREATE TABLE IF NOT EXISTS sends(id INTEGER PRIMARY KEY,status TEXT,detail TEXT)"); err != nil {
		t.Fatal(err)
	}
	return db
}
func TestCampaignCrashRecovery(t *testing.T) {
	path := filepath.Join(t.TempDir(), "campaign.db")
	db := campaignDB(t, path)
	d := &Desk{history: db, Delay: 8, Template: "Hello {name}", Rows: []Person{{Phone: "15555550101", Status: "sent", Selected: true}, {Phone: "15555550102", Status: "sending", Selected: true}, {Phone: "15555550103", Status: "ready", Selected: true}}}
	if err := d.loadCampaign(); err != nil {
		t.Fatal(err)
	}
	d.attachment = &Attachment{Name: "photo.png", MIME: "image/png", Data: []byte("\x89PNG\r\n\x1a\n")}
	if err := d.saveCampaign(); err != nil {
		t.Fatal(err)
	}
	db.Exec("INSERT INTO sends(status,detail) VALUES('sending','')")
	db.Close()
	restored := &Desk{history: campaignDB(t, path)}
	if err := restored.loadCampaign(); err != nil {
		t.Fatal(err)
	}
	if restored.Running || !restored.Restored || restored.Template != "Hello {name}" || restored.Delay != 8 {
		t.Fatal("Campaign settings not recovered safely")
	}
	if restored.Rows[0].Status != "sent" || restored.Rows[1].Status != "uncertain" || restored.Rows[1].Selected || restored.Rows[2].Status != "ready" || !restored.Rows[2].Selected {
		t.Fatal(restored.Rows)
	}
	if restored.attachment == nil || !bytes.Equal(restored.attachment.Data, d.attachment.Data) {
		t.Fatal("Attachment lost")
	}
	var status string
	restored.history.QueryRow("SELECT status FROM sends LIMIT 1").Scan(&status)
	if status != "uncertain" {
		t.Fatal(status)
	}
	restored.attachment = nil
	if err := restored.saveCampaign(); err != nil {
		t.Fatal(err)
	}
	again := &Desk{history: restored.history}
	if err := again.loadCampaign(); err != nil || again.attachment != nil {
		t.Fatal("Removed attachment recovered", err)
	}
}
func TestStorageFailurePreventsSend(t *testing.T) {
	d := &Desk{Rows: []Person{{Phone: "15555550101", Status: "ready", Selected: true}}, Running: true}
	// A nil WhatsApp client would panic if the send were attempted.
	d.run("Hello", 1)
	if d.Running || d.Rows[0].Status != "ready" || d.Error == "" {
		t.Fatal("Failed to stop before an unrecorded send")
	}
}

func TestCampaignActionsPersist(t *testing.T) {
	d := &Desk{history: campaignDB(t, filepath.Join(t.TempDir(), "campaign.db")), Delay: 8, host: "127.0.0.1:1234", token: "test", Rows: []Person{{First: "Ada", Phone: "15555550101", Status: "sent"}, {First: "Ben", Phone: "15555550102", Status: "uncertain"}, {First: "Casey", Phone: "15555550103", Status: "ready"}}}
	if err := d.loadCampaign(); err != nil {
		t.Fatal(err)
	}
	post := func(path, body string) {
		t.Helper()
		r := httptest.NewRequest("POST", "http://127.0.0.1:1234/api/"+path, strings.NewReader(body))
		r.Header.Set("X-App-Token", "test")
		w := httptest.NewRecorder()
		d.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	post("save-draft", `{"template":"Hi {name}","delay":12,"selected":[0,1,2]}`)
	post("clear-completed", `{}`)
	recovered := &Desk{history: d.history}
	if err := recovered.loadCampaign(); err != nil {
		t.Fatal(err)
	}
	if len(recovered.Rows) != 2 || recovered.Rows[0].Status != "uncertain" || recovered.Rows[0].Selected || !recovered.Rows[1].Selected || recovered.Delay != 12 || recovered.Template != "Hi {name}" {
		t.Fatal(recovered.Rows)
	}
}
