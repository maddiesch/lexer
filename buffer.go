package lexer

// Buffer is responsible for collecting the runes as they are read from the
// source input.
type Buffer interface {
	// Append a rune to the buffer
	Write(Location, rune)

	// Returns all of the buffer contents
	Runes() []rune

	// Returns the location when the buffer was first written to
	StartLocation() Location

	// Should reset the buffer's internal state to empty
	Reset()

	// Should delete the last x number of items from the buffer
	//
	// It should guard against an over-delete
	Delete(int)
}

// BufferProvider is called every time the lexer needs a new buffer.
type BufferProvider func() Buffer

// NewBuffer is the default implementation of a BufferProvider and is the
// default BufferProvider for all new lexers.
func NewBuffer() Buffer {
	return &dBuffer{}
}

// dBuffer is the default implementation of Buffer
type dBuffer struct {
	runes   []rune
	started Location
}

const dBufferSliceChunkSize = 64

func (d *dBuffer) Write(l Location, r rune) {
	if len(d.runes) == 0 {
		d.started = l
		d.runes = make([]rune, 0, dBufferSliceChunkSize)
	}

	runes := d.runes

	if len(runes)+1 > cap(runes) {
		new := make([]rune, len(runes), cap(runes)+dBufferSliceChunkSize)

		copy(new, runes)

		runes = new
	}

	d.runes = append(runes, r)
}

func (d *dBuffer) Runes() []rune {
	return d.runes
}

func (d *dBuffer) StartLocation() Location {
	return d.started
}

func (d *dBuffer) Reset() {
	d.runes = nil
	d.started = mLoc(LocationValueInvalid, LocationValueInvalid)
}

func (d *dBuffer) Delete(i int) {
	if i > len(d.runes) {
		return
	}

	d.runes = d.runes[:len(d.runes)-i]
}
