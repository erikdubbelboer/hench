package main

import (
	"time"
)

type Durations []time.Duration

func (d Durations) Len() int {
	return len(d)
}

func (d Durations) Swap(i, j int) {
	d[i], d[j] = d[j], d[i]
}

func (d Durations) Less(i, j int) bool {
	return d[i] < d[j]
}
