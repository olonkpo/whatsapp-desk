package main

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"encoding/base64"
	"go.mau.fi/whatsmeow"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportAndPersonalize(t *testing.T) {
	csv := "First Name,Last Name,Phone Number\nAda,Test,+15555550101\nDuplicate,,+15555550101\nBad,,0123\n"
	rows, e := importList("test.csv", base64.StdEncoding.EncodeToString([]byte(csv)))
	if e != nil || len(rows) != 3 {
		t.Fatal(e, rows)
	}
	if rows[0].Status != "ready" || rows[1].Status != "invalid" || rows[2].Status != "invalid" {
		t.Fatal(rows)
	}
	s, e := personalize("Good morning, {first_name}!", rows[0])
	if e != nil || s != "Good morning, Ada!" {
		t.Fatal(s, e)
	}
	s, e = personalize("Good morning, {first_name}!", Person{})
	if e != nil || s != "Good morning!" {
		t.Fatal(s, e)
	}
	if _, e = personalize("Hello {unknown}", Person{}); e == nil {
		t.Fatal("Unknown placeholder accepted")
	}
}
func TestNamePlaceholder(t *testing.T) {
	for _, test := range []struct {
		p    Person
		want string
	}{{Person{First: "Ada", Last: "Test", Name: "Profile"}, "Hi Ada Test"}, {Person{Name: "Profile Name"}, "Hi Profile Name"}, {Person{First: "Ada"}, "Hi Ada"}, {Person{}, "Hi "}} {
		got, e := personalize("Hi {name}", test.p)
		if e != nil || got != test.want {
			t.Fatal(got, e, test.want)
		}
	}
}
func TestWorkbook(t *testing.T) {
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	f, _ := z.Create("xl/worksheets/sheet1.xml")
	f.Write([]byte(`<worksheet><sheetData><row><c r="A1" t="inlineStr"><is><t>First Name</t></is></c><c r="B1" t="inlineStr"><is><t>Phone Number</t></is></c></row><row><c r="A2" t="inlineStr"><is><t>Ada</t></is></c><c r="B2" t="inlineStr"><is><t>'+15555550101</t></is></c></row></sheetData></worksheet>`))
	z.Close()
	r, e := importList("test.xlsx", base64.StdEncoding.EncodeToString(b.Bytes()))
	if e != nil || len(r) != 1 || r[0].First != "Ada" || r[0].Phone != "15555550101" {
		t.Fatal(r, e)
	}
}
func TestPureSQLite(t *testing.T) {
	db, e := sql.Open("sqlite3", "file:"+filepath.ToSlash(filepath.Join(t.TempDir(), "test.db"))+"?_pragma=foreign_keys(1)")
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	if _, e = db.Exec("CREATE TABLE test(id INTEGER PRIMARY KEY, value TEXT)"); e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec("INSERT INTO test(value) VALUES(?)", "ok"); e != nil {
		t.Fatal(e)
	}
}
func TestAccessAndImportGuards(t *testing.T) {
	d := &Desk{host: "127.0.0.1:1234", token: "test", groups: map[string]string{}}
	r := httptest.NewRequest("GET", "http://127.0.0.1:1234/api/state", nil)
	w := httptest.NewRecorder()
	d.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatal(w.Code)
	}
	r = httptest.NewRequest("POST", "http://127.0.0.1:1234/api/group-import", strings.NewReader(`{"jid":"unknown@g.us"}`))
	r.Header.Set("X-App-Token", "test")
	w = httptest.NewRecorder()
	d.ServeHTTP(w, r)
	if w.Code != 400 {
		t.Fatal(w.Code)
	}
}

func TestAttachments(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\n")
	a, e := parseAttachment("photo.png", png)
	if e != nil {
		t.Fatal(e)
	}
	m := mediaMessage(a, whatsmeow.UploadResponse{}, "Hello Ada")
	if m.GetImageMessage().GetCaption() != "Hello Ada" || m.GetConversation() != "" {
		t.Fatal("Caption must be part of the photo message")
	}
	video := &Attachment{MIME: "video/mp4"}
	m = mediaMessage(video, whatsmeow.UploadResponse{}, "Hello Ben")
	if m.GetVideoMessage().GetCaption() != "Hello Ben" {
		t.Fatal("Missing video caption")
	}
	for _, raw := range [][]byte{nil, []byte("not a photo"), make([]byte, maxAttachmentBytes+1)} {
		if _, e := parseAttachment("fake.jpg", raw); e == nil {
			t.Fatal("Invalid media accepted")
		}
	}
	d := &Desk{host: "127.0.0.1:1234", token: "test", Running: true, attachment: a}
	r := httptest.NewRequest("POST", "http://127.0.0.1:1234/api/attachment", bytes.NewReader(png))
	r.Header.Set("X-App-Token", "test")
	w := httptest.NewRecorder()
	d.ServeHTTP(w, r)
	if w.Code != 400 || d.attachment != a {
		t.Fatal("Attachment changed during send")
	}
	d.Running = false
	r = httptest.NewRequest("POST", "http://127.0.0.1:1234/api/remove-attachment", strings.NewReader("{}"))
	r.Header.Set("X-App-Token", "test")
	w = httptest.NewRecorder()
	d.ServeHTTP(w, r)
	if w.Code != 200 || d.attachment != nil {
		t.Fatal("Attachment not removed")
	}
}
