package naro

// AnomalyDetector is a type that can be trained to detect issues
// within new NodeTimePeriodSummaries.
type AnomalyDetector interface {
	Train(summaries <-chan *NodeTimePeriodSummary, done chan<- struct{}) error
	IsAnomaly(ns *NodeTimePeriodSummary) (bool, error)
}
