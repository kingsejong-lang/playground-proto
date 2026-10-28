package limits

import (
	"errors"
	"strings"
	"testing"

	apiv1 "github.com/kingsejong-lang/playground-proto/gen/go/playground/api/v1"
)

func TestCheckFiles(t *testing.T) {
	chunk := strings.Repeat("a", MaxFileBytes)
	mk := func(n int) *apiv1.Files {
		fs := &apiv1.Files{}
		for i := 0; i < n; i++ {
			fs.Files = append(fs.Files, &apiv1.SourceFile{Path: "f" + string(rune('a'+i)), Content: chunk})
		}
		return fs
	}
	if err := CheckFiles(mk(4)); err != nil { // exactly 256 KiB
		t.Fatalf("4 x 64KiB should pass: %v", err)
	}
	if err := CheckFiles(mk(5)); !errors.Is(err, ErrTotalSourceTooLarge) {
		t.Fatalf("5 x 64KiB should fail with ErrTotalSourceTooLarge, got %v", err)
	}
	if err := CheckFiles(nil); err != nil {
		t.Fatalf("nil should pass: %v", err)
	}
}
