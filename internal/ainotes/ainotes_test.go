package ainotes

import (
	"os"
	"path/filepath"
	"testing"

	"code.8labs.io/jsuresh/note/internal/paths"
)

func TestSlugFromTrashName(t *testing.T) {
	s, err := slugFromTrashName("ai-topic.2.txt")
	if err != nil || s != "ai-topic" {
		t.Fatalf("got %q %v want ai-topic", s, err)
	}
	s, err = slugFromTrashName("ai-topic.txt")
	if err != nil || s != "ai-topic" {
		t.Fatalf("got %q %v want ai-topic", s, err)
	}
}

func TestUniqueTrashPath(t *testing.T) {
	dir := t.TempDir()
	trash := filepath.Join(dir, "trash")
	if err := os.MkdirAll(trash, 0755); err != nil {
		t.Fatal(err)
	}
	p1, err := uniqueTrashPath(trash, "ai-a")
	if err != nil || filepath.Base(p1) != "ai-a.txt" {
		t.Fatalf("first %q %v", p1, err)
	}
	if err := os.WriteFile(p1, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	p2, err := uniqueTrashPath(trash, "ai-a")
	if err != nil || filepath.Base(p2) != "ai-a.2.txt" {
		t.Fatalf("second %q %v", p2, err)
	}
}

func TestSearchTerms(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ai-one.txt"), []byte("alpha beta\nno match\n"), 0644); err != nil {
		t.Fatal(err)
	}
	ms, err := SearchTerms(dir, []string{"alpha", "beta"})
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 1 || ms[0].Line != 1 {
		t.Fatalf("matches: %+v", ms)
	}
}

func TestValidateSlug(t *testing.T) {
	if ValidateSlug("bad slug") == nil {
		t.Fatal("expected error")
	}
	if ValidateSlug("ok-slug_9") != nil {
		t.Fatal("unexpected error")
	}
}

func TestValidateAISlug(t *testing.T) {
	if ValidateAISlug("preferences") == nil {
		t.Fatal("expected error without ai- prefix")
	}
	if ValidateAISlug("ai-") == nil {
		t.Fatal("expected error for bare ai-")
	}
	if ValidateAISlug("ai-prefs") != nil {
		t.Fatalf("unexpected: %v", ValidateAISlug("ai-prefs"))
	}
}

func TestTrashDirConstant(t *testing.T) {
	d := paths.TrashDir("/tmp/notes")
	if filepath.Base(d) != paths.TrashDirName {
		t.Fatalf("unexpected trash dir %q", d)
	}
}
