// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package testing provides tools to tests the HTTP routes, the message bus, email sending, etc.
package testing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"runtime"
	"strings"
	"testing"
	"text/template"
	"time"

	"golang.org/x/net/html"

	"github.com/itchyny/gojq"
	"github.com/kinbiko/jsonassert"
	"github.com/stretchr/testify/require"
	"github.com/wneessen/go-mail"

	"codeberg.org/readeck/readeck/configs"
	"codeberg.org/readeck/readeck/internal/app"
	"codeberg.org/readeck/readeck/internal/auth/tokens"
	"codeberg.org/readeck/readeck/internal/auth/users"
	"codeberg.org/readeck/readeck/internal/bookmarks"
	"codeberg.org/readeck/readeck/internal/db"
	"codeberg.org/readeck/readeck/internal/email"
	"codeberg.org/readeck/readeck/internal/server"
)

type fixtureData struct {
	Users map[string]struct {
		Group     string `json:"group"`
		Bookmarks []struct {
			UID    string                  `json:"uid"`
			URL    string                  `json:"url"`
			State  bookmarks.BookmarkState `json:"state"`
			Labels []string                `json:"labels"`
		} `json:"bookmarks"`
	}
	Files map[string]string `json:"files"`

	users map[string]*TestUser
}

func loadFixtures(t *testing.T) *fixtureData {
	_, curFile, _, _ := runtime.Caller(0)
	fd, err := os.Open(path.Join(path.Dir(curFile), "fixtures/data.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer fd.Close() // nolint:errcheck

	dec := json.NewDecoder(fd)
	res := new(fixtureData)
	if err := dec.Decode(res); err != nil {
		t.Fatal(err)
	}

	res.copyFiles(t)
	res.createUsers(t)
	res.createBookmarks(t)

	return res
}

func (f *fixtureData) createUsers(t *testing.T) {
	f.users = map[string]*TestUser{}
	for name, user := range f.Users {
		if name == "" {
			f.users[name] = &TestUser{}
			continue
		}

		tu, err := NewTestUser(name, name+"@localhost", name, user.Group)
		if err != nil {
			t.Fatal(err)
		}
		f.users[name] = tu
		t.Logf("created user: %s[%s]", tu.User.Username, tu.User.Group)
	}
}

func (f *fixtureData) copyFiles(t *testing.T) {
	_, curFile, _, _ := runtime.Caller(0)
	root := path.Join(path.Dir(curFile), "fixtures")

	for dstFile, srcFile := range f.Files {
		func(dstFile, srcFile string) {
			dstFile = path.Join(configs.Config.Main.DataDirectory, dstFile)
			srcFile = path.Join(root, srcFile)
			if err := os.MkdirAll(path.Dir(dstFile), 0o750); err != nil {
				t.Fatal(err)
			}

			src, err := os.Open(srcFile)
			if err != nil {
				t.Fatal(err)
			}
			defer src.Close() // nolint:errcheck

			dst, err := os.Create(dstFile)
			if err != nil {
				t.Fatal(err)
			}
			defer dst.Close() // nolint:errcheck

			if _, err := io.Copy(dst, src); err != nil {
				t.Fatal(err)
			}

			t.Logf("copy %s -> %s", srcFile, dstFile)
		}(dstFile, srcFile)
	}
}

func (f *fixtureData) createBookmarks(t *testing.T) {
	for username, tu := range f.users {
		tu.Bookmarks = []*bookmarks.Bookmark{}
		for _, bookmark := range f.Users[username].Bookmarks {
			b := &bookmarks.Bookmark{
				URL:      bookmark.URL,
				State:    bookmark.State,
				FilePath: bookmark.UID[0:2] + "/" + bookmark.UID,
				Labels:   bookmark.Labels,
			}
			if username == "" {
				b.UID = bookmark.UID
				tu.Bookmarks = append(tu.Bookmarks, b)
				continue
			}

			b.UserID = &tu.User.ID

			if err := bookmarks.Bookmarks.Create(b); err != nil {
				t.Fatal(err)
			}
			b.UID = bookmark.UID
			if err := b.Save(); err != nil {
				t.Fatal(err)
			}
			tu.Bookmarks = append(tu.Bookmarks, b)
		}
	}
}

// TestUser contains the user data that we can use during tests.
type TestUser struct {
	User      *users.User
	Token     *tokens.Token
	Bookmarks []*bookmarks.Bookmark
	password  string
	token     string
}

// NewTestUser creates a new user for testing.
func NewTestUser(name, email, password, group string) (*TestUser, error) {
	u := &users.User{
		Username: name,
		Email:    email,
		Password: password,
		Group:    group,
		Settings: &users.UserSettings{
			Lang: "en-US",
		},
	}
	if err := users.Users.Create(u); err != nil {
		return nil, err
	}

	res := &TestUser{
		User:      u,
		password:  password,
		Bookmarks: []*bookmarks.Bookmark{},
	}

	res.Token = &tokens.Token{
		UserID:      &u.ID,
		IsEnabled:   true,
		Application: "tests",
	}
	if err := tokens.Tokens.Create(res.Token); err != nil {
		return nil, err
	}
	token, err := configs.Keys.TokenKey().Encode(res.Token.UID)
	if err != nil {
		return nil, err
	}
	res.token = token

	return res, nil
}

// Password returns the user's password.
func (tu *TestUser) Password() string {
	return tu.password
}

// APIToken returns the user's API token.
func (tu *TestUser) APIToken() string {
	return tu.token
}

// Login performs the login for the user.
func (tu *TestUser) Login(c *Client) {
	c.Login(tu.User.Username, tu.Password())
}

// TestApp holds information of the application for testing.
type TestApp struct {
	TmpDir    string
	Srv       *server.Server
	Users     map[string]*TestUser
	Bookmarks map[string]*bookmarks.Bookmark
	LastEmail string
}

// NewTestApp initializes TestApp with a default configuration,
// some users, and an http muxer ready to accept requests.
func NewTestApp(t *testing.T) *TestApp {
	var err error
	tmpDir, err := os.MkdirTemp(os.TempDir(), "readeck_*")
	if err != nil {
		t.Fatal(err)
	}

	configs.Config.Main.SecretKey = "1234567890"
	configs.Config.Main.DataDirectory = tmpDir
	configs.Config.Extractor.ContentScripts = []string{path.Join(tmpDir, "content-scripts")}
	configs.Config.Main.LogLevel = slog.LevelError
	configs.Config.Database.Source = "sqlite3::memory:"
	configs.Config.Server.AllowedHosts = []string{"readeck.example.org"}

	configs.InitConfiguration()

	// Init test app
	ta := &TestApp{
		TmpDir: tmpDir,
		Users:  make(map[string]*TestUser),
	}

	// Email sender before init app
	configs.Config.Email.Host = "localhost"
	email.InitSender()
	email.Sender = ta

	// Init application
	app.InitApp()
	configs.Config.Commissioned = true

	// Load data
	fixtures := loadFixtures(t)
	ta.Users = fixtures.users

	// Start event manager
	startEventManager()

	// Init test server
	ta.Srv = server.New()
	err = app.InitServer(ta.Srv)
	if err != nil {
		t.Fatal(err)
	}

	return ta
}

// Close removes artifacts that were needed for testing.
func (ta *TestApp) Close(t *testing.T) {
	if err := db.Close(); err != nil {
		t.Logf("error closing database: %s", err)
	}
	if err := os.RemoveAll(ta.TmpDir); err != nil {
		t.Logf("error removing temporary folder: %s", err)
	}

	t.Logf("removed folder: %s", ta.TmpDir)

	// Reset the bus
	Events().Stop()
	Store().Clear()
}

// SendEmail implements email.sender interface and stores the last sent message.
func (ta *TestApp) SendEmail(msg *mail.Msg) error {
	buf := new(bytes.Buffer)
	msg.WriteTo(buf) // nolint:errcheck
	ta.LastEmail = buf.String()
	return nil
}

// Client is a thin HTTP client over the main server router.
type Client struct {
	*testing.T
	app *TestApp
	URL *url.URL
	Jar http.CookieJar
}

// NewClient creates a new Client instance.
func NewClient(t *testing.T, app *TestApp) *Client {
	jar, _ := cookiejar.New(nil)
	return &Client{
		T:   t,
		app: app,
		URL: &url.URL{Scheme: "https", Host: "readeck.example.org"},
		Jar: jar,
	}
}

// NewRequest returns a new http.Request instance ready for tests.
func (c *Client) NewRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	req.URL.Host = c.URL.Host
	req.URL.Scheme = c.URL.Scheme
	req.Host = c.URL.Host

	// Set request cookies
	for _, cookie := range c.Jar.Cookies(req.URL) {
		req.AddCookie(cookie)
	}

	return req
}

// NewFormRequest returns a new http.Request instance to be used for sending form data.
func (c *Client) NewFormRequest(method, target string, data url.Values) *http.Request {
	req := c.NewRequest(method, target, strings.NewReader(data.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req
}

// NewJSONRequest returns a new http.Request instance to be used for sending and receiving
// JSON data.
func (c *Client) NewJSONRequest(method, target string, data interface{}) *http.Request {
	var body io.Reader
	if data != nil {
		b, err := json.Marshal(data)
		if err != nil {
			c.Fatal("unable to marshal data")
		}
		body = bytes.NewReader(b)
	}

	req := c.NewRequest(method, target, body)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

// Request performs a request using httptest tools.
// It returns a Response instance that can be evaluated for testing
// purposes.
func (c *Client) Request(req *http.Request) *Response {
	w := httptest.NewRecorder()

	// Perform request
	c.app.Srv.ServeHTTP(w, req)

	// Update cookies from response
	//nolint:bodyclose
	if rc := w.Result().Cookies(); len(rc) > 0 {
		c.Jar.SetCookies(req.URL, rc)
	}

	// Prepare response instance
	rsp, err := NewResponse(w, req)
	rsp.Request = req
	if err != nil {
		c.Fatal(err)
	}

	return rsp
}

// Login logs in the given user.
func (c *Client) Login(username, password string) {
	r := c.Get("/login")
	if r.StatusCode != http.StatusOK {
		c.Fatalf("Invalid status %d", r.StatusCode)
	}

	data := url.Values{}
	data.Add("username", username)
	data.Add("password", password)
	r = c.PostForm("/login", data)
	if r.StatusCode != http.StatusSeeOther {
		c.Fatalf("Invalid status %d", r.StatusCode)
	}
}

// Logout empties the client's cookie jar.
func (c *Client) Logout() {
	c.Jar, _ = cookiejar.New(nil)
}

// Cookies returns the stored cookie for the current client session.
func (c *Client) Cookies() []*http.Cookie {
	return c.Jar.Cookies(c.URL)
}

// Get performs a GET request on the given path.
func (c *Client) Get(target string) *Response {
	return c.Request(c.NewRequest("GET", target, nil))
}

// PostForm performs a POST request on the given path with some data.
func (c *Client) PostForm(target string, data url.Values) *Response {
	return c.Request(c.NewFormRequest("POST", target, data))
}

// RequestJSON performs a JSON HTTP requests (sending and receiving data).
func (c *Client) RequestJSON(method string, target string, data interface{}) *Response {
	return c.Request(c.NewJSONRequest(method, target, data))
}

// renderTemplateString executes a template string using some properties of the client.
// (Users, URL). Extra data can be sent using the extra map.
func (c *Client) renderTemplateString(src string, extra map[string]interface{}) (string, error) {
	tpl, err := template.New("").Parse(src)
	if err != nil {
		return "", err
	}
	buf := bytes.Buffer{}
	data := map[string]any{
		"Users": c.app.Users,
		"URL":   "http://" + c.URL.Host,
	}
	maps.Copy(data, extra)

	if err = tpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// renderTemplate executes [renderTemplateString] on the given variable when its
// type is string, map[string]any and []any. This lets us use template values
// in JSON payloads.
func (c *Client) renderTemplate(src any, extra map[string]any) (any, error) {
	var err error
	switch s := src.(type) {
	case string:
		return c.renderTemplateString(s, extra)
	case map[string]any:
		for k, v := range s {
			s[k], err = c.renderTemplate(v, extra)
			if err != nil {
				return nil, err
			}
		}
		return s, nil
	case []any:
		for i, v := range s {
			s[i], err = c.renderTemplate(v, extra)
			if err != nil {
				return nil, err
			}
		}
		return s, nil
	}

	return src, nil
}

// Response is a wrapper around http.Response where the body is stored and
// the HTML (when applicable) is parsed in advance.
type Response struct {
	*http.Response
	URL      *url.URL
	Redirect string
	Body     []byte
	HTML     *html.Node
	JSON     any
}

// NewResponse returns a Response instance based on the ResponseRecorder
// given in input.
func NewResponse(rec *httptest.ResponseRecorder, req *http.Request) (*Response, error) {
	var err error
	r := &Response{Response: rec.Result()} //nolint:bodyclose

	u2 := new(url.URL)
	*u2 = *req.URL
	u2.Scheme = "http"

	r.URL = u2

	// Set redirect if any
	if loc := r.Header.Get("location"); loc != "" {
		redir, err := r.URL.Parse(loc)
		if err != nil {
			return nil, err
		}
		if redir.Host == u2.Host {
			redir.Scheme = ""
			redir.Host = ""
		}
		r.Redirect = redir.String()
	}

	// Read the response's body
	r.Body, err = io.ReadAll(r.Response.Body)
	if err != nil {
		return nil, err
	}

	// When an HTML response is received, parse it
	switch {
	case strings.HasPrefix(r.Header.Get("content-type"), "text/html"):
		r.HTML, err = html.Parse(bytes.NewReader(r.Body))
		if err != nil {
			return nil, err
		}
	case strings.HasPrefix(r.Header.Get("content-type"), "application/json"):
		err := json.Unmarshal(r.Body, &r.JSON)
		if err != nil {
			return nil, err
		}
	}

	return r, nil
}

// Path returns the path and querystring of the response URL.
func (r *Response) Path() string {
	u := new(url.URL)
	*u = *r.URL
	u.Scheme = ""
	u.Host = ""
	return u.String()
}

// AssertStatus checks the response's expected status.
func (r *Response) AssertStatus(t *testing.T, expected int) {
	require.Equal(t, expected, r.StatusCode)
}

// AssertRedirect checks that the expected target is present in a Location header.
func (r *Response) AssertRedirect(t *testing.T, expected string) {
	require.Regexp(t, expected, r.Redirect)
}

// AssertJSON checks that the response's JSON matches what we expect.
func (r *Response) AssertJSON(t *testing.T, expected string) {
	jsonassert.New(t).Assertf(string(r.Body), "%s", expected)
	if t.Failed() {
		t.Errorf("Received JSON: %s\n", string(r.Body))
		t.FailNow()
	}
}

// AssertJQ performs the JQ query and check every expected result argument.
// The test fails when the returned results count differs from the expected arguments.
func (r *Response) AssertJQ(t *testing.T, q string, expected ...any) {
	query, err := gojq.Parse(q)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	iter := query.RunWithContext(ctx, r.JSON)
	i := 0
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			t.Fatal(err)
		}

		require.LessOrEqual(t, i+1, len(expected), "not enough assertions")
		require.Equal(t, expected[i], v)
		i++
	}
	require.Len(t, expected, i, "not all assertions tested")
}

// RequestTest contains data that are used to perform requests and assert some results.
type RequestTest struct {
	Name           string
	Method         string
	Target         string
	Form           url.Values
	JSON           interface{}
	Headers        map[string]string
	ExpectStatus   int
	ExpectRedirect string
	ExpectJSON     string
	ExpectJQ       []any
	ExpectContains string
	Assert         func(*testing.T, *Response)
}

// RunRequestSequence performs a serie of requests using RequestTest instances.
func RunRequestSequence(t *testing.T, c *Client, user string, tests ...RequestTest) { //nolint:gocognit
	if user != "" {
		c.app.Users[user].Login(c)
		defer c.Logout()
	} else {
		c.Logout()
	}

	// Empty the event queue after a sequence
	defer Events().Clear()

	t.Run(fmt.Sprintf("sequence (%s)", user), func(t *testing.T) {
		history := []*Response{}
		templateData := map[string]any{
			"History": &history,
			"User":    c.app.Users[user],
		}

		for _, test := range tests {
			target, err := c.renderTemplateString(test.Target, templateData)
			if err != nil {
				t.Error(err)
				return
			}
			if test.Method == "" {
				test.Method = "GET"
			}

			name := test.Name
			if name == "" {
				name = fmt.Sprintf("%s %s", test.Method, target)
			}

			t.Run(name, func(t *testing.T) {
				var req *http.Request

				switch {
				case test.JSON != nil:
					var data any
					switch test.Method {
					case "POST", "PATCH", "PUT":
						data, err = c.renderTemplate(test.JSON, templateData)
						if err != nil {
							t.Error(err)
							return
						}
					default:
						data = nil
					}
					req = c.NewJSONRequest(test.Method, target, data)
					req.Header.Del("Cookie")
					if user != "" {
						req.Header.Set("Authorization", "Bearer "+c.app.Users[user].APIToken())
					} else {
						req.Header.Del("Authorization")
					}

				case test.Form != nil || test.Method == "POST":
					if test.Form == nil {
						test.Form = url.Values{}
					}
					req = c.NewFormRequest(test.Method, target, test.Form)

				default:
					req = c.NewRequest(test.Method, target, nil)
				}

				for k, v := range test.Headers {
					V, err := c.renderTemplateString(v, templateData)
					if err != nil {
						t.Error(err)
						return
					}
					req.Header.Add(k, V)
				}

				// Perform request
				rsp := c.Request(req)

				// Add request to history before all the asserts
				history = append([]*Response{rsp}, history...)

				if test.ExpectStatus != 0 {
					rsp.AssertStatus(t, test.ExpectStatus)
				}

				if test.ExpectRedirect != "" {
					rsp.AssertRedirect(t, test.ExpectRedirect)
				}

				if test.ExpectJSON != "" {
					s, err := c.renderTemplateString(test.ExpectJSON, templateData)
					if err != nil {
						t.Error(err)
						return
					}
					rsp.AssertJSON(t, s)
				}

				if test.ExpectContains != "" {
					require.Contains(t, string(rsp.Body), test.ExpectContains)
				}

				if test.Assert != nil {
					test.Assert(t, rsp)
				}
			})
		}
	})
}
