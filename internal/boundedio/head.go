package boundedio

// Head keeps the first bytes written to it, up to a ceiling. It is the
// counterpart of Tail, which keeps the last ones for diagnostics: a document
// is only useful from its beginning -- a WebVTT truncated at the head is no
// subtitle at all -- so what a collector of beginnings owes its caller is the
// prefix it kept and the knowledge that more came after.
//
// Nothing here grows without bound: whatever arrives past the ceiling is
// consumed and dropped, and the caller decides what that means.
type Head struct {
	maximum  int
	data     []byte
	overflow bool
}

// NewHead prepares a collector for the first maximum bytes.
func NewHead(maximum int) *Head { return &Head{maximum: maximum} }

// Write stores what still fits and records that anything further arrived.
func (h *Head) Write(p []byte) (int, error) {
	room := h.maximum - len(h.data)
	if room >= len(p) {
		h.data = append(h.data, p...)
		return len(p), nil
	}
	if room > 0 {
		h.data = append(h.data, p[:room]...)
	}
	h.overflow = true
	return len(p), nil
}

// Bytes answers the prefix that was kept.
func (h *Head) Bytes() []byte { return h.data }

// switches to streaming what remains; keeping a document that outgrew the
// ceiling is not an option Head offers.
func (h *Head) Overflowed() bool { return h.overflow }
