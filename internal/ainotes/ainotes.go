package ainotes

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"code.8labs.io/jsuresh/note/analyze"
	"code.8labs.io/jsuresh/note/internal/paths"
)

var slugRE = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// ErrInvalidSlug indicates the note name does not match allowed characters.
var ErrInvalidSlug = errors.New("invalid note slug: use only letters, digits, underscore, and hyphen")

// ErrNoteExists is returned when creating a note that already exists.
var ErrNoteExists = errors.New("note already exists")

// ErrNoteNotFound is returned when a note file is missing.
var ErrNoteNotFound = errors.New("note not found")

// ErrTrashEntryNotFound is returned when a trash basename is missing.
var ErrTrashEntryNotFound = errors.New("trash entry not found")

// AISlugPrefix is required at the start of slugs for all `note ai` create/edit/delete/search/list paths.
const AISlugPrefix = "ai-"

// ErrNotAISlug is returned when a slug does not start with AISlugPrefix.
var ErrNotAISlug = errors.New(`slug must start with "ai-" (e.g. ai-preferences, ai-project-foo)`)

// Match is one line in a file where all search terms appear.
type Match struct {
	Path   string
	Line   int
	Column int
	Text   string
}

// ListNotePaths returns absolute paths of top-level note .txt files (excluding trash and dotfiles).
func ListNotePaths(notesDir string) ([]string, error) {
	entries, err := os.ReadDir(notesDir)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, ".") || !strings.HasSuffix(name, ".txt") {
			continue
		}
		out = append(out, filepath.Join(notesDir, name))
	}
	sort.Strings(out)
	return out, nil
}

// ListAINotePaths returns absolute paths of AI notes: top-level `ai-*.txt` files only.
func ListAINotePaths(notesDir string) ([]string, error) {
	all, err := ListNotePaths(notesDir)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, p := range all {
		slug := strings.TrimSuffix(filepath.Base(p), ".txt")
		if err := ValidateAISlug(slug); err != nil {
			continue
		}
		out = append(out, p)
	}
	return out, nil
}

// ListSlugs returns sorted slugs for top-level notes.
func ListSlugs(notesDir string) ([]string, error) {
	pathsList, err := ListNotePaths(notesDir)
	if err != nil {
		return nil, err
	}
	slugs := make([]string, 0, len(pathsList))
	for _, p := range pathsList {
		base := filepath.Base(p)
		slugs = append(slugs, strings.TrimSuffix(base, ".txt"))
	}
	return slugs, nil
}

// ValidateSlug returns an error if slug is not safe for note filenames.
func ValidateSlug(slug string) error {
	if !slugRE.MatchString(slug) {
		return fmt.Errorf("%w: %q", ErrInvalidSlug, slug)
	}
	return nil
}

// ValidateAISlug enforces slug rules plus the required `ai-` prefix for agent-managed notes.
func ValidateAISlug(slug string) error {
	if err := ValidateSlug(slug); err != nil {
		return err
	}
	if !strings.HasPrefix(slug, AISlugPrefix) || len(slug) <= len(AISlugPrefix) {
		return fmt.Errorf("%w: %q", ErrNotAISlug, slug)
	}
	return nil
}

// CreateNote creates an empty note or copies from fromPath when non-empty.
func CreateNote(notesDir, slug, fromPath string) error {
	if err := ValidateAISlug(slug); err != nil {
		return err
	}
	dst := paths.NoteFile(notesDir, slug)
	if _, err := os.Stat(dst); err == nil {
		return ErrNoteExists
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(notesDir, 0755); err != nil {
		return err
	}
	if fromPath != "" {
		src, err := os.Open(fromPath)
		if err != nil {
			return err
		}
		defer src.Close()
		out, err := os.Create(dst)
		if err != nil {
			return err
		}
		defer out.Close()
		if _, err := io.Copy(out, src); err != nil {
			return err
		}
	} else {
		f, err := os.Create(dst)
		if err != nil {
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
	}
	analyze.AnalyzeFile(slug)
	return nil
}

// NotePath returns the absolute path to slug.txt.
func NotePath(notesDir, slug string) (string, error) {
	if err := ValidateAISlug(slug); err != nil {
		return "", err
	}
	return filepath.Abs(paths.NoteFile(notesDir, slug))
}

// ReplaceNote copies replacementPath over the note for slug and re-indexes.
func ReplaceNote(notesDir, slug, replacementPath string) error {
	if err := ValidateAISlug(slug); err != nil {
		return err
	}
	dst := paths.NoteFile(notesDir, slug)
	if _, err := os.Stat(dst); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ErrNoteNotFound
		}
		return err
	}
	src, err := os.Open(replacementPath)
	if err != nil {
		return err
	}
	defer src.Close()
	tmp := dst + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, src); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, dst); err != nil {
		os.Remove(tmp)
		return err
	}
	analyze.AnalyzeFile(slug)
	return nil
}

// DeleteNote moves the note into trash and removes search index rows.
func DeleteNote(notesDir, slug string) error {
	if err := ValidateAISlug(slug); err != nil {
		return err
	}
	src := paths.NoteFile(notesDir, slug)
	if _, err := os.Stat(src); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ErrNoteNotFound
		}
		return err
	}
	trashDir := paths.TrashDir(notesDir)
	if err := os.MkdirAll(trashDir, 0755); err != nil {
		return err
	}
	dst, err := uniqueTrashPath(trashDir, slug)
	if err != nil {
		return err
	}
	if err := os.Rename(src, dst); err != nil {
		return err
	}
	if err := analyze.DeleteTokensForDocument(slug); err != nil {
		return err
	}
	return nil
}

func uniqueTrashPath(trashDir, slug string) (string, error) {
	base := slug + ".txt"
	dst := filepath.Join(trashDir, base)
	if _, err := os.Stat(dst); errors.Is(err, os.ErrNotExist) {
		return dst, nil
	} else if err != nil {
		return "", err
	}
	for n := 2; ; n++ {
		candidate := filepath.Join(trashDir, fmt.Sprintf("%s.%d.txt", slug, n))
		if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate, nil
		} else if err != nil {
			return "", err
		}
	}
}

// ListTrash returns sorted basenames in the trash folder.
func ListTrash(notesDir string) ([]string, error) {
	trashDir := paths.TrashDir(notesDir)
	entries, err := os.ReadDir(trashDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names, nil
}

// RestoreTrash moves a trash basename back into notesDir as slug.txt and returns the restored file path.
func RestoreTrash(notesDir, basename string) (string, error) {
	trashDir := paths.TrashDir(notesDir)
	src := filepath.Join(trashDir, basename)
	if _, err := os.Stat(src); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", ErrTrashEntryNotFound
		}
		return "", err
	}
	slug, err := slugFromTrashName(basename)
	if err != nil {
		return "", err
	}
	if err := ValidateAISlug(slug); err != nil {
		return "", err
	}
	dst := paths.NoteFile(notesDir, slug)
	if _, err := os.Stat(dst); err == nil {
		return "", fmt.Errorf("restore blocked: note %q already exists", slug)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if err := os.Rename(src, dst); err != nil {
		return "", err
	}
	analyze.AnalyzeFile(slug)
	abs, err := filepath.Abs(dst)
	if err != nil {
		return dst, nil
	}
	return abs, nil
}

func slugFromTrashName(basename string) (string, error) {
	if !strings.HasSuffix(basename, ".txt") {
		return "", fmt.Errorf("invalid trash entry: %q", basename)
	}
	without := strings.TrimSuffix(basename, ".txt")
	// Prefer "slug.N.txt" pattern (N numeric) from collision naming.
	if i := strings.LastIndex(without, "."); i > 0 {
		suffix := without[i+1:]
		if _, err := strconv.Atoi(suffix); err == nil {
			return without[:i], nil
		}
	}
	return without, nil
}

// PurgeTrash removes a file from trash permanently.
func PurgeTrash(notesDir, basename string) error {
	p := filepath.Join(paths.TrashDir(notesDir), basename)
	if _, err := os.Stat(p); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ErrTrashEntryNotFound
		}
		return err
	}
	return os.Remove(p)
}

// SearchTerms finds lines in each note where every term appears as a substring (case-insensitive).
func SearchTerms(notesDir string, terms []string) ([]Match, error) {
	if len(terms) == 0 {
		return nil, errors.New("at least one search term is required")
	}
	lowerTerms := make([]string, len(terms))
	for i, t := range terms {
		if t == "" {
			return nil, errors.New("empty search term")
		}
		lowerTerms[i] = strings.ToLower(t)
	}
	pathsList, err := ListAINotePaths(notesDir)
	if err != nil {
		return nil, err
	}
	var matches []Match
	for _, p := range pathsList {
		f, err := os.Open(p)
		if err != nil {
			return nil, err
		}
		sc := bufio.NewScanner(f)
		lineNum := 0
		for sc.Scan() {
			lineNum++
			line := sc.Text()
			lower := strings.ToLower(line)
			if !containsAll(lower, lowerTerms) {
				continue
			}
			col := firstMatchColumn(lower, line, lowerTerms)
			matches = append(matches, Match{
				Path:   p,
				Line:   lineNum,
				Column: col,
				Text:   line,
			})
		}
		if err := sc.Err(); err != nil {
			f.Close()
			return nil, err
		}
		f.Close()
	}
	return matches, nil
}

func containsAll(hay string, needles []string) bool {
	for _, n := range needles {
		if !strings.Contains(hay, n) {
			return false
		}
	}
	return true
}

func firstMatchColumn(lower, original string, lowerTerms []string) int {
	minIdx := len(lower)
	for _, t := range lowerTerms {
		idx := strings.Index(lower, t)
		if idx >= 0 && idx < minIdx {
			minIdx = idx
		}
	}
	if minIdx >= len(lower) {
		return 1
	}
	return utf8.RuneCountInString(original[:minIdx]) + 1
}
