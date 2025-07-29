package metrics

// Metrics Enums
//
//go:generate enumer -json -output id_generated.go -type=ID
type ID int

const (
	// Take the zero value with this so that TestMetric{} reports something reasonable.
	UnidentifiedMetric ID = iota
	SSHDStartTime
	MultiUserTarget
	SystemdState
)
