package oidctest

import (
	"html/template"
	"net/http"
	"net/url"
)

// signInHTML is the provider's sign-in page: a Username field, and one button per person for whoever
// clicks through by hand. It asks for no password; it is a test double, never a real login.
const signInHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Sign in (fake provider)</title>
<style>body{font-family:system-ui,sans-serif;max-width:32rem;margin:2rem auto;padding:0 1rem}form{margin:0 0 1rem}button{margin:.25rem 0}</style>
</head>
<body>
<main>
<h1>Sign in</h1>
<p>A fake OIDC provider for Grimoire's tests. Type a Username, with <code>+tag</code> for a fresh identity of that kind.</p>
{{if .Error}}<p role="alert">{{.Error}}</p>{{end}}
<form method="post" action="/authorize">
<input type="hidden" name="request" value="{{.Request}}">
<label for="username">Username</label>
<input id="username" name="username" autocomplete="off" required>
<button type="submit">Sign in</button>
</form>
<h2>People</h2>
{{range .People}}<form method="post" action="/authorize">
<input type="hidden" name="request" value="{{$.Request}}">
<button type="submit" name="username" value="{{.Username}}">{{.Name}} ({{.Username}}): {{range $i, $r := .Roles}}{{if $i}}, {{end}}{{$r}}{{else}}no roles{{end}}</button>
</form>
{{end}}
</main>
</body>
</html>
`

func signInPage() *template.Template {
	return template.Must(template.New("sign-in").Parse(signInHTML))
}

type signInView struct {
	Request string
	Error   string
	People  []Login
}

func (p *Provider) render(w http.ResponseWriter, status int, request, problem string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = p.page.Execute(w, signInView{Request: request, Error: problem, People: p.opts.People})
}

// valid reports whether a raw query is an authorization request for this client, sending the browser
// back to its registered redirect URI when it has one.
func (p *Provider) valid(rawQuery string) (url.Values, bool) {
	q, err := url.ParseQuery(rawQuery)
	if err != nil || q.Get("client_id") != p.opts.ClientID || q.Get("response_type") != "code" ||
		q.Get("code_challenge_method") != "S256" || q.Get("code_challenge") == "" || q.Get("redirect_uri") == "" ||
		(p.opts.RedirectURL != "" && q.Get("redirect_uri") != p.opts.RedirectURL) {
		return nil, false
	}
	return q, true
}

func (p *Provider) showSignIn(w http.ResponseWriter, r *http.Request) {
	if _, ok := p.valid(r.URL.RawQuery); !ok {
		http.Error(w, "not an authorization request for this client", http.StatusBadRequest)
		return
	}
	p.render(w, http.StatusOK, r.URL.RawQuery, "")
}

// signIn grants whom the form names and sends the browser back to the client with a code.
func (p *Provider) signIn(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	request := r.PostForm.Get("request")
	q, ok := p.valid(request)
	if !ok {
		http.Error(w, "not an authorization request for this client", http.StatusBadRequest)
		return
	}
	l, ok := person(p.opts.People, r.PostForm.Get("username"))
	if !ok {
		p.render(w, http.StatusUnauthorized, request, "No such person here. Pick one below.")
		return
	}
	code, state, err := p.Grant(p.opts.Issuer+"/authorize?"+request, l)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	back, err := url.Parse(q.Get("redirect_uri"))
	if err != nil {
		http.Error(w, "bad redirect_uri", http.StatusBadRequest)
		return
	}
	bq := back.Query()
	bq.Set("code", code)
	bq.Set("state", state)
	back.RawQuery = bq.Encode()
	// A test double sends the browser to the client's redirect URI, the registered one when it has one.
	http.Redirect(w, r, back.String(), http.StatusSeeOther) //nolint:gosec // checked against Options.RedirectURL in valid
}
