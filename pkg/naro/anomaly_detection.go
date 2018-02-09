package naro

import (
	"context"
	"sync"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// AnomalyDetector is a type that can be trained to detect issues
// within new NodeTimePeriodSummaries.
type AnomalyDetector interface {
	String() string
	Train(summaries []*NodeTimePeriodSummary) error
	IsAnomalous(ns *NodeTimePeriodSummary) (bool, string, error)
}

// AnomalyDetectorFactory is a function that can create new
// AnomalyDetectors.
type AnomalyDetectorFactory func() (AnomalyDetector, error)

// AnomalyHandler is a type that can respond to anomalies. This is
// where node repairs are wired in.
type AnomalyHandler interface {
	HandleAnomaly(context.Context, *NodeTimePeriodSummary, string) error
}

// DetectorController runs a set of AnomalyDetectors periodically,
// informing handlers of when an anomaly is detected.
type DetectorController struct {
	stopChan            chan struct{}
	wg                  *sync.WaitGroup
	factories           []AnomalyDetectorFactory
	trainingTimePeriod  time.Duration
	detectionTimePeriod time.Duration
	runInterval         time.Duration
	store               Store
	clock               clockwork.Clock
	handlers            []AnomalyHandler
}

// NewDetectorController returns a new DetectorController.
func NewDetectorController(trainingTimePeriod, detectionTimePeriod,
	runInterval time.Duration, factories []AnomalyDetectorFactory,
	store Store, clock clockwork.Clock, handlers []AnomalyHandler) *DetectorController {
	return &DetectorController{
		stopChan:            make(chan struct{}),
		wg:                  &sync.WaitGroup{},
		factories:           factories,
		trainingTimePeriod:  trainingTimePeriod,
		detectionTimePeriod: detectionTimePeriod,
		runInterval:         runInterval,
		store:               store,
		clock:               clock,
		handlers:            handlers,
	}
}

func (d *DetectorController) getDetectors() ([]AnomalyDetector, error) {
	var detectors []AnomalyDetector
	for _, factory := range d.factories {
		detector, err := factory()
		if err != nil {
			return nil, errors.Wrapf(err, "error creating AnomalyDetector with factory")
		}
		detectors = append(detectors, detector)
	}

	currentTime := d.clock.Now()
	trainingStart := currentTime.Add(-d.trainingTimePeriod)

	summaries, err := d.store.GetNodeTimePeriodSummaries(trainingStart, currentTime)
	if err != nil {
		return nil, errors.Wrapf(err, "error fetching NodeTimePeriodSummaries for training")
	}

	for _, detector := range detectors {
		logrus.Debugf("DetectorController: training %s with %d summaries", detector, len(summaries))
		if err := detector.Train(summaries); err != nil {
			return nil, errors.Wrapf(err, "error training detector")
		}
	}

	return detectors, nil
}

// Start begins starts the controller's run loop.
func (d *DetectorController) Start() {
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		for {
			select {
			case <-d.stopChan:
				return
			case <-d.clock.After(d.runInterval):
			}

			logrus.Debugf("DetectorController: starting run loop")
			if err := d.run(); err != nil {
				logrus.WithError(err).Errorf("error running DetectorController")
			}
		}
	}()
}

// Stop stops the detector's run loop.
func (d *DetectorController) Stop() {
	close(d.stopChan)
	d.wg.Wait()
}

// run performs anomaly detection, informing handlers of any
// anomalies. The set of detectors is trained prior to running any
// detection algorithms.
func (d *DetectorController) run() error {
	detectors, err := d.getDetectors()
	if err != nil {
		return errors.Wrapf(err, "error creating detectors")
	}

	currentTime := d.clock.Now()
	detectionStart := currentTime.Add(-d.detectionTimePeriod)

	summaries, err := d.store.GetNodeTimePeriodSummaries(detectionStart, currentTime)
	if err != nil {
		return errors.Wrapf(err, "error fetching NodeTimePeriodSummaries for detection")
	}

	for _, detector := range detectors {
		for _, nodeSummary := range summaries {
			logrus.Debugf("DetectorController: attempting to detect anomaly in %s with %s", nodeSummary.Node, detector)

			nodeSummary.RemoveOlderRepairedEvents()

			isAnomalous, meta, err := detector.IsAnomalous(nodeSummary)
			if err != nil {
				logrus.WithError(err).Errorf("error detecting anomaly in NodeTimePeriodSummary")
				continue
			}
			if isAnomalous {
				logrus.Infof("DetectorController: detected anomaly in %s with %s: %s", nodeSummary.Node, detector, meta)

				for _, handler := range d.handlers {
					if err := handler.HandleAnomaly(context.Background(), nodeSummary, meta); err != nil {
						logrus.WithError(err).
							Errorf("error handling anomalous NodeTimePeriodSummary")
						continue
					}
				}
			}
		}
	}

	return nil
}
