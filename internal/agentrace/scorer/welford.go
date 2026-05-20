package scorer

import "math"

// Welford implements Welford's online algorithm for computing
// running mean and population variance in a single pass.
type Welford struct {
	n    int64
	mean float64
	m2   float64
}

// Update adds a new observation.
func (w *Welford) Update(value float64) {
	w.n++
	delta := value - w.mean
	w.mean += delta / float64(w.n)
	delta2 := value - w.mean
	w.m2 += delta * delta2
}

// Mean returns the running mean. Zero if no observations.
func (w *Welford) Mean() float64 {
	return w.mean
}

// Variance returns the population variance. Zero if fewer than 2 observations.
func (w *Welford) Variance() float64 {
	if w.n < 2 {
		return 0
	}
	return w.m2 / float64(w.n)
}

// StdDev returns the population standard deviation.
func (w *Welford) StdDev() float64 {
	return math.Sqrt(w.Variance())
}

// Count returns the number of observations so far.
func (w *Welford) Count() int64 {
	return w.n
}
