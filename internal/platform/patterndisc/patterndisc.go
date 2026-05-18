package patterndisc

import "sort"

// Pattern is a detected tool-use sequence
type Pattern struct {
	ID         string
	Sequence   []string
	Occurrences int
	Confidence float64
	IsAntiPattern bool
}

// Discoverer detects frequent patterns from tool-use sequences
type Discoverer struct {
	counts map[string]int
	total  int
}

// NewDiscoverer creates an empty Discoverer
func NewDiscoverer() *Discoverer {
	return &Discoverer{counts: map[string]int{}}
}

// Record registers one tool-use sequence (as an ordered list of tool names)
func (d *Discoverer) Record(sequence []string) {
	if len(sequence) == 0 {
		return
	}
	key := joinSequence(sequence)
	d.counts[key]++
	d.total++
}

// Discover returns patterns that appear at least minOccurrences times,
// sorted by occurrences descending. Confidence = occurrences / total.
func (d *Discoverer) Discover(minOccurrences int) []Pattern {
	var patterns []Pattern
	for key, count := range d.counts {
		if count >= minOccurrences {
			conf := 0.0
			if d.total > 0 {
				conf = float64(count) / float64(d.total)
			}
			patterns = append(patterns, Pattern{
				ID:          key,
				Sequence:    splitSequence(key),
				Occurrences: count,
				Confidence:  conf,
			})
		}
	}
	sort.Slice(patterns, func(i, j int) bool {
		return patterns[i].Occurrences > patterns[j].Occurrences
	})
	return patterns
}

// MarkAntiPattern flags a pattern by ID as an anti-pattern
func MarkAntiPattern(patterns []Pattern, id string) []Pattern {
	for i := range patterns {
		if patterns[i].ID == id {
			patterns[i].IsAntiPattern = true
		}
	}
	return patterns
}

func joinSequence(seq []string) string {
	result := ""
	for i, s := range seq {
		if i > 0 {
			result += "->"
		}
		result += s
	}
	return result
}

func splitSequence(key string) []string {
	if key == "" {
		return nil
	}
	var parts []string
	cur := ""
	for i := 0; i < len(key); i++ {
		if i+1 < len(key) && key[i] == '-' && key[i+1] == '>' {
			parts = append(parts, cur)
			cur = ""
			i++
		} else {
			cur += string(key[i])
		}
	}
	parts = append(parts, cur)
	return parts
}
