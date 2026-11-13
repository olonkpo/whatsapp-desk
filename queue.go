package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

type campaignSnapshot struct {
	Rows     []Person               `json:"rows"`
	Template string                 `json:"template"`
	Delay    float64                `json:"delay"`
	Source   map[string]interface{} `json:"source,omitempty"`
}

// Call with d.mu held. SQLite commits the complete campaign before any send.
func (d *Desk) saveCampaign() error {
	if d.history == nil {
		return fmt.Errorf("Campaign storage is unavailable")
	}
	raw, err := json.Marshal(campaignSnapshot{d.Rows, d.Template, d.Delay, d.Source})
	if err != nil {
		return err
	}
	tx, err := d.history.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec("INSERT INTO campaign(id,payload) VALUES(1,?) ON CONFLICT(id) DO UPDATE SET payload=excluded.payload", raw); err != nil {
		return err
	}
	if d.attachment != d.savedAttachment {
		if _, err = tx.Exec("DELETE FROM campaign_attachment"); err != nil {
			return err
		}
		if a := d.attachment; a != nil {
			if _, err = tx.Exec("INSERT INTO campaign_attachment(id,name,mime,data) VALUES(1,?,?,?)", a.Name, a.MIME, a.Data); err != nil {
				return err
			}
		}
	}
	if err = tx.Commit(); err == nil {
		d.savedAttachment = d.attachment
	}
	return err
}
func (d *Desk) loadCampaign() error {
	for _, query := range []string{
		"CREATE TABLE IF NOT EXISTS campaign(id INTEGER PRIMARY KEY CHECK(id=1),payload BLOB NOT NULL)",
		"CREATE TABLE IF NOT EXISTS campaign_attachment(id INTEGER PRIMARY KEY CHECK(id=1),name TEXT,mime TEXT,data BLOB)",
	} {
		if _, err := d.history.Exec(query); err != nil {
			return err
		}
	}
	var raw []byte
	err := d.history.QueryRow("SELECT payload FROM campaign WHERE id=1").Scan(&raw)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	var saved campaignSnapshot
	if err = json.Unmarshal(raw, &saved); err != nil {
		return fmt.Errorf("Could not read saved campaign: %w", err)
	}
	if saved.Delay < 1 || saved.Delay > 3600 {
		return fmt.Errorf("Saved campaign has an invalid interval")
	}
	d.Rows = saved.Rows
	if d.Rows == nil {
		d.Rows = []Person{}
	}
	d.Template = saved.Template
	d.Delay = saved.Delay
	d.Source = saved.Source
	var a Attachment
	err = d.history.QueryRow("SELECT name,mime,data FROM campaign_attachment WHERE id=1").Scan(&a.Name, &a.MIME, &a.Data)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if err == nil {
		if _, err = parseAttachment(a.Name, a.Data); err != nil {
			return fmt.Errorf("Saved attachment is invalid: %w", err)
		}
		d.attachment = &a
		d.savedAttachment = &a
		d.AttachmentName = a.Name
	}
	recovered := 0
	for i := range d.Rows {
		if d.Rows[i].Status == "sending" {
			d.Rows[i].Status = "uncertain"
			d.Rows[i].Detail = "The app closed during this send. Check WhatsApp before sending again."
			d.Rows[i].Selected = false
			recovered++
		}
	}
	d.Restored = len(d.Rows) > 0
	if recovered > 0 {
		d.Error = "Recovered campaign. Interrupted sends need review in WhatsApp and will not be retried automatically."
	}
	if _, err = d.history.Exec("UPDATE sends SET status='uncertain',detail='App closed during send; check WhatsApp before retrying' WHERE status='sending'"); err != nil {
		return err
	}
	return d.saveCampaign()
}
