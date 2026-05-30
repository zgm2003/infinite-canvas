package service

import "testing"

func TestStopPromptSyncSchedulerStopsAndAllowsRestart(t *testing.T) {
	StopPromptSyncScheduler()
	setupServiceTestDB(t)
	t.Cleanup(StopPromptSyncScheduler)

	StartPromptSyncScheduler()
	if promptSyncCron == nil || len(promptSyncCron.Entries()) == 0 {
		t.Fatal("expected prompt sync scheduler to start with configured entries")
	}

	StopPromptSyncScheduler()
	if promptSyncCron != nil {
		t.Fatal("expected prompt sync scheduler to clear global cron after stop")
	}

	StartPromptSyncScheduler()
	if promptSyncCron == nil || len(promptSyncCron.Entries()) == 0 {
		t.Fatal("expected prompt sync scheduler to restart after stop")
	}
}
