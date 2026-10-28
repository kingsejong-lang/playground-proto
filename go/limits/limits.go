// Package limits holds edge limits that protovalidate rules cannot express and
// that the gateway must apply before a request reaches any service.
//
// Usage (gateway):
//
//	if err := limits.CheckFiles(req.GetFiles()); err != nil {
//	    return nil, connect.NewError(connect.CodeInvalidArgument, err)
//	}
package limits

import (
	"errors"
	"fmt"

	apiv1 "github.com/kingsejong-lang/playground-proto/gen/go/playground/api/v1"
)

const (
	// MaxFileBytes mirrors SourceFile.content max_bytes in common.proto.
	MaxFileBytes = 64 << 10
	// MaxFiles mirrors Files.files max_items.
	MaxFiles = 16
	// MaxTotalSourceBytes is the sum limit that CEL cannot express.
	MaxTotalSourceBytes = 256 << 10
	// MaxStdinBytes mirrors RunRequest.stdin max_len.
	MaxStdinBytes = 64 << 10
	// EdgeReadMaxBytes bounds one decoded message at the edge
	// (connecthttp.WithReadMaxBytes). Larger than the sum of field limits so
	// that field-level validation reports a precise error first.
	EdgeReadMaxBytes = 512 << 10
	// EdgeRequestMaxBytes bounds the raw HTTP body (http.MaxBytesHandler).
	EdgeRequestMaxBytes = 1 << 20
)

// ErrTotalSourceTooLarge is returned when all files together exceed MaxTotalSourceBytes.
var ErrTotalSourceTooLarge = errors.New("total source size exceeds limit")

// CheckFiles enforces the aggregate size limit. Field-level limits are already
// enforced by protovalidate; this function only adds what CEL cannot.
func CheckFiles(files *apiv1.Files) error {
	if files == nil {
		return nil
	}
	total := 0
	for _, f := range files.GetFiles() {
		total += len(f.GetContent())
		if total > MaxTotalSourceBytes {
			return fmt.Errorf("%w: more than %d bytes", ErrTotalSourceTooLarge, MaxTotalSourceBytes)
		}
	}
	return nil
}
