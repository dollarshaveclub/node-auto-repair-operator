package zscore

import (
	"fmt"

	"github.com/dollarshaveclub/node-auto-repair-operator/pkg/nodes"
	"github.com/montanaflynn/stats"
	"github.com/pkg/errors"
)

const (
	// The z-score values for certain percentiles.
	ZScore95 = 1.6449
	ZScore99 = 2.3263
)

// A FeatureExtractor extracts a single float64 feature from a
// nodes.NodeTimePeriodSummary.
type FeatureExtractor interface {
	Extract(*nodes.NodeTimePeriodSummary) (float64, error)
}

// Detector can run z-tests on nodes.NodeTimePeriodSummary
// instances. More information:
// http://colingorrie.github.io/outlier-detection.html
type Detector struct {
	mean       float64
	stddev     float64
	zthreshold float64
	extractor  FeatureExtractor
}

// String returns the string representation of a Detector.
func (d *Detector) String() string {
	return fmt.Sprintf("Detector: mean(%f), stddev(%f)", d.mean, d.stddev)
}

// NewDetector creates a new Detector instance.
func NewDetector(zthreshold float64, extractor FeatureExtractor) *Detector {
	return &Detector{
		zthreshold: zthreshold,
		extractor:  extractor,
	}
}

// Train prepares the Detector for testing new
// nodes.NodeTimePeriodSummary instances.
func (d *Detector) Train(summaries <-chan *nodes.NodeTimePeriodSummary, done chan<- struct{}) error {
	var features []float64

	// TODO: process these in a stream, taking advantage of the
	// input channel. This involves calculating means & stddevs
	// manually.
	for ns := range summaries {
		feature, err := d.extractor.Extract(ns)
		if err != nil {
			return errors.Wrapf(err, "error extracting feature from nodes.NodeTimePeriodSummary")
		}
		features = append(features, feature)
	}

	mean, err := stats.Mean(features)
	if err != nil {
		return errors.Wrapf(err, "error calculating mean")
	}
	stddev, err := stats.StandardDeviation(features)
	if err != nil {
		return errors.Wrapf(err, "error calculating standard deviation")
	}

	d.mean = mean
	d.stddev = stddev

	done <- struct{}{}

	return nil
}

// IsAnomaly returns true if the nodes.NodeTimePeriodSummary is
// anomalous.
func (d *Detector) IsAnomaly(ns *nodes.NodeTimePeriodSummary) (bool, error) {
	feature, err := d.extractor.Extract(ns)
	if err != nil {
		return false, errors.Wrapf(err, "error extracting feature from nodes.NodeTimePeriodSummary")
	}

	zscore := (feature - d.mean) / d.stddev

	return zscore >= d.zthreshold, nil
}
