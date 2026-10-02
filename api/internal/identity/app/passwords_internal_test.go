package app

import (
	"strings"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/identity/domain"
)

func TestPasswordsHashAndVerify(t *testing.T) {
	t.Parallel()
	p := Passwords{MemoryKiB: 64, Time: 1, Threads: 1}
	hash, err := p.Hash("correct horse battery")
	if err != nil || !strings.HasPrefix(hash, "$argon2id$v=19$m=64,t=1,p=1$") {
		t.Fatalf("hash = %q %v", hash, err)
	}
	if !p.Verify("correct horse battery", hash) || p.Verify("wrong", hash) {
		t.Fatal("verify")
	}
	for _, bad := range []string{"", "$bcrypt$x$y$z$w", "$argon2id$v=19$m=x$a$b", "$argon2id$v=19$m=64,t=1,p=1$!!$AAAA", "$argon2id$v=19$m=64,t=1,p=1$AAAA$!!"} {
		if p.Verify("x", bad) {
			t.Errorf("%q verified", bad)
		}
	}
	if DefaultPasswords() != (Passwords{MemoryKiB: 19 * 1024, Time: 2, Threads: 1}) {
		t.Error("defaults")
	}
	if p.Verify("anything", dummyHash) {
		t.Error("nothing signs in against the dummy hash")
	}
}

func TestSetupIsCleaned(t *testing.T) {
	t.Parallel()
	good := domain.Setup{Username: " Aria ", Nickname: " Aria ", Email: " aria@example.com ", Password: "0123456789"}
	got, err := clean(good)
	if err != nil || got.Username != "aria" || got.Nickname != "Aria" || got.Email != "aria@example.com" {
		t.Fatalf("cleaned = %+v %v", got, err)
	}
	for name, change := range map[string]func(*domain.Setup){
		"short username": func(s *domain.Setup) { s.Username = "ab" },
		"empty nickname": func(s *domain.Setup) { s.Nickname = " " },
		"long nickname":  func(s *domain.Setup) { s.Nickname = strings.Repeat("n", 41) },
		"no at":          func(s *domain.Setup) { s.Email = "aria.example.com" },
		"leading at":     func(s *domain.Setup) { s.Email = "@example.com" },
		"trailing at":    func(s *domain.Setup) { s.Email = "aria@" },
		"long email":     func(s *domain.Setup) { s.Email = strings.Repeat("a", 250) + "@x.io" },
		"short password": func(s *domain.Setup) { s.Password = "012345678" },
		"long password":  func(s *domain.Setup) { s.Password = strings.Repeat("p", 201) },
	} {
		s := good
		change(&s)
		if _, err := clean(s); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}
