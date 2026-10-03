package httpapi_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/httpapi"
)

const emberDesign = `{"dice":{"d20":{"pattern":"marble","body":"#7a1f1a","numbers":"#f3d27a"},"d6":{"pattern":"plain","body":"#1f3a5a","numbers":"#ffffff"}}}`

// sendImage uploads a picture as a signed-in Account.
func sendImage(h http.Handler, path, cookie string, data []byte) *httptest.ResponseRecorder {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, path, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/octet-stream")
	req.AddCookie(&http.Cookie{Name: httpapi.SessionCookie, Value: cookie, Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func setNames(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var names []string
	for _, e := range decode(t, rec)["items"].([]any) {
		names = append(names, e.(map[string]any)["name"].(string))
	}
	return strings.Join(names, ",")
}

// An Account makes Dice Sets and chooses one to roll with. A set is private until shared: with Friends,
// or with everyone, where one with an uploaded picture waits for an Admin first. Whoever a set is shared
// with takes a copy they cannot edit, and keeps it when the sharing stops.
func TestDiceSetsAndWhoTheyAreSharedWith(t *testing.T) {
	t.Parallel()
	trusted, public, _, _ := accountServers(t)
	aria, bram, cara := setUp(t, trusted, public, "aria"), setUp(t, trusted, public, "bram"), setUp(t, trusted, public, "cara")
	send(public, http.MethodPost, "/api/v1/friend-requests", aria, "", `{"username":"bram"}`)
	request := firstRequestID(friendsOf(t, public, bram), "incoming")
	send(public, http.MethodPost, "/api/v1/friend-requests/"+request+"/accept", bram, "", "")

	rec := send(public, http.MethodPost, "/api/v1/dice-sets", aria, "", `{"name":" Ember ","design":`+emberDesign+`}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	ember := decode(t, rec)
	id, _ := ember["id"].(string)
	if rec.Code != http.StatusCreated || ember["name"] != "Ember" || ember["sharing"] != "private" || ember["review"] != "none" || ember["mine"] != true || ember["copy"] != false ||
		ember["design"].(map[string]any)["dice"].(map[string]any)["d20"].(map[string]any)["pattern"] != "marble" {
		t.Fatalf("create: %d %v", rec.Code, ember)
	}
	one := "/api/v1/dice-sets/" + id
	for name, body := range map[string]string{
		"a nameless set":     `{"name":" ","design":` + emberDesign + `}`,
		"an unknown die":     `{"name":"X","design":{"dice":{"d7":{"pattern":"plain","body":"#000000","numbers":"#ffffff"}}}}`,
		"an unknown pattern": `{"name":"X","design":{"dice":{"d20":{"pattern":"lava","body":"#000000","numbers":"#ffffff"}}}}`,
		"a colour by name":   `{"name":"X","design":{"dice":{"d20":{"pattern":"plain","body":"red","numbers":"#ffffff"}}}}`,
		"a picture far off":  `{"name":"X","design":{"dice":{"d20":{"pattern":"plain","body":"#000000","numbers":"#ffffff","image":{"x":9,"y":0,"scale":1,"rotation":0}}}}}`,
	} {
		if rec := send(public, http.MethodPost, "/api/v1/dice-sets", aria, "", body); rec.Code != http.StatusUnprocessableEntity && rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: %d %s", name, rec.Code, rec.Body.String())
		}
	}

	// Private: only Aria has it, and nobody else can read, copy or change it.
	if got := setNames(t, send(public, http.MethodGet, "/api/v1/dice-sets", aria, "", "")); got != "Ember" {
		t.Fatalf("Aria's sets = %s", got)
	}
	for who, cookie := range map[string]string{"a Friend": bram, "a stranger": cara} {
		if got := setNames(t, send(public, http.MethodGet, "/api/v1/dice-sets/shared", cookie, "", "")); got != "" {
			t.Fatalf("a private set shows to %s: %s", who, got)
		}
		for method, path := range map[string]string{http.MethodPost: one + "/copy", http.MethodPut: one, http.MethodDelete: one} {
			if rec := send(public, method, path, cookie, "", `{"name":"Mine","design":`+emberDesign+`}`); rec.Code != http.StatusNotFound {
				t.Fatalf("%s %s by %s: %d", method, path, who, rec.Code)
			}
		}
	}

	// Shared with Friends: Bram sees it and takes a copy; Cara does not.
	if rec := send(public, http.MethodPut, one+"/sharing", aria, "", `{"sharing":"friends"}`); rec.Code != http.StatusOK || decode(t, rec)["sharing"] != "friends" {
		t.Fatalf("share with friends: %d", rec.Code)
	}
	if got := setNames(t, send(public, http.MethodGet, "/api/v1/dice-sets/shared", bram, "", "")); got != "Ember" {
		t.Fatalf("shared with Bram = %s", got)
	}
	if got := setNames(t, send(public, http.MethodGet, "/api/v1/dice-sets/shared", cara, "", "")); got != "" {
		t.Fatalf("a Friends-only set shows to a stranger: %s", got)
	}
	if rec := send(public, http.MethodPost, one+"/copy", cara, "", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("a stranger copies a Friends-only set: %d", rec.Code)
	}
	rec = send(public, http.MethodPost, one+"/copy", bram, "", "")
	copied := decode(t, rec)
	copyID, _ := copied["id"].(string)
	if rec.Code != http.StatusCreated || copied["copy"] != true || copied["mine"] != true || copied["by"] != "aria" || copied["name"] != "Ember" || copied["sharing"] != "private" {
		t.Fatalf("Bram's copy: %d %v", rec.Code, copied)
	}
	if rec := send(public, http.MethodPost, one+"/copy", bram, "", ""); rec.Code != http.StatusConflict {
		t.Fatalf("a second copy of the same set: %d", rec.Code)
	}
	// A copy cannot be edited or shared on; it can be rolled with and thrown away.
	for name, call := range map[string]*httptest.ResponseRecorder{
		"edit":  send(public, http.MethodPut, "/api/v1/dice-sets/"+copyID, bram, "", `{"name":"Mine","design":`+emberDesign+`}`),
		"share": send(public, http.MethodPut, "/api/v1/dice-sets/"+copyID+"/sharing", bram, "", `{"sharing":"everyone"}`),
	} {
		if call.Code != http.StatusConflict {
			t.Fatalf("%s a copy: %d", name, call.Code)
		}
	}

	// Aria changes hers and stops sharing: Bram keeps the copy as it was.
	other := strings.Replace(emberDesign, "marble", "speckled", 1)
	if rec := send(public, http.MethodPut, one, aria, "", `{"name":"Ember II","design":`+other+`}`); rec.Code != http.StatusOK || decode(t, rec)["name"] != "Ember II" {
		t.Fatalf("edit: %d", rec.Code)
	}
	send(public, http.MethodPut, one+"/sharing", aria, "", `{"sharing":"private"}`)
	if got := setNames(t, send(public, http.MethodGet, "/api/v1/dice-sets/shared", bram, "", "")); got != "" {
		t.Fatalf("after sharing stopped Bram still sees: %s", got)
	}
	mine := decode(t, send(public, http.MethodGet, "/api/v1/dice-sets", bram, "", ""))["items"].([]any)
	if kept := mine[0].(map[string]any); len(mine) != 1 || kept["name"] != "Ember" || kept["design"].(map[string]any)["dice"].(map[string]any)["d20"].(map[string]any)["pattern"] != "marble" {
		t.Fatalf("Bram's kept copy = %v", mine)
	}

	// Shared with everyone: a set of presets shows at once; one with an uploaded picture waits for an Admin.
	if rec := send(public, http.MethodPut, one+"/sharing", aria, "", `{"sharing":"everyone"}`); rec.Code != http.StatusOK || decode(t, rec)["review"] != "none" {
		t.Fatalf("share presets with everyone: %d", rec.Code)
	}
	if got := setNames(t, send(public, http.MethodGet, "/api/v1/dice-sets/shared", cara, "", "")); got != "Ember II" {
		t.Fatalf("everyone sees = %s", got)
	}
	if rec := send(public, http.MethodPost, one+"/copy", aria, "", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("a copy of your own set, shared with everyone: %d", rec.Code)
	}
	if rec := sendImage(public, one+"/image", aria, []byte("not a picture")); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("a file that is no picture: %d", rec.Code)
	}
	if rec := sendImage(public, one+"/image", bram, pngBytes); rec.Code != http.StatusNotFound {
		t.Fatalf("someone else's picture on Aria's set: %d", rec.Code)
	}
	// Any of the three picture formats is served back as what it is.
	for kind, data := range map[string][]byte{"image/webp": webpBytes, "image/jpeg": jpegBytes} {
		sendImage(public, one+"/image", aria, data)
		if rec := send(public, http.MethodGet, one+"/image", aria, "", ""); rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != kind {
			t.Fatalf("a %s picture: %d %s", kind, rec.Code, rec.Header().Get("Content-Type"))
		}
	}
	if rec := sendImage(public, one+"/image", aria, pngBytes); rec.Code != http.StatusOK || decode(t, rec)["review"] != "pending" || decode(t, rec)["hasImage"] != true {
		t.Fatalf("upload: %d %s", rec.Code, rec.Body.String())
	}
	if got := setNames(t, send(public, http.MethodGet, "/api/v1/dice-sets/shared", cara, "", "")); got != "" {
		t.Fatalf("an unchecked picture shows to everyone: %s", got)
	}
	// While it waits a stranger can neither copy it nor fetch its picture; a Friend can do both.
	for name, call := range map[string]*httptest.ResponseRecorder{
		"copy":    send(public, http.MethodPost, one+"/copy", cara, "", ""),
		"picture": send(public, http.MethodGet, one+"/image", cara, "", ""),
	} {
		if call.Code != http.StatusNotFound {
			t.Fatalf("a stranger's %s of a set that waits: %d", name, call.Code)
		}
	}
	if rec := send(public, http.MethodGet, one+"/image", bram, "", ""); rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("a Friend fetches the picture: %d", rec.Code)
	}
	// Friends still see it: it was shared with them all along.
	if got := setNames(t, send(public, http.MethodGet, "/api/v1/dice-sets/shared", bram, "", "")); got != "Ember II" {
		t.Fatalf("a Friend sees the set that waits = %s", got)
	}
	if rec := send(public, http.MethodGet, "/api/v1/admin/dice-sets", aria, "", ""); rec.Code != http.StatusForbidden {
		t.Fatalf("the review queue for a non-Admin: %d", rec.Code)
	}
	if got := setNames(t, send(trusted, http.MethodGet, "/api/v1/admin/dice-sets", "", "root", "")); got != "Ember II" {
		t.Fatalf("the review queue = %s", got)
	}
	if rec := send(trusted, http.MethodGet, one+"/image", "", "root", ""); rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("the Admin looks at the picture: %d", rec.Code)
	}
	// An Admin's decision names the picture they looked at.
	seen := func() string {
		t.Helper()
		items, _ := decode(t, send(trusted, http.MethodGet, "/api/v1/admin/dice-sets", "", "root", ""))["items"].([]any)
		if len(items) != 1 {
			t.Fatalf("the review queue = %v", items)
		}
		version, _ := items[0].(map[string]any)["imageVersion"].(string)
		if len(version) != 64 {
			t.Fatalf("the picture's version = %q", version)
		}
		return version
	}
	first := seen()
	if rec := send(public, http.MethodPost, "/api/v1/admin/dice-sets/"+id+"/review", aria, "", `{"approve":true,"picture":"`+first+`"}`); rec.Code != http.StatusForbidden {
		t.Fatalf("approving your own set: %d", rec.Code)
	}
	if rec := send(trusted, http.MethodPost, "/api/v1/admin/dice-sets/"+id+"/review", "", "root", `{"approve":false,"picture":"`+first+`"}`); rec.Code != http.StatusOK || decode(t, rec)["review"] != "rejected" {
		t.Fatalf("reject: %d", rec.Code)
	}
	if got := setNames(t, send(public, http.MethodGet, "/api/v1/dice-sets/shared", cara, "", "")); got != "" {
		t.Fatalf("a rejected picture shows to everyone: %s", got)
	}
	// A new picture asks again; approved, everyone sees the set and can take a copy, picture and all.
	if rec := sendImage(public, one+"/image", aria, jpegBytes); rec.Code != http.StatusOK || decode(t, rec)["review"] != "pending" {
		t.Fatalf("a new picture: %d", rec.Code)
	}
	// The owner swapped the picture after the Admin looked: approving the old one approves nothing.
	if rec := send(trusted, http.MethodPost, "/api/v1/admin/dice-sets/"+id+"/review", "", "root", `{"approve":true,"picture":"`+first+`"}`); rec.Code != http.StatusConflict {
		t.Fatalf("approving a picture that is no longer on the set: %d", rec.Code)
	}
	if got := setNames(t, send(public, http.MethodGet, "/api/v1/dice-sets/shared", cara, "", "")); got != "" {
		t.Fatalf("after approving a swapped picture everyone sees: %s", got)
	}
	second := seen()
	if second == first {
		t.Fatal("a new picture keeps the old version")
	}
	// A picture is named by the whole hash of its content: one that starts like it is another picture,
	// and half a name is no name.
	if rec := send(trusted, http.MethodPost, "/api/v1/admin/dice-sets/"+id+"/review", "", "root", `{"approve":true,"picture":"`+second[:12]+strings.Repeat("0", 52)+`"}`); rec.Code != http.StatusConflict {
		t.Fatalf("approving a picture that starts like the one on the set: %d", rec.Code)
	}
	if rec := send(trusted, http.MethodPost, "/api/v1/admin/dice-sets/"+id+"/review", "", "root", `{"approve":true,"picture":"`+second[:12]+`"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("approving half a name: %d", rec.Code)
	}
	// What the Admin is shown is the picture the link names, or nothing: never the one that replaced it.
	for version, want := range map[string]int{second: http.StatusOK, first: http.StatusNotFound, second[:12] + strings.Repeat("0", 52): http.StatusNotFound, "x": http.StatusBadRequest} {
		if rec := send(trusted, http.MethodGet, one+"/image?v="+version, "", "root", ""); rec.Code != want {
			t.Fatalf("the picture asked for as %s: %d, want %d", version, rec.Code, want)
		}
	}
	if link, _ := decode(t, send(public, http.MethodGet, "/api/v1/dice-sets", aria, "", ""))["items"].([]any)[0].(map[string]any)["imageUrl"].(string); link != one+"/image?v="+second {
		t.Fatalf("the link to the picture = %s", link)
	}
	if rec := send(trusted, http.MethodPost, "/api/v1/admin/dice-sets/"+id+"/review", "", "root", `{"approve":true,"picture":"`+second+`"}`); rec.Code != http.StatusOK || decode(t, rec)["review"] != "approved" {
		t.Fatalf("approve: %d", rec.Code)
	}
	if got := setNames(t, send(trusted, http.MethodGet, "/api/v1/admin/dice-sets", "", "root", "")); got != "" {
		t.Fatalf("the queue after the review = %s", got)
	}
	if rec := send(trusted, http.MethodPost, "/api/v1/admin/dice-sets/"+id+"/review", "", "root", `{"approve":false,"picture":"`+second+`"}`); rec.Code != http.StatusConflict {
		t.Fatalf("reviewing a set that no longer waits: %d", rec.Code)
	}
	rec = send(public, http.MethodPost, one+"/copy", cara, "", "")
	carasID, _ := decode(t, rec)["id"].(string)
	if rec.Code != http.StatusCreated || decode(t, rec)["hasImage"] != true {
		t.Fatalf("Cara's copy: %d", rec.Code)
	}
	if rec := send(public, http.MethodGet, "/api/v1/dice-sets/"+carasID+"/image", cara, "", ""); rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatalf("the picture on Cara's copy: %d", rec.Code)
	}

	// Choosing: one of your own sets, or none.
	if rec := send(public, http.MethodPut, "/api/v1/dice-sets/chosen", cara, "", `{"diceSetId":"`+id+`"}`); rec.Code != http.StatusNotFound {
		t.Fatalf("choosing someone else's set: %d", rec.Code)
	}
	if rec := send(public, http.MethodPut, "/api/v1/dice-sets/chosen", cara, "", `{"diceSetId":"`+carasID+`"}`); rec.Code != http.StatusNoContent {
		t.Fatalf("choose: %d", rec.Code)
	}
	if list := decode(t, send(public, http.MethodGet, "/api/v1/dice-sets", cara, "", "")); list["chosen"] != carasID {
		t.Fatalf("chosen = %v", list["chosen"])
	}
	if rec := send(public, http.MethodDelete, "/api/v1/dice-sets/"+carasID, cara, "", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d", rec.Code)
	}
	if list := decode(t, send(public, http.MethodGet, "/api/v1/dice-sets", cara, "", "")); list["chosen"] != nil || len(list["items"].([]any)) != 0 {
		t.Fatalf("after deleting the chosen set = %v", list)
	}
	if rec := send(public, http.MethodPut, "/api/v1/dice-sets/chosen", aria, "", `{}`); rec.Code != http.StatusNoContent {
		t.Fatalf("choosing none: %d", rec.Code)
	}
	if rec := send(public, http.MethodDelete, one+"/image", aria, "", ""); rec.Code != http.StatusOK || decode(t, rec)["hasImage"] != false || decode(t, rec)["review"] != "none" {
		t.Fatalf("taking the picture off: %d", rec.Code)
	}
}
