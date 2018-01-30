package naro_test

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/dollarshaveclub/node-auto-repair-operator/pkg/naro"
	"github.com/dollarshaveclub/node-auto-repair-operator/pkg/naro/boltdb"
	"github.com/dollarshaveclub/node-auto-repair-operator/pkg/naro/testutil"
	"github.com/dollarshaveclub/node-auto-repair-operator/pkg/naro/testutil/mocks"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestDetectorControllerRun(t *testing.T) {
	db, cleaner := testutil.DB(t)
	defer cleaner()

	store, err := boltdb.NewStore(db)
	assert.NoError(t, err)

	trainingTimePeriod := 100 * time.Second
	detectionTimePeriod := 4 * time.Second
	runInterval := 3 * time.Second

	t.Run("detect-anomaly", func(t *testing.T) {
		clock := clockwork.NewFakeClock()
		currentTime := clock.Now()

		node := naro.NewNodeFromKubeNode(testutil.FakeKubeNode(t))
		event := testutil.FakeKubeNodeEvent(t)
		event.LastTimestamp = metav1.NewTime(currentTime.Add(-time.Second))
		e := naro.NewNodeEventFromKubeEvent(node, event)
		assert.NoError(t, store.CreateNode(node))
		assert.NoError(t, store.CreateNodeEvent(e))

		detector := &mocks.AnomalyDetector{}
		detector.On("String").Return("AnomalyDetector")
		detector.On("Train", mock.MatchedBy(func(summaries []*naro.NodeTimePeriodSummary) bool {
			summary := summaries[0]
			return summary.Events[0].ID == e.ID && summary.Node.ID == node.ID
		})).Return(nil)
		detector.On("IsAnomalous", mock.MatchedBy(func(summary *naro.NodeTimePeriodSummary) bool {
			return summary.Events[0].ID == e.ID && summary.Node.ID == node.ID
		})).Return(true, "metadata", nil)
		defer detector.AssertExpectations(t)

		factory := func() (naro.AnomalyDetector, error) {
			return detector, nil
		}

		// This is the important event.
		handler := &mocks.AnomalyHandler{}
		handler.On("HandleAnomaly", mock.MatchedBy(func(summary *naro.NodeTimePeriodSummary) bool {
			return summary.Events[0].ID == e.ID && summary.Node.ID == node.ID
		}), "metadata").Return(nil)
		defer handler.AssertExpectations(t)

		controller := naro.NewDetectorController(trainingTimePeriod, detectionTimePeriod,
			runInterval, []naro.AnomalyDetectorFactory{factory}, store, clock,
			[]naro.AnomalyHandler{handler},
		)

		controller.Start()
		clock.BlockUntil(1)
		clock.Advance(runInterval)
		controller.Stop()
	})

	t.Run("no-anomaly", func(t *testing.T) {
		clock := clockwork.NewFakeClock()
		currentTime := clock.Now()

		node := naro.NewNodeFromKubeNode(testutil.FakeKubeNode(t))
		event := testutil.FakeKubeNodeEvent(t)
		event.LastTimestamp = metav1.NewTime(currentTime.Add(-time.Second))
		e := naro.NewNodeEventFromKubeEvent(node, event)
		assert.NoError(t, store.CreateNode(node))
		assert.NoError(t, store.CreateNodeEvent(e))

		detector := &mocks.AnomalyDetector{}
		detector.On("String").Return("AnomalyDetector")
		detector.On("Train", mock.MatchedBy(func(summaries []*naro.NodeTimePeriodSummary) bool {
			summary := summaries[0]
			return summary.Events[0].ID == e.ID && summary.Node.ID == node.ID
		})).Return(nil)
		detector.On("IsAnomalous", mock.MatchedBy(func(summary *naro.NodeTimePeriodSummary) bool {
			return summary.Events[0].ID == e.ID && summary.Node.ID == node.ID
		})).Return(false, "metadata", nil)
		defer detector.AssertExpectations(t)

		factory := func() (naro.AnomalyDetector, error) {
			return detector, nil
		}

		handler := &mocks.AnomalyHandler{}
		controller := naro.NewDetectorController(trainingTimePeriod, detectionTimePeriod,
			runInterval, []naro.AnomalyDetectorFactory{factory}, store, clock,
			[]naro.AnomalyHandler{handler},
		)

		controller.Start()
		clock.BlockUntil(1)
		clock.Advance(runInterval)
		controller.Stop()
	})
}
