package main

import (
 "archive/zip"
 "bytes"
 "context"
 "crypto/rand"
 "database/sql"
 _ "embed"
 "encoding/base64"
 "encoding/csv"
 "encoding/hex"
 "encoding/json"
 "encoding/xml"
 "fmt"
 "io"
 "net"
 "net/http"
 "os"
 "os/exec"
 "path/filepath"
 "regexp"
 "runtime"
 "sort"
 "strconv"
 "strings"
 "sync"
 "time"
 "go.mau.fi/whatsmeow"
 "go.mau.fi/whatsmeow/types"
 waProto "go.mau.fi/whatsmeow/binary/proto"
 "modernc.org/sqlite"
)

//go:embed index.html
var deskHTML string
type Person struct {
 First string `json:"first_name"`; Last string `json:"last_name"`; Name string `json:"display_name,omitempty"`; Phone string `json:"phone"`
 Selected bool `json:"selected"`; Status string `json:"status"`; Detail string `json:"detail"`
}
type Desk struct {
 mu sync.Mutex; Rows []Person `json:"rows"`; Running bool `json:"running"`; Paused bool `json:"paused"`; Delay float64 `json:"delay"`; Template string `json:"template"`; Error string `json:"error"`; Source map[string]interface{} `json:"source,omitempty"`
 stop bool; groups map[string]string; client *whatsmeow.Client; token,host string; history *sql.DB
}
type Action struct { Data,Name,JID,Template,First,Last string; Delay float64; Expected bool; Selected []int; Index int; FirstName string `json:"first_name"`; LastName string `json:"last_name"` }

func init(){sql.Register("sqlite3", &sqlite.Driver{})}
func setupData() {
 base,err:=os.UserConfigDir();if err!=nil{panic(err)}
 path:=filepath.Join(base,"WhatsAppDesk");if override:=os.Getenv("WHATSAPP_DESK_DATA_DIR");override!=""{path=override};if err=os.MkdirAll(path,0700);err!=nil{panic(err)}
 if err=os.Chdir(path);err!=nil{panic(err)}
 os.MkdirAll("store",0700)
}
func startDesk(client *whatsmeow.Client) *Desk {
 b:=make([]byte,32);rand.Read(b)
 db,err:=sql.Open("sqlite3","store/desk-history.db");if err!=nil{panic(err)}
 _,err=db.Exec(`CREATE TABLE IF NOT EXISTS sends (id INTEGER PRIMARY KEY,phone TEXT,name TEXT,message TEXT,status TEXT,detail TEXT,time TEXT)`);if err!=nil{panic(err)}
 d:=&Desk{Rows:[]Person{},Delay:8,client:client,groups:map[string]string{},token:hex.EncodeToString(b),history:db}
 listener,err:=net.Listen("tcp","127.0.0.1:0");if err!=nil{panic(err)};d.host=listener.Addr().String()
 go http.Serve(listener,d)
 url:="http://"+d.host
 fmt.Println("WhatsApp Desk: "+url)
 if os.Getenv("WHATSAPP_DESK_NO_BROWSER")=="" {
  switch runtime.GOOS{case "darwin":exec.Command("open",url).Start();case "windows":exec.Command("rundll32","url.dll,FileProtocolHandler",url).Start();default:exec.Command("xdg-open",url).Start()}
 }
 return d
}
func respond(w http.ResponseWriter,v interface{},status int){w.Header().Set("Content-Type","application/json");w.Header().Set("Cache-Control","no-store");w.WriteHeader(status);json.NewEncoder(w).Encode(v)}
func (d *Desk) ServeHTTP(w http.ResponseWriter,r *http.Request){
 if r.Host!=d.host{respond(w,map[string]string{"error":"Invalid host"},403);return}
 if r.URL.Path=="/"&&r.Method=="GET" {w.Header().Set("Content-Type","text/html; charset=utf-8");w.Header().Set("Cache-Control","no-store");io.WriteString(w,strings.ReplaceAll(deskHTML,"__TOKEN__",d.token));return}
 if r.Header.Get("X-App-Token")!=d.token{respond(w,map[string]string{"error":"Unauthorized"},403);return}
 if r.Method=="GET" {
  switch r.URL.Path {
  case "/api/state":d.mu.Lock();defer d.mu.Unlock();respond(w,d,200)
  case "/api/connection":respond(w,map[string]interface{}{"connected":d.client.IsConnected()&&d.client.IsLoggedIn()},200)
  case "/api/qr":
   if d.client.IsLoggedIn(){respond(w,map[string]string{"error":"Already connected"},409);return}
   raw,err:=os.ReadFile("store/pairing.png");if err!=nil{respond(w,map[string]string{"error":"Waiting for pairing code"},404);return};w.Header().Set("Content-Type","image/png");w.Header().Set("Cache-Control","no-store");w.Write(raw)
  case "/api/groups":
   ctx,cancel:=context.WithTimeout(r.Context(),40*time.Second);defer cancel();gs,err:=d.client.GetJoinedGroups(ctx);if err!=nil{respond(w,map[string]string{"error":err.Error()},400);return}
   result:=[]map[string]interface{}{};d.mu.Lock();for _,g:=range gs{d.groups[g.JID.String()]=g.Name;result=append(result,map[string]interface{}{"jid":g.JID.String(),"name":g.Name})};d.mu.Unlock();sort.Slice(result,func(i,j int)bool{return fmt.Sprint(result[i]["name"])<fmt.Sprint(result[j]["name"])});respond(w,map[string]interface{}{"groups":result},200)
  case "/api/history":
   rows,err:=d.history.Query("SELECT phone,name,message,status,detail,time FROM sends ORDER BY id DESC LIMIT 1000");if err!=nil{respond(w,map[string]string{"error":err.Error()},500);return};defer rows.Close();out:=[]map[string]string{};for rows.Next(){var p,n,m,s,e,t string;rows.Scan(&p,&n,&m,&s,&e,&t);out=append(out,map[string]string{"phone":p,"name":n,"message":m,"status":s,"detail":e,"time":t})};respond(w,out,200)
  default:respond(w,map[string]string{"error":"Not found"},404)
  };return
 }
 if r.Method!="POST"{respond(w,map[string]string{"error":"Method not allowed"},405);return}
 var a Action
 if err:=json.NewDecoder(http.MaxBytesReader(w,r.Body,14_000_000)).Decode(&a);err!=nil{respond(w,map[string]string{"error":"Invalid request"},400);return}
 d.mu.Lock();defer d.mu.Unlock()
 fail:=func(err error){respond(w,map[string]string{"error":err.Error()},400)}
 switch r.URL.Path{
 case "/api/quit":
  if d.Running{fail(fmt.Errorf("Stop sending before quitting"));return};respond(w,map[string]bool{"ok":true},200);go func(){time.Sleep(250*time.Millisecond);d.client.Disconnect();d.history.Close();os.Exit(0)}();return
 case "/api/connect":
  respond(w,map[string]bool{"ok":true},200);return
 case "/api/import":
  if d.Running{fail(fmt.Errorf("Stop sending before replacing the list"));return};rows,err:=importList(a.Name,a.Data);if err!=nil{fail(err);return};d.Rows=rows;d.Source=nil;d.Error=""
 case "/api/group-import":
  if d.Running{fail(fmt.Errorf("Stop sending first"));return};name,ok:=d.groups[a.JID];if !ok{fail(fmt.Errorf("Refresh and choose one of your groups"));return}
  jid,err:=types.ParseJID(a.JID);if err!=nil{fail(err);return};ctx,cancel:=context.WithTimeout(r.Context(),40*time.Second);defer cancel();info,err:=d.client.GetGroupInfo(ctx,jid);if err!=nil{fail(err);return}
  rows:=[]Person{};seen:=map[string]bool{};missing:=0
  for _,p:=range info.Participants{
   phone:=p.PhoneNumber.User;if phone==""&&p.JID.Server==types.DefaultUserServer{phone=p.JID.User}
   if phone==""{mapped,e:=d.client.Store.LIDs.GetPNForLID(ctx,p.JID);if e==nil{phone=mapped.User}}
   if !validPhone(phone){missing++;continue};if seen[phone]{continue};seen[phone]=true
   contact,_:=d.client.Store.Contacts.GetContact(ctx,types.NewJID(phone,types.DefaultUserServer));display:=contact.FullName;if display==""{display=contact.FirstName};if display==""{display=contact.PushName};if display==""{display=p.DisplayName}
   rows=append(rows,Person{Phone:phone,Name:display,Status:"ready"})
  };sort.Slice(rows,func(i,j int)bool{return rows[i].Name<rows[j].Name});d.Rows=rows;d.Source=map[string]interface{}{"name":name,"total":len(info.Participants),"unavailable":missing};d.Error=""
 case "/api/edit-name":
  if d.Running||a.Index<0||a.Index>=len(d.Rows){fail(fmt.Errorf("Cannot edit this row"));return};d.Rows[a.Index].First=strings.TrimSpace(a.FirstName);d.Rows[a.Index].Last=strings.TrimSpace(a.LastName)
 case "/api/pause":d.Paused=!d.Paused
 case "/api/stop":d.stop=true;d.Paused=false
 case "/api/start":
  if d.Running{fail(fmt.Errorf("Sending is already running"));return};if !d.client.IsConnected()||!d.client.IsLoggedIn(){fail(fmt.Errorf("Connect WhatsApp first"));return};if !a.Expected{fail(fmt.Errorf("Confirm recipients expect these messages"));return}
  if a.Delay<1||a.Delay>3600||len(a.Template)==0||len(a.Template)>4000{fail(fmt.Errorf("Enter a message and an interval of 1–3600 seconds"));return}
  selected:=map[int]bool{};for _,i:=range a.Selected{selected[i]=true};count:=0
  for i:=range d.Rows{d.Rows[i].Selected=selected[i]&&d.Rows[i].Status=="ready";if d.Rows[i].Selected{count++;if _,err:=personalize(a.Template,d.Rows[i]);err!=nil{fail(err);return}}};if count==0{fail(fmt.Errorf("Select at least one ready recipient"));return}
  d.Running=true;d.Paused=false;d.stop=false;d.Error="";d.Template=a.Template;d.Delay=a.Delay;go d.run(a.Template,a.Delay)
 default:fail(fmt.Errorf("Unknown action"));return
 };respond(w,map[string]bool{"ok":true},200)
}
func validPhone(s string)bool{return regexp.MustCompile(`^[1-9][0-9]{6,14}$`).MatchString(s)}
func personalize(s string,p Person)(string,error){name:=strings.TrimSpace(strings.Join([]string{p.First,p.Last}," "));if name==""{name=p.Name};s=strings.NewReplacer("{name}",name,"{first_name}",p.First,"{last_name}",p.Last,"{phone}",p.Phone).Replace(s);if p.First==""{s=strings.ReplaceAll(s,"Good morning, !","Good morning!");s=strings.ReplaceAll(s,"Hi, !","Hi!")};if regexp.MustCompile(`\{[^{}]+\}`).MatchString(s){return "",fmt.Errorf("Unknown message placeholder")};return s,nil}
func (d *Desk) wait(seconds float64)bool{
 for seconds>0{d.mu.Lock();stop,paused:=d.stop,d.Paused;d.mu.Unlock();if stop{return false};time.Sleep(200*time.Millisecond);if !paused{seconds-=.2}}
 return true
}
func (d *Desk) run(template string,delay float64){
 defer func(){d.mu.Lock();d.Running=false;d.mu.Unlock()}()
 for i:=range d.Rows {
  for {d.mu.Lock();stop,paused:=d.stop,d.Paused;d.mu.Unlock();if stop{return};if !paused{break};time.Sleep(200*time.Millisecond)}
  d.mu.Lock();p:=d.Rows[i];if !p.Selected||p.Status!="ready"{d.mu.Unlock();continue};message,_:=personalize(template,p);d.Rows[i].Status="sending";d.mu.Unlock()
  record,err:=d.history.Exec("INSERT INTO sends(phone,name,message,status,detail,time) VALUES(?,?,?,'sending','',?)",p.Phone,p.First,message,time.Now().Format(time.RFC3339));if err!=nil{d.mu.Lock();d.Rows[i].Status="ready";d.Error="Could not save send history. Nothing sent for this recipient.";d.mu.Unlock();return}
  id,_:=record.LastInsertId();ctx,cancel:=context.WithTimeout(context.Background(),60*time.Second);jid,_:=types.ParseJID(p.Phone+"@s.whatsapp.net")
  _,err=d.client.SendMessage(ctx,jid,&waProto.Message{Conversation:&message});cancel()
  status,detail:="sent","Accepted by WhatsApp; delivery/read status unavailable.";if err!=nil{status="uncertain";detail=err.Error()}
  d.history.Exec("UPDATE sends SET status=?,detail=? WHERE id=?",status,detail,id)
  d.mu.Lock();d.Rows[i].Status=status;d.Rows[i].Detail=detail;if err!=nil{d.Error="Stopped after an uncertain send. Check WhatsApp before retrying."};d.mu.Unlock();if err!=nil{return};if !d.wait(delay){return}
 }
}

func importList(name,data string)([]Person,error){
 raw,err:=base64.StdEncoding.DecodeString(data);if err!=nil||len(raw)>10_000_000{return nil,fmt.Errorf("Use a file smaller than 10 MB")};var grid [][]string
 if strings.HasSuffix(strings.ToLower(name),".csv") {reader:=csv.NewReader(strings.NewReader(strings.TrimPrefix(string(raw),"\ufeff")));reader.FieldsPerRecord=-1;grid,err=reader.ReadAll()}else{grid,err=readXLSX(raw)};if err!=nil{return nil,err}
 norm:=func(s string)string{return regexp.MustCompile(`[^a-z]`).ReplaceAllString(strings.ToLower(s),"")};header,phone,first,last,full:=-1,-1,-1,-1,-1
 for i,row:=range grid{if i>30{break};for j,v:=range row{switch norm(v){case "phonenumber","phone","mobile","whatsapp":phone=j}};if phone>=0{header=i;for j,v:=range row{switch norm(v){case "firstname","first":first=j;case "lastname","surname":last=j;case "name","fullname","currentname","whatsappname":full=j}};break}}
 if header<0{return nil,fmt.Errorf("First sheet needs Phone Number and First Name headers")};result:=[]Person{};seen:=map[string]bool{}
 for _,row:=range grid[header+1:]{get:=func(i int)string{if i<0||i>=len(row){return ""};return strings.TrimSpace(row[i])};source:=get(phone);if source==""{continue};p:=regexp.MustCompile(`[^0-9]`).ReplaceAllString(source,"");r:=Person{First:get(first),Last:get(last),Name:get(full),Phone:p,Status:"ready",Selected:true};if !validPhone(p){r.Status="invalid";r.Detail="Use international format with country code"}else if seen[p]{r.Status="invalid";r.Detail="Duplicate number"};r.Selected=r.Status=="ready";seen[p]=true;result=append(result,r)};if len(result)==0{return nil,fmt.Errorf("No phone numbers found")};return result,nil
}
func readXLSX(raw []byte)([][]string,error){
 z,err:=zip.NewReader(bytes.NewReader(raw),int64(len(raw)));if err!=nil{return nil,err};files:=map[string]*zip.File{};var size uint64;for _,f:=range z.File{size+=f.UncompressedSize64;files[f.Name]=f};if size>40_000_000{return nil,fmt.Errorf("Workbook is too large")}
 read:=func(name string)([]byte,error){f:=files[name];if f==nil{return nil,fmt.Errorf("Workbook sheet unavailable")};r,e:=f.Open();if e!=nil{return nil,e};defer r.Close();return io.ReadAll(io.LimitReader(r,40_000_001))}
 var shared []string
 if files["xl/sharedStrings.xml"]!=nil{b,e:=read("xl/sharedStrings.xml");if e!=nil{return nil,e};var doc struct{Items []struct{Text string `xml:"t"`;Runs []struct{Text string `xml:"t"`} `xml:"r"`} `xml:"si"`};if e=xml.Unmarshal(b,&doc);e!=nil{return nil,e};for _,x:=range doc.Items{s:=x.Text;for _,r:=range x.Runs{s+=r.Text};shared=append(shared,s)}}
 b,err:=read("xl/worksheets/sheet1.xml");if err!=nil{return nil,err};var doc struct{Rows []struct{Cells []struct{Ref string `xml:"r,attr"`;Type string `xml:"t,attr"`;Value string `xml:"v"`;Inline struct{Text string `xml:"t"`;Runs []struct{Text string `xml:"t"`} `xml:"r"`} `xml:"is"`} `xml:"c"`} `xml:"sheetData>row"`};if err=xml.Unmarshal(b,&doc);err!=nil{return nil,err};grid:=[][]string{}
 for _,row:=range doc.Rows{out:=[]string{};for _,c:=range row.Cells{col:=0;for _,ch:=range c.Ref{if ch<'A'||ch>'Z'{break};col=col*26+int(ch-'A'+1)};if col<1||col>16384{continue};for len(out)<col{out=append(out,"")};s:=c.Value;if c.Type=="s"{i,e:=strconv.Atoi(s);if e!=nil||i<0||i>=len(shared){return nil,fmt.Errorf("Invalid workbook text")};s=shared[i]}else if c.Type=="inlineStr"{s=c.Inline.Text;for _,r:=range c.Inline.Runs{s+=r.Text}};out[col-1]=s};grid=append(grid,out)};return grid,nil
}
