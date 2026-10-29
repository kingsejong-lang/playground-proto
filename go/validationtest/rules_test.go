// Package validationtest exercises the protovalidate rules declared in the
// .proto files, so a rule change that weakens the edge is caught in CI.
package validationtest

import (
	"strings"
	"testing"

	"buf.build/go/protovalidate"
	"google.golang.org/protobuf/proto"

	apiv1 "github.com/kingsejong-lang/playground-proto/gen/go/playground/api/v1"
)

func files(paths ...string) *apiv1.Files {
	fs := &apiv1.Files{}
	for _, p := range paths {
		fs.Files = append(fs.Files, &apiv1.SourceFile{Path: p, Content: "출력(\"안녕\")"})
	}
	return fs
}

func TestRunRequestRules(t *testing.T) {
	v, err := protovalidate.New()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name  string
		msg   proto.Message
		valid bool
	}{
		{"ok", &apiv1.RunRequest{LanguageId: "kingsejong", Files: files("main.ksj", "lib/util.ksj")}, true},
		{"missing files", &apiv1.RunRequest{LanguageId: "kingsejong"}, false},
		{"bad language id", &apiv1.RunRequest{LanguageId: "King Sejong;rm -rf /", Files: files("main.ksj")}, false},
		{"path traversal", &apiv1.RunRequest{LanguageId: "kingsejong", Files: files("../etc/passwd")}, false},
		{"dot segment", &apiv1.RunRequest{LanguageId: "kingsejong", Files: files("a/../b.ksj")}, false},
		{"absolute path", &apiv1.RunRequest{LanguageId: "kingsejong", Files: files("/etc/passwd")}, false},
		{"duplicate paths", &apiv1.RunRequest{LanguageId: "kingsejong", Files: files("main.ksj", "main.ksj")}, false},
		{"too many files", &apiv1.RunRequest{LanguageId: "kingsejong", Files: files("a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o", "p", "q")}, false},
		{"entry not in files", &apiv1.RunRequest{LanguageId: "kingsejong", Files: files("main.ksj"), EntryFile: "other.ksj"}, false},
		{"stdin too big", &apiv1.RunRequest{LanguageId: "kingsejong", Files: files("main.ksj"), Stdin: make([]byte, 65537)}, false},
		{"too many args", &apiv1.RunRequest{LanguageId: "kingsejong", Files: files("main.ksj"), Args: make([]string, 17)}, false},
		{"wall time too high", &apiv1.RunRequest{LanguageId: "kingsejong", Files: files("main.ksj"), WallTimeLimitMs: 60001}, false},
		{"file too big", &apiv1.RunRequest{LanguageId: "kingsejong", Files: &apiv1.Files{Files: []*apiv1.SourceFile{{Path: "main.ksj", Content: strings.Repeat("a", 65537)}}}}, false},
		{"bad execution id", &apiv1.CancelRequest{ExecutionId: "short"}, false},
		{"resume ok", &apiv1.ResumeRunRequest{ExecutionId: "01HZX3K9Q2V7M5T8R4W6Y1B0CD", AfterSeq: 42}, true},
		{"resume bad id", &apiv1.ResumeRunRequest{ExecutionId: "../../x"}, false},
		{"share title too long", &apiv1.CreateShareRequest{LanguageId: "kingsejong", Files: files("main.ksj"), Title: strings.Repeat("가", 121)}, false},
		{"settings font too big", &apiv1.UpdateSettingsRequest{Settings: &apiv1.Settings{FontSize: 99, TabSize: 4}}, false},
		{"bad email", &apiv1.RegisterRequest{Email: "not-an-email", Password: "0123456789", DisplayName: "a"}, false},
		{"short password", &apiv1.RegisterRequest{Email: "a@example.com", Password: "short", DisplayName: "a"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := v.Validate(tc.msg)
			if tc.valid && err != nil {
				t.Fatalf("expected valid, got %v", err)
			}
			if !tc.valid && err == nil {
				t.Fatalf("expected a validation error")
			}
		})
	}
}
