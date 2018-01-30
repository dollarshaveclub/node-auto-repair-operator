/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package util

// FloatSlidingWindow is a buffer with a fixed capacity. Elements are
// inserted/removed in the FIFO order. Elements are removed from the buffer
// only when it runs out of capacity and a new element is inserted.
type FloatSlidingWindow interface {
	// Add a value to the end of the queue. On overflow returns true and the
	// oldest value, which is also removed from the buffer. Otherwise
	// returns (false, _).
	Push(value float64) (bool, float64)

	// Returns the elements in the buffer, ordered by time of insertion
	// (oldest first).
	Contents() []float64

	// Returns a pointer to the most recently added element. The pointer can
	// be used to modify the last element. It is only valid until the next
	// call to Push(). Return nil if called on an empty buffer.
	Head() *float64

	// Reset the contents of the window.
	Clear()
}

// NewFloatSlidingWindow returns a new instance of FloatSlidingWindowImpl with a
// given size.
func NewFloatSlidingWindow(size int) FloatSlidingWindow {
	if size < 1 {
		panic("Buffer size must be at least 1")
	}
	return &floatSlidingWindow{make([]float64, 0), -1, size, false}
}

type floatSlidingWindow struct {
	buffer []float64
	// Index of the most recently added element.
	head int
	// Max number of elements.
	capacity int
	// Whether the buffer is full, i.e. the number of elements in the buffer
	// equals capacity.
	isFull bool
}

// Head returns a pointer to the most recently added element. The pointer can be
// used to modify the last element. It is only valid until the next call to
// Push(). Returns nil if called on an empty buffer.
func (b *floatSlidingWindow) Head() *float64 {
	if b.head == -1 {
		return nil
	}
	return &b.buffer[b.head]
}

// Contents returns the elements in the buffer, ordered by time of insertion
// (oldest first).
func (b *floatSlidingWindow) Contents() []float64 {
	return append(b.buffer[b.head+1:], b.buffer[:b.head+1]...)
}

// Push adds a value to the end of the window. On overflow returns true and the
// oldest value, which is also removed from the window. Otherwise returns
// (false, _).
func (b *floatSlidingWindow) Push(value float64) (bool, float64) {
	b.head++
	if b.head == b.capacity {
		b.head = 0
		b.isFull = true
	}
	if !b.isFull {
		b.buffer = append(b.buffer, value)
		return false, 0.0
	}
	// Buffer is full. Rewrite the oldest entry and return it.
	prevValue := b.buffer[b.head]
	b.buffer[b.head] = value
	return true, prevValue
}

// Clear resets the contents of the window.
func (b *floatSlidingWindow) Clear() {
	b.buffer = make([]float64, 0)
	b.head = -1
	b.isFull = false
}
