package letters_test

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/mail/letters"
)

var update = flag.Bool("update", false, "rewrite the golden emails")

// sample is a representative Letter of each kind.
func sample(k letters.Kind) letters.Letter {
	l := letters.Letter{Kind: k, Subject: "", Nickname: "Aria", Heading: "", Paragraphs: nil, Action: &letters.Action{Label: "Open", URL: "https://grimoire.example/x"}, Items: nil}
	l.Heading, l.Paragraphs = string(k), []string{"Something happened that you asked to hear about."}
	if k == letters.Digest {
		l.Heading, l.Action, l.Paragraphs = "Since your last Digest", nil, nil
		l.Items = []letters.Item{{Title: "Bram wrote to you", Body: "See you <Friday>", URL: "https://grimoire.example/conversations/1"}, {Title: "Kara can level up", Body: "", URL: ""}}
	}
	if k == letters.AccountDisabled {
		l.Heading, l.Action = "Your Account was disabled", nil
		l.Paragraphs = []string{"An Admin disabled your Account. Every device was signed out."}
	}
	return l
}

// Every email renders from the one template, matching its golden file.
func TestEveryEmailRendersFromOneTemplate(t *testing.T) {
	t.Parallel()
	if len(letters.Kinds()) != 14 {
		t.Fatalf("kinds = %d", len(letters.Kinds()))
	}
	for _, k := range letters.Kinds() {
		subject, text, html, err := letters.Render(sample(k))
		if err != nil {
			t.Fatalf("%s: %v", k, err)
		}
		got := "Subject: " + subject + "\n\n" + text + "\n====\n" + html
		path := filepath.Join("testdata", string(k)+".golden")
		if *update {
			if err := os.WriteFile(path, []byte(got), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		want, err := os.ReadFile(path) //nolint:gosec // a test fixture
		if err != nil {
			t.Fatalf("%s: %v (run with -update)", k, err)
		}
		if got != string(want) {
			t.Errorf("%s differs from %s", k, path)
		}
		if !strings.Contains(html, `max-width:560px`) || !strings.Contains(html, `background:#14110d`) || subject == "" {
			t.Errorf("%s is not the shared dark, centred template", k)
		}
	}
}

// HTML is escaped, a Letter may set its own subject, and security mail says it cannot be turned off.
func TestLettersEscapeAndExplain(t *testing.T) {
	t.Parallel()
	_, text, html, _ := letters.Render(sample(letters.Digest))
	if !strings.Contains(html, "See you &lt;Friday&gt;") || !strings.Contains(text, "See you <Friday>") {
		t.Error("the Digest body is not escaped in HTML only")
	}
	subject, _, _, _ := letters.Render(letters.Letter{Kind: letters.Conversation, Subject: "Bram wrote to you"})
	if subject != "Bram wrote to you" {
		t.Errorf("subject = %q", subject)
	}
	_, text, _, _ = letters.Render(letters.Letter{Kind: letters.NewSignIn})
	if !letters.Security(letters.NewSignIn) || letters.Security(letters.Digest) || !strings.Contains(text, "cannot be turned off") {
		t.Errorf("security footer = %q", text)
	}
}
