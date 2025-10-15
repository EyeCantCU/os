package files

// Files Enums
//
//go:generate enumer -json -output id_generated.go -type=ID
type ID int

const (
	// Take the zero value with this so that TestMetric{} reports something reasonable.
	UnidentifiedFile ID = iota
	Dmesg
	JournalCtlB0
	SystemdAnalyze
	SystemdCriticalChain
	Proc1Mountinfo
	NestedQemuConsole
	ComplianceHTMLReport
	ComplianceXMLResults
	BootCtl
)
