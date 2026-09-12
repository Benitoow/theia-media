// Package boundedio provides subprocess output collectors whose memory use is
// fixed even when a broken tool writes forever.
package boundedio

type Tail struct {
	maximum int
	data    []byte
}

func NewTail(maximum int) *Tail { return &Tail{maximum: maximum} }

func (b *Tail) Write(p []byte) (int, error) {
	written := len(p)
	if b.maximum <= 0 {
		return written, nil
	}
	if len(p) >= b.maximum {
		b.data = append(b.data[:0], p[len(p)-b.maximum:]...)
		return written, nil
	}
	overflow := len(b.data) + len(p) - b.maximum
	if overflow > 0 {
		copy(b.data, b.data[overflow:])
		b.data = b.data[:len(b.data)-overflow]
	}
	b.data = append(b.data, p...)
	return written, nil
}

func (b *Tail) Bytes() []byte  { return b.data }
func (b *Tail) String() string { return string(b.data) }
