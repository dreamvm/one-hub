package model

import "time"

// FlushQuotaBatchForTest runs the existing batch writer once without starting
// the process-wide background loop. Only linked into model's test binary.
func FlushQuotaBatchForTest() {
	batchUpdate()
	flushBatchLogs()
}

// InsertStatisticsMonthAtForTest supplies a fixed clock only in the test binary.
func InsertStatisticsMonthAtForTest(now time.Time) error {
	return insertStatisticsMonth(now)
}
