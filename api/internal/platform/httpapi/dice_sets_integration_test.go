//go:build integration

package httpapi_test

import (
	"bytes"
	"context"
	"fmt"
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
// diceTable is Aria, who made the dice set Ember; Bram, her Friend; and Cara, a stranger.
type diceTable struct {
	trusted, public  http.Handler
	aria, bram, cara string
	id, one          string
	created          map[string]any
}

func newDiceTable(t *testing.T) *diceTable {
	t.Helper()
	trusted, public, _, _ := accountServers(t)
	d := &diceTable{trusted: trusted, public: public}
	d.aria, d.bram, d.cara = setUp(t, trusted, public, "aria"), setUp(t, trusted, public, "bram"), setUp(t, trusted, public, "cara")
	send(public, http.MethodPost, "/api/v1/friend-requests", d.aria, "", `{"username":"bram"}`)
	request := firstRequestID(friendsOf(t, public, d.bram), "incoming")
	send(public, http.MethodPost, "/api/v1/friend-requests/"+request+"/accept", d.bram, "", "")
	rec := send(public, http.MethodPost, "/api/v1/dice-sets", d.aria, "", `{"name":" Ember ","design":`+emberDesign+`}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	d.created = decode(t, rec)
	d.id, _ = d.created["id"].(string)
	d.one = "/api/v1/dice-sets/" + d.id
	return d
}

func (d *diceTable) share(t *testing.T, sharing string) map[string]any {
	t.Helper()
	rec := send(d.public, http.MethodPut, d.one+"/sharing", d.aria, "", `{"sharing":"`+sharing+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("share with %s: %d", sharing, rec.Code)
	}
	return decode(t, rec)
}

func (d *diceTable) shared(t *testing.T, cookie string) string {
	t.Helper()
	return setNames(t, send(d.public, http.MethodGet, "/api/v1/dice-sets/shared", cookie, "", ""))
}

// copyFor takes a copy of Ember for cookie and returns the response.
func (d *diceTable) copyFor(cookie string) *httptest.ResponseRecorder {
	return send(d.public, http.MethodPost, d.one+"/copy", cookie, "", "")
}

func (d *diceTable) upload(t *testing.T, data []byte) map[string]any {
	t.Helper()
	rec := sendImage(d.public, d.one+"/image", d.aria, data)
	if rec.Code != http.StatusOK {
		t.Fatalf("upload: %d %s", rec.Code, rec.Body.String())
	}
	return decode(t, rec)
}

// waiting shares Ember with everyone with an uploaded picture, which waits for an Admin.
func (d *diceTable) waiting(t *testing.T) {
	t.Helper()
	d.share(t, "everyone")
	d.upload(t, pngBytes)
}

// seen is the version of the picture the Admin is shown in the review queue.
func (d *diceTable) seen(t *testing.T) string {
	t.Helper()
	items, _ := decode(t, send(d.trusted, http.MethodGet, "/api/v1/admin/dice-sets", "", "root", ""))["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("the review queue = %v", items)
	}
	version, _ := items[0].(map[string]any)["imageVersion"].(string)
	if len(version) != 64 {
		t.Fatalf("the picture's version = %q", version)
	}
	return version
}

func (d *diceTable) review(approve bool, picture string) *httptest.ResponseRecorder {
	return send(d.trusted, http.MethodPost, "/api/v1/admin/dice-sets/"+d.id+"/review", "", "root", fmt.Sprintf(`{"approve":%v,"picture":"%s"}`, approve, picture))
}

// approved is Ember shared with everyone, with a JPEG picture an Admin approved.
func (d *diceTable) approved(t *testing.T) {
	t.Helper()
	d.share(t, "everyone")
	d.upload(t, jpegBytes)
	if rec := d.review(true, d.seen(t)); rec.Code != http.StatusOK {
		t.Fatalf("approve: %d", rec.Code)
	}
}

func TestANewDiceSetIsPrivateTrimmedAndKeepsItsDesign(t *testing.T) {
	t.Parallel()
	ember := newDiceTable(t).created
	if ember["name"] != "Ember" || ember["sharing"] != "private" || ember["review"] != "none" || ember["mine"] != true || ember["copy"] != false ||
		ember["design"].(map[string]any)["dice"].(map[string]any)["d20"].(map[string]any)["pattern"] != "marble" {
		t.Fatalf("created %v", ember)
	}
}

func TestADiceSetWithABadNameOrDesignIsRefused(t *testing.T) {
	t.Parallel()
	d := newDiceTable(t)
	for name, body := range map[string]string{
		"a nameless set":     `{"name":" ","design":` + emberDesign + `}`,
		"an unknown die":     `{"name":"X","design":{"dice":{"d7":{"pattern":"plain","body":"#000000","numbers":"#ffffff"}}}}`,
		"an unknown pattern": `{"name":"X","design":{"dice":{"d20":{"pattern":"lava","body":"#000000","numbers":"#ffffff"}}}}`,
		"a colour by name":   `{"name":"X","design":{"dice":{"d20":{"pattern":"plain","body":"red","numbers":"#ffffff"}}}}`,
		"a picture far off":  `{"name":"X","design":{"dice":{"d20":{"pattern":"plain","body":"#000000","numbers":"#ffffff","image":{"x":9,"y":0,"scale":1,"rotation":0}}}}}`,
	} {
		t.Run(name, func(t *testing.T) {
			if rec := send(d.public, http.MethodPost, "/api/v1/dice-sets", d.aria, "", body); rec.Code != http.StatusUnprocessableEntity && rec.Code != http.StatusBadRequest {
				t.Fatalf("%d %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestTheOwnerListsTheirPrivateSet(t *testing.T) {
	t.Parallel()
	d := newDiceTable(t)
	if got := setNames(t, send(d.public, http.MethodGet, "/api/v1/dice-sets", d.aria, "", "")); got != "Ember" {
		t.Fatalf("Aria's sets = %s", got)
	}
}

func TestNobodyElseCanSeeCopyOrChangeAPrivateSet(t *testing.T) {
	t.Parallel()
	d := newDiceTable(t)
	for who, cookie := range map[string]string{"a Friend": d.bram, "a stranger": d.cara} {
		t.Run(who, func(t *testing.T) {
			if got := d.shared(t, cookie); got != "" {
				t.Fatalf("shown: %s", got)
			}
			for method, path := range map[string]string{http.MethodPost: d.one + "/copy", http.MethodPut: d.one, http.MethodDelete: d.one} {
				if rec := send(d.public, method, path, cookie, "", `{"name":"Mine","design":`+emberDesign+`}`); rec.Code != http.StatusNotFound {
					t.Errorf("%s %s: %d", method, path, rec.Code)
				}
			}
		})
	}
}

func TestAFriendSeesASetSharedWithFriendsAndAStrangerDoesNot(t *testing.T) {
	t.Parallel()
	d := newDiceTable(t)
	if got := d.share(t, "friends"); got["sharing"] != "friends" {
		t.Fatalf("sharing = %v", got["sharing"])
	}
	if got := d.shared(t, d.bram); got != "Ember" {
		t.Fatalf("shared with Bram = %s", got)
	}
	if got := d.shared(t, d.cara); got != "" {
		t.Fatalf("shown to a stranger: %s", got)
	}
	if rec := d.copyFor(d.cara); rec.Code != http.StatusNotFound {
		t.Fatalf("a stranger copied it: %d", rec.Code)
	}
}

func TestAFriendCopiesASharedSetOnceAsTheirOwnPrivateCopy(t *testing.T) {
	t.Parallel()
	d := newDiceTable(t)
	d.share(t, "friends")
	rec := d.copyFor(d.bram)
	copied := decode(t, rec)
	if rec.Code != http.StatusCreated || copied["copy"] != true || copied["mine"] != true || copied["by"] != "aria" || copied["name"] != "Ember" || copied["sharing"] != "private" {
		t.Fatalf("Bram's copy: %d %v", rec.Code, copied)
	}
	if rec := d.copyFor(d.bram); rec.Code != http.StatusConflict {
		t.Fatalf("a second copy: %d", rec.Code)
	}
}

func TestACopyCannotBeEditedOrSharedOn(t *testing.T) {
	t.Parallel()
	d := newDiceTable(t)
	d.share(t, "friends")
	copyID, _ := decode(t, d.copyFor(d.bram))["id"].(string)
	for name, call := range map[string]*httptest.ResponseRecorder{
		"edit":  send(d.public, http.MethodPut, "/api/v1/dice-sets/"+copyID, d.bram, "", `{"name":"Mine","design":`+emberDesign+`}`),
		"share": send(d.public, http.MethodPut, "/api/v1/dice-sets/"+copyID+"/sharing", d.bram, "", `{"sharing":"everyone"}`),
	} {
		t.Run(name, func(t *testing.T) {
			if call.Code != http.StatusConflict {
				t.Fatalf("%d", call.Code)
			}
		})
	}
}

func TestACopyKeepsItsDesignWhenTheOwnerChangesTheirsAndStopsSharing(t *testing.T) {
	t.Parallel()
	d := newDiceTable(t)
	d.share(t, "friends")
	d.copyFor(d.bram)
	other := strings.Replace(emberDesign, "marble", "speckled", 1)
	if rec := send(d.public, http.MethodPut, d.one, d.aria, "", `{"name":"Ember II","design":`+other+`}`); rec.Code != http.StatusOK || decode(t, rec)["name"] != "Ember II" {
		t.Fatalf("edit: %d", rec.Code)
	}
	d.share(t, "private")
	if got := d.shared(t, d.bram); got != "" {
		t.Fatalf("after sharing stopped Bram still sees: %s", got)
	}
	mine := decode(t, send(d.public, http.MethodGet, "/api/v1/dice-sets", d.bram, "", ""))["items"].([]any)
	if kept := mine[0].(map[string]any); len(mine) != 1 || kept["name"] != "Ember" || kept["design"].(map[string]any)["dice"].(map[string]any)["d20"].(map[string]any)["pattern"] != "marble" {
		t.Fatalf("Bram's kept copy = %v", mine)
	}
}

func TestASetOfPresetsSharedWithEveryoneShowsAtOnce(t *testing.T) {
	t.Parallel()
	d := newDiceTable(t)
	if got := d.share(t, "everyone"); got["review"] != "none" {
		t.Fatalf("review = %v", got["review"])
	}
	if got := d.shared(t, d.cara); got != "Ember" {
		t.Fatalf("everyone sees = %s", got)
	}
}

func TestAnOwnerCannotCopyTheirOwnSet(t *testing.T) {
	t.Parallel()
	d := newDiceTable(t)
	d.share(t, "everyone")
	if rec := d.copyFor(d.aria); rec.Code != http.StatusNotFound {
		t.Fatalf("copy = %d", rec.Code)
	}
}

func TestAFileThatIsNoPictureIsRefused(t *testing.T) {
	t.Parallel()
	d := newDiceTable(t)
	if rec := sendImage(d.public, d.one+"/image", d.aria, []byte("not a picture")); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("upload = %d", rec.Code)
	}
}

func TestOnlyTheOwnerPutsAPictureOnTheirSet(t *testing.T) {
	t.Parallel()
	d := newDiceTable(t)
	if rec := sendImage(d.public, d.one+"/image", d.bram, pngBytes); rec.Code != http.StatusNotFound {
		t.Fatalf("upload = %d", rec.Code)
	}
}

func TestAPictureIsServedBackInItsOwnFormat(t *testing.T) {
	t.Parallel()
	for kind, data := range map[string][]byte{"image/webp": webpBytes, "image/jpeg": jpegBytes, "image/png": pngBytes} {
		t.Run(kind, func(t *testing.T) {
			d := newDiceTable(t)
			d.upload(t, data)
			if rec := send(d.public, http.MethodGet, d.one+"/image", d.aria, "", ""); rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != kind {
				t.Fatalf("%d %s", rec.Code, rec.Header().Get("Content-Type"))
			}
		})
	}
}

func TestASetSharedWithEveryoneWithAnUploadedPictureWaitsForAnAdmin(t *testing.T) {
	t.Parallel()
	d := newDiceTable(t)
	d.share(t, "everyone")
	if got := d.upload(t, pngBytes); got["review"] != "pending" || got["hasImage"] != true {
		t.Fatalf("upload = %v", got)
	}
	if got := d.shared(t, d.cara); got != "" {
		t.Fatalf("an unchecked picture shows to everyone: %s", got)
	}
}

func TestWhileASetWaitsAStrangerCannotCopyItOrFetchItsPicture(t *testing.T) {
	t.Parallel()
	d := newDiceTable(t)
	d.waiting(t)
	for name, call := range map[string]*httptest.ResponseRecorder{
		"copy":    d.copyFor(d.cara),
		"picture": send(d.public, http.MethodGet, d.one+"/image", d.cara, "", ""),
	} {
		t.Run(name, func(t *testing.T) {
			if call.Code != http.StatusNotFound {
				t.Fatalf("%d", call.Code)
			}
		})
	}
}

func TestWhileASetWaitsAFriendStillSeesItAndItsPicture(t *testing.T) {
	t.Parallel()
	d := newDiceTable(t)
	d.waiting(t)
	if rec := send(d.public, http.MethodGet, d.one+"/image", d.bram, "", ""); rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("picture = %d", rec.Code)
	}
	if got := d.shared(t, d.bram); got != "Ember" {
		t.Fatalf("a Friend sees = %s", got)
	}
}

func TestOnlyAnAdminSeesTheReviewQueueAndThePicturesInIt(t *testing.T) {
	t.Parallel()
	d := newDiceTable(t)
	d.waiting(t)
	if rec := send(d.public, http.MethodGet, "/api/v1/admin/dice-sets", d.aria, "", ""); rec.Code != http.StatusForbidden {
		t.Fatalf("the queue for a non-Admin: %d", rec.Code)
	}
	if got := setNames(t, send(d.trusted, http.MethodGet, "/api/v1/admin/dice-sets", "", "root", "")); got != "Ember" {
		t.Fatalf("the queue = %s", got)
	}
	if rec := send(d.trusted, http.MethodGet, d.one+"/image", "", "root", ""); rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("the Admin's look at the picture: %d", rec.Code)
	}
}

func TestAnOwnerCannotApproveTheirOwnSet(t *testing.T) {
	t.Parallel()
	d := newDiceTable(t)
	d.waiting(t)
	if rec := send(d.public, http.MethodPost, "/api/v1/admin/dice-sets/"+d.id+"/review", d.aria, "", `{"approve":true,"picture":"`+d.seen(t)+`"}`); rec.Code != http.StatusForbidden {
		t.Fatalf("approve = %d", rec.Code)
	}
}

func TestARejectedPictureStaysHidden(t *testing.T) {
	t.Parallel()
	d := newDiceTable(t)
	d.waiting(t)
	if rec := d.review(false, d.seen(t)); rec.Code != http.StatusOK || decode(t, rec)["review"] != "rejected" {
		t.Fatalf("reject = %d", rec.Code)
	}
	if got := d.shared(t, d.cara); got != "" {
		t.Fatalf("everyone sees: %s", got)
	}
}

// swapped is a waiting set whose picture was swapped after the Admin saw the first; it returns the
// version of each.
func (d *diceTable) swapped(t *testing.T) (first, second string) {
	t.Helper()
	d.waiting(t)
	first = d.seen(t)
	if got := d.upload(t, jpegBytes); got["review"] != "pending" {
		t.Fatalf("a new picture: %v", got["review"])
	}
	return first, d.seen(t)
}

func TestApprovingAPictureThatWasSwappedApprovesNothing(t *testing.T) {
	t.Parallel()
	d := newDiceTable(t)
	first, second := d.swapped(t)
	if rec := d.review(true, first); rec.Code != http.StatusConflict {
		t.Fatalf("approve = %d", rec.Code)
	}
	if got := d.shared(t, d.cara); got != "" {
		t.Fatalf("everyone sees: %s", got)
	}
	if second == first {
		t.Fatal("a new picture keeps the old version")
	}
}

func TestAPictureIsNamedByTheWholeHashOfItsContent(t *testing.T) {
	t.Parallel()
	d := newDiceTable(t)
	_, second := d.swapped(t)
	if rec := d.review(true, second[:12]+strings.Repeat("0", 52)); rec.Code != http.StatusConflict {
		t.Fatalf("a picture that starts like it: %d", rec.Code)
	}
	if rec := d.review(true, second[:12]); rec.Code != http.StatusBadRequest {
		t.Fatalf("half a name: %d", rec.Code)
	}
}

func TestTheAdminIsShownOnlyThePictureTheLinkNames(t *testing.T) {
	t.Parallel()
	d := newDiceTable(t)
	first, second := d.swapped(t)
	for version, want := range map[string]int{second: http.StatusOK, first: http.StatusNotFound, second[:12] + strings.Repeat("0", 52): http.StatusNotFound, "x": http.StatusBadRequest} {
		t.Run(version, func(t *testing.T) {
			if rec := send(d.trusted, http.MethodGet, d.one+"/image?v="+version, "", "root", ""); rec.Code != want {
				t.Fatalf("%d, want %d", rec.Code, want)
			}
		})
	}
	if link, _ := decode(t, send(d.public, http.MethodGet, "/api/v1/dice-sets", d.aria, "", ""))["items"].([]any)[0].(map[string]any)["imageUrl"].(string); link != d.one+"/image?v="+second {
		t.Fatalf("the link to the picture = %s", link)
	}
}

func TestAnApprovedSetLeavesTheQueueAndCannotBeReviewedAgain(t *testing.T) {
	t.Parallel()
	d := newDiceTable(t)
	d.share(t, "everyone")
	d.upload(t, jpegBytes)
	version := d.seen(t)
	if rec := d.review(true, version); rec.Code != http.StatusOK || decode(t, rec)["review"] != "approved" {
		t.Fatalf("approve = %d", rec.Code)
	}
	if got := setNames(t, send(d.trusted, http.MethodGet, "/api/v1/admin/dice-sets", "", "root", "")); got != "" {
		t.Fatalf("the queue = %s", got)
	}
	if rec := d.review(false, version); rec.Code != http.StatusConflict {
		t.Fatalf("a second review = %d", rec.Code)
	}
}

func TestEveryoneCopiesAnApprovedSetPictureAndAll(t *testing.T) {
	t.Parallel()
	d := newDiceTable(t)
	d.approved(t)
	rec := d.copyFor(d.cara)
	carasID, _ := decode(t, rec)["id"].(string)
	if rec.Code != http.StatusCreated || decode(t, rec)["hasImage"] != true {
		t.Fatalf("copy = %d", rec.Code)
	}
	if rec := send(d.public, http.MethodGet, "/api/v1/dice-sets/"+carasID+"/image", d.cara, "", ""); rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatalf("the picture on the copy: %d", rec.Code)
	}
}

func TestOnlyYourOwnSetCanBeChosen(t *testing.T) {
	t.Parallel()
	d := newDiceTable(t)
	if rec := send(d.public, http.MethodPut, "/api/v1/dice-sets/chosen", d.cara, "", `{"diceSetId":"`+d.id+`"}`); rec.Code != http.StatusNotFound {
		t.Fatalf("choose = %d", rec.Code)
	}
}

func TestAChosenSetStaysChosenUntilItIsDeleted(t *testing.T) {
	t.Parallel()
	d := newDiceTable(t)
	d.approved(t)
	carasID, _ := decode(t, d.copyFor(d.cara))["id"].(string)
	if rec := send(d.public, http.MethodPut, "/api/v1/dice-sets/chosen", d.cara, "", `{"diceSetId":"`+carasID+`"}`); rec.Code != http.StatusNoContent {
		t.Fatalf("choose = %d", rec.Code)
	}
	if list := decode(t, send(d.public, http.MethodGet, "/api/v1/dice-sets", d.cara, "", "")); list["chosen"] != carasID {
		t.Fatalf("chosen = %v", list["chosen"])
	}
	if rec := send(d.public, http.MethodDelete, "/api/v1/dice-sets/"+carasID, d.cara, "", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete = %d", rec.Code)
	}
	if list := decode(t, send(d.public, http.MethodGet, "/api/v1/dice-sets", d.cara, "", "")); list["chosen"] != nil || len(list["items"].([]any)) != 0 {
		t.Fatalf("after deleting the chosen set = %v", list)
	}
}

func TestChoosingNoSetIsAllowed(t *testing.T) {
	t.Parallel()
	d := newDiceTable(t)
	if rec := send(d.public, http.MethodPut, "/api/v1/dice-sets/chosen", d.aria, "", `{}`); rec.Code != http.StatusNoContent {
		t.Fatalf("choose none = %d", rec.Code)
	}
}

func TestTakingThePictureOffEndsItsReview(t *testing.T) {
	t.Parallel()
	d := newDiceTable(t)
	d.waiting(t)
	if rec := send(d.public, http.MethodDelete, d.one+"/image", d.aria, "", ""); rec.Code != http.StatusOK || decode(t, rec)["hasImage"] != false || decode(t, rec)["review"] != "none" {
		t.Fatalf("take it off = %d", rec.Code)
	}
}
