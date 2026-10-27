package user

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/user/legacy/testutil"
)

func TestUserAndCredFlow(t *testing.T) {
	a := testutil.New(t)
	Register(a)
	c := &testutil.Client{T: t, Mux: a.Mux}
	creds := map[string]any{"email": "A@Example.com", "password": "pw1", "userName": "ann"}

	// sign up: duplicate (case-insensitive), bad email, bodiless
	m := c.JSON("POST", "/user/signUp", creds)
	c.Want(m, 200)
	if m["t"] != "Succeed" || m["token"] == nil {
		t.Fatalf("signUp = %v", m)
	}
	c.Token = m["token"].(string)
	if m := c.JSON("POST", "/user/signUp", map[string]any{"email": "a@example.com", "password": "x"}); m["msgCode"] != 401.0 || m["t"] != "Repeated Email Address" {
		t.Fatalf("dup = %v", m)
	}
	if m := c.JSON("POST", "/user/signUp", map[string]any{"email": "nope", "password": "x"}); m["t"] != "Email is incorrect" || m["msgCode"] != 400.0 {
		t.Fatalf("bad email = %v", m)
	}
	if m := c.JSON("POST", "/user/signUp", map[string]any{"password": "x"}); m["t"] != "Email is null" {
		t.Fatalf("null email = %v", m)
	}
	if m := c.JSON("POST", "/user/signUp", map[string]any{"email": "b@b.co", "password": ""}); m["t"] != "Error Create User" || m["msgCode"] != 400.0 || m["token"] != nil {
		t.Fatalf("empty pw = %v", m)
	}

	// sign in
	if m := c.JSON("POST", "/user/signIn", map[string]any{"email": "a@example.com", "password": "bad"}); m["t"] != "Invalid Email or Password" {
		t.Fatalf("bad pw = %v", m)
	}
	m = c.JSON("POST", "/user/signIn", map[string]any{"email": "A@EXAMPLE.COM", "password": "pw1"})
	c.Want(m, 200)
	c2 := &testutil.Client{T: t, Mux: a.Mux, Token: m["token"].(string)}

	// unauthenticated envelope
	anon := &testutil.Client{T: t, Mux: a.Mux}
	if m := anon.JSON("GET", "/user/getInfo", nil); m["msgCode"] != 403.0 || m["t"] != nil {
		t.Fatalf("anon = %v", m)
	}

	info := c.Want(c.JSON("GET", "/user/getInfo", nil), 200).(map[string]any)
	if info["email"] != "A@Example.com" || info["userName"] != "ann" || info["role"] != "user" || info["password"] != nil || info["createDate"] == nil {
		t.Fatalf("info = %v", info)
	}
	st := c.Want(c.JSON("GET", "/user/getStorage", nil), 200).(map[string]any)
	if st["usedBytes"] != 0.0 || st["groupName"] != "Default" || st["quotaSource"] != "group" || st["overQuota"] != false || st["capacityBytes"] != 50000000000.0 {
		t.Fatalf("storage = %v", st)
	}

	// signup created the root namespace and favorite gallery
	_, raw := c.Do("GET", "/namespace/getRoot", "", nil)
	_ = raw

	// updates use form/query args
	form := func(path, body string) map[string]any {
		_, raw := c.Do("POST", path, "application/x-www-form-urlencoded", strings.NewReader(body))
		return decode(t, raw)
	}
	if m := form("/user/updateUsername", "username=bob"); m["t"] != 1.0 || m["msgCode"] != 200.0 {
		t.Fatalf("username = %v", m)
	}
	if m := form("/user/updateUsername", "username="); m["t"] != -1.0 || m["msgCode"] != 400.0 {
		t.Fatalf("empty username = %v", m)
	}
	if m := form("/user/updateEmail", "email=a%40example.com"); m["t"] != -2.0 {
		t.Fatalf("same email = %v", m)
	}
	if m := form("/user/updateEmail", "email=new%40example.com"); m["t"] != 1.0 {
		t.Fatalf("email = %v", m)
	}
	if m := form("/user/updatePassword", "newPass=n&oldPass=wrong"); m["t"] != -2.0 {
		t.Fatalf("pw wrong = %v", m)
	}
	if m := form("/user/updatePassword", "newPass=pw2&oldPass=pw1"); m["t"] != 1.0 {
		t.Fatalf("pw = %v", m)
	}
	if m := c.JSON("POST", "/user/signIn", map[string]any{"email": "new@example.com", "password": "pw2"}); m["msgCode"] != 200.0 {
		t.Fatalf("signIn after change = %v", m)
	}

	// avatar: default first, then an upload
	code, body := c.Do("GET", "/user/getAvatar/"+c.Token, "", nil)
	if code != 200 || len(body) != len(defaultAvatar) {
		t.Fatalf("default avatar: %d %d bytes", code, len(body))
	}
	if code, _ := anon.Do("GET", "/user/getAvatar/nope", "", nil); code != 403 {
		t.Fatalf("avatar anon = %d", code)
	}
	var png1 bytes.Buffer
	img := image.NewNRGBA(image.Rect(0, 0, 80, 40))
	for x := 0; x < 80; x++ {
		for y := 0; y < 40; y++ {
			img.Set(x, y, color.NRGBA{200, 30, 30, 255})
		}
	}
	_ = png.Encode(&png1, img)
	mp, ctype := multipartFile("file", "a.png", png1.Bytes())
	code, raw = c.Do("POST", "/user/updateAvatar", ctype, mp)
	if m := decode(t, raw); code != 200 || m["t"] != 1.0 {
		t.Fatalf("updateAvatar = %s", raw)
	}
	_, body = c.Do("GET", "/user/getAvatar/"+c.Token, "", nil)
	if cfg, err := decodeCfg(body); err != nil || cfg.Width != 512 || cfg.Height != 512 {
		t.Fatalf("avatar = %v %v", cfg, err)
	}
	mp, ctype = multipartFile("file", "a.png", []byte("not an image"))
	if _, raw = c.Do("POST", "/user/updateAvatar", ctype, mp); decode(t, raw)["t"] != -1.0 {
		t.Fatalf("bad avatar = %s", raw)
	}
	if st := c.Want(c.JSON("GET", "/user/getStorage", nil), 200).(map[string]any); st["usedBytes"].(float64) <= 0 {
		t.Fatalf("avatar bytes not counted: %v", st)
	}

	// creds
	if m := c.JSON("POST", "/cred/create", map[string]any{"authorization": "xyz"}); m["t"] != 0.0 || m["msgCode"] != 400.0 {
		t.Fatalf("bad cred = %v", m)
	}
	c.Want(c.JSON("POST", "/cred/create", map[string]any{"authorization": "dcr"}), 200)
	list := c.Want(c.JSON("GET", "/cred/query", nil), 200).([]any)
	cr := list[0].(map[string]any)
	if len(list) != 1 || cr["authorization"] != "CRD" {
		t.Fatalf("creds = %v", list)
	}
	tok := cr["token"].(string)
	c.Want(c.JSON("POST", "/cred/update", map[string]any{"credId": cr["credId"], "authorization": "r"}), 200)
	if m := c.JSON("POST", "/cred/update", map[string]any{"credId": 99999, "authorization": "r"}); m["t"] != -1.0 || m["msgCode"] != 400.0 {
		t.Fatalf("update foreign = %v", m)
	}
	// token routes: raw bodies, no envelope
	if code, b := anon.Do("GET", "/cred/queryRootNamespace/bad", "", nil); code != 200 || len(b) != 0 {
		t.Fatalf("raw unauth = %d %q", code, b)
	}
	_, b := anon.Do("GET", "/cred/queryRootNamespace/"+tok, "", nil)
	if root := decode(t, b); root["nsName"] != "/" || root["nsParentId"] != -1.0 {
		t.Fatalf("root = %s", b)
	}
	if _, b = anon.Do("POST", "/cred/deletePhoto/bad?uuid=x", "", nil); strings.TrimSpace(string(b)) != "-1" {
		t.Fatalf("deletePhoto bad token = %q", b)
	}
	if _, b = anon.Do("POST", "/cred/deletePhoto/"+tok+"?uuid=x", "", nil); strings.TrimSpace(string(b)) != "-2" {
		t.Fatalf("deletePhoto no D = %q", b)
	}
	mp, ctype = multipartFile("file", "p.jpg", []byte("x"))
	if _, b = anon.Do("POST", "/cred/uploadPhoto/bad", ctype, mp); string(b) != "Unidentified Token" {
		t.Fatalf("upload bad token = %q", b)
	}
	if _, b = anon.Do("POST", "/cred/uploadPhoto/"+tok, ctype, bytes.NewReader(nil)); string(b) != "Wrong Auth for current Token" {
		t.Fatalf("upload wrong auth = %q", b)
	}
	c.Want(c.JSON("POST", "/cred/delete?credId="+itoaF(cr["credId"]), nil), 200)

	// photos + render/deletePhoto through a CRD token, then user deletion
	c.Want(c.JSON("POST", "/cred/create", map[string]any{"authorization": "CRD"}), 200)
	tok = c.Want(c.JSON("GET", "/cred/query", nil), 200).([]any)[0].(map[string]any)["token"].(string)
	var uid int64
	var uuid string
	must(t, a.Pool.QueryRow(t.Context(), `SELECT user_id, uuid FROM "user" WHERE email='new@example.com'`).Scan(&uid, &uuid))
	var ns int64
	must(t, a.Pool.QueryRow(t.Context(), `SELECT ns_id FROM namespace WHERE user_id=$1`, uid).Scan(&ns))
	must(t, os.MkdirAll(filepath.Join(a.Cfg.UploadFolder, uuid), 0o755))
	for _, n := range []string{"u1.jpg", "u1_thumbnail.jpg"} {
		must(t, os.WriteFile(filepath.Join(a.Cfg.UploadFolder, uuid, n), []byte("jpegdata"), 0o644))
	}
	_, err := a.Pool.Exec(t.Context(), `INSERT INTO photo(photo_name,upload_date,format,ns_id,visibility,token,photo_uuid,user_id) VALUES ('p',now(),'jpg',$1,0,'t1','u1',$2)`, ns, uid)
	must(t, err)
	_, b = anon.Do("GET", "/cred/queryPhotoList/"+tok+"?nsId="+itoa(ns), "", nil)
	if !strings.Contains(string(b), `"photoUuid":"u1"`) {
		t.Fatalf("photo list = %s", b)
	}
	if code, b = anon.Do("GET", "/cred/render/"+tok+"?uuid=u1", "", nil); code != 200 || string(b) != "jpegdata" {
		t.Fatalf("render = %d %q", code, b)
	}
	if _, b = anon.Do("POST", "/cred/deletePhoto/"+tok+"?uuid=u1", "", nil); strings.TrimSpace(string(b)) != "1" {
		t.Fatalf("deletePhoto = %q", b)
	}
	if _, err := os.Stat(filepath.Join(a.Cfg.UploadFolder, uuid, "u1.jpg")); !os.IsNotExist(err) {
		t.Fatalf("file not removed: %v", err)
	}

	// delete user: counts namespaces+galleries+user (avatar is not counted) and kills sessions
	m = c.JSON("GET", "/user/deleteUser", nil)
	if m["msgCode"] != 200.0 || m["t"] != 3.0 {
		t.Fatalf("deleteUser = %v", m)
	}
	if m := c2.JSON("GET", "/user/getInfo", nil); m["msgCode"] != 403.0 {
		t.Fatalf("session survived delete: %v", m)
	}
	var n int
	must(t, a.Pool.QueryRow(t.Context(), `SELECT count(*) FROM "user"`).Scan(&n))
	if n != 0 {
		t.Fatalf("users left: %d", n)
	}
}

func TestSignUpClosedAndSignOut(t *testing.T) {
	a := testutil.New(t)
	Register(a)
	c := &testutil.Client{T: t, Mux: a.Mux}
	_, err := a.Pool.Exec(t.Context(), `UPDATE setting SET setting_value='0' WHERE setting_name='SignUp'`)
	must(t, err)
	if m := c.JSON("POST", "/user/signUp", map[string]any{"email": "a@b.co", "password": "x"}); m["t"] != "Register is closed by Admin" || m["msgCode"] != 400.0 {
		t.Fatalf("closed = %v", m)
	}
	_, err = a.Pool.Exec(t.Context(), `UPDATE setting SET setting_value='1' WHERE setting_name='SignUp'`)
	must(t, err)
	m := c.JSON("POST", "/user/signUp", map[string]any{"email": "a@b.co", "password": "x"})
	c.Token = m["token"].(string)
	if m := c.JSON("POST", "/user/pingToken", nil); m["t"] != 1.0 || m["msgCode"] != 200.0 {
		t.Fatalf("ping = %v", m)
	}
	if m := c.JSON("POST", "/user/signOut", nil); m["t"] != 1.0 {
		t.Fatalf("signOut = %v", m)
	}
	if m := c.JSON("POST", "/user/pingToken", nil); m["msgCode"] != 403.0 {
		t.Fatalf("after signOut = %v", m)
	}
}

func TestStorageQuota(t *testing.T) {
	a := testutil.New(t)
	Register(a)
	c := &testutil.Client{T: t, Mux: a.Mux}
	m := c.JSON("POST", "/user/signUp", map[string]any{"email": "q@q.co", "password": "x"})
	c.Token = m["token"].(string)
	_, err := a.Pool.Exec(t.Context(), `UPDATE user_group SET max_storage_bytes=10`)
	must(t, err)
	mp, ctype := multipartFile("file", "a.png", pngBytes())
	_, raw := c.Do("POST", "/user/updateAvatar", ctype, mp)
	if m := decode(t, raw); m["t"] != "storage_quota_exceeded" || m["msgCode"] != 413.0 {
		t.Fatalf("quota = %s", raw)
	}
	var used int64
	must(t, a.Pool.QueryRow(t.Context(), `SELECT used_bytes FROM "user"`).Scan(&used))
	if used != 0 {
		t.Fatalf("used_bytes = %d", used)
	}
}

var _ = http.StatusOK

func multipartFile(field, name string, data []byte) (*bytes.Buffer, string) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, _ := w.CreateFormFile(field, name)
	_, _ = fw.Write(data)
	w.Close()
	return &buf, w.FormDataContentType()
}

func pngBytes() []byte {
	var b bytes.Buffer
	_ = png.Encode(&b, image.NewNRGBA(image.Rect(0, 0, 64, 64)))
	return b.Bytes()
}
