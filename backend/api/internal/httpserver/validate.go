package httpserver

import (
	"encoding/json"
	"io"

	"github.com/google/uuid"
)

// parseID parses a request's id path parameter. It is uuid.Parse's contract,
// named and pinned here so the request-level rules are checkable without a
// request: the canonical dashed form is accepted case-insensitively, as are
// the braced, urn:uuid: and undashed-32-hex spellings, while a value padded
// with whitespace is rejected rather than trimmed.
func parseID(raw string) (uuid.UUID, error) {
	return uuid.Parse(raw)
}

// decodeJSON decodes the first JSON value in r into dst. Unknown fields are
// ignored (DisallowUnknownFields is deliberately not set) and bytes after
// that first value are left unread, so trailing input never fails a request.
func decodeJSON(r io.Reader, dst any) error {
	return json.NewDecoder(r).Decode(dst)
}
