package datastruct

type ClientStats struct {
	ClientID            int
	InitalReportingTime float64
	SecreteShareTime    float64
	ShuffleTime         float64
	RevealTime          float64
	FTTime              float64
	UploadBytes         int
	DownloadBytes       int
	Entry               []byte
}

type AuditorReport struct {
	TotalClients      uint32
	MaxSitOut         uint32
	CalculatedEntries [][][]byte
}

type TestRun struct {
	Auditor AuditorReport
	Clients []ClientStats
}

type RunTask struct {
	TotalClients uint32
	MaxSitOut    uint32
}

type ReportStatsReply struct {
	Status bool
}
