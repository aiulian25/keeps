package attachments

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestCleanFilenameStaysBoundedForEveryExtensionLength(t *testing.T) {
	extensionLengths := []int{FilenameMaximumRunes - 1, FilenameMaximumRunes, FilenameMaximumRunes + 1, 2 * FilenameMaximumRunes}
	for _, extensionRunes := range extensionLengths {
		for _, letter := range []string{"a", "é", "照"} {
			// The extension counts its dot, as filepath.Ext returns it.
			name := "photo." + strings.Repeat(letter, extensionRunes-1)
			cleaned := CleanFilename(name)
			if cleaned == "" || !utf8.ValidString(cleaned) || utf8.RuneCountInString(cleaned) > FilenameMaximumRunes {
				t.Errorf("a %d-rune extension of %q gave %d runes: %q", extensionRunes, letter, utf8.RuneCountInString(cleaned), cleaned)
			}
		}
	}

	if cleaned := CleanFilename("p." + strings.Repeat("a", FilenameMaximumRunes-2) + "x"); utf8.RuneCountInString(cleaned) != FilenameMaximumRunes {
		t.Fatalf("a name just over the limit was not cut to it: %d runes", utf8.RuneCountInString(cleaned))
	}
	long := strings.Repeat("照", 2*FilenameMaximumRunes) + ".pdf"
	if cleaned := CleanFilename(long); !strings.HasSuffix(cleaned, ".pdf") || utf8.RuneCountInString(cleaned) != FilenameMaximumRunes {
		t.Fatalf("a long name lost its extension: %q", cleaned)
	}
	for _, ordinary := range []string{"receipt.pdf", "Invoice – Sage.pdf", "photo.JPG", "no extension"} {
		if cleaned := CleanFilename(ordinary); cleaned != ordinary {
			t.Errorf("%q became %q", ordinary, cleaned)
		}
	}
}
