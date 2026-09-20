package api

import (
	"net/http"
	"strconv"
)

// positivePathID reads a numeric path segment and refuses anything that is not a
// number greater than zero, with the route's own error code.
//
// One function for both route families. The same URL shape used to be validated
// by two names with one rule, which is how a reader comes to believe there are
// two rules; a zero or a negative id is a client mistake, not a record to look
// up.
func positivePathID(w http.ResponseWriter, r *http.Request, name, code string) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil || id <= 0 {
		writeJSONError(w, http.StatusBadRequest, code)
		return 0, false
	}
	return id, true
}
