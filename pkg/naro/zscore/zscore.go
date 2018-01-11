package zscore

import (
	"fmt"

	"github.com/dollarshaveclub/node-auto-repair-operator/pkg/naro"
	"github.com/montanaflynn/stats"
	"github.com/pkg/errors"
)

const (
	// The z-score values for certain percentiles.
	ZScore95 = 1.6449
	ZScore99 = 2.3263
)

// A FeatureExtractor extracts a single float64 feature from a
// naro.NodeTimePeriodSummary.
type FeatureExtractor interface {
	Extract(*naro.NodeTimePeriodSummary) (float64, error)
}

// Detector can run z-tests on naro.NodeTimePeriodSummary
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
	return fmt.Sprintf("zscore.Detector: mean(%f), stddev(%f)", d.mean, d.stddev)
}

// NewDetector creates a new Detector instance.
func NewDetector(zthreshold float64, extractor FeatureExtractor) *Detector {
	return &Detector{
		zthreshold: zthreshold,
		extractor:  extractor,
	}
}

// Train prepares the Detector for testing new
// naro.NodeTimePeriodSummary instances.
func (d *Detector) Train(summaries []*naro.NodeTimePeriodSummary) error {
	var features []float64

	// TODO: process these in a stream.
	for _, ns := range summaries {
		feature, err := d.extractor.Extract(ns)
		if err != nil {
			return errors.Wrapf(err, "error extracting feature from naro.NodeTimePeriodSummary")
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

	return nil
}

// IsAnomaly returns true if the naro.NodeTimePeriodSummary is
// anomalous.
func (d *Detector) IsAnomaly(ns *naro.NodeTimePeriodSummary) (bool, error) {
	feature, err := d.extractor.Extract(ns)
	if err != nil {
		return false, errors.Wrapf(err, "error extracting feature from naro.NodeTimePeriodSummary")
	}

	zscore := (feature - d.mean) / d.stddev

	return zscore >= d.zthreshold, nil
}
