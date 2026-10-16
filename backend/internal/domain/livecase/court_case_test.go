package livecase

import "testing"

func TestCourtCaseCanPostUpdate(t *testing.T) {
	listedCase := &CourtCase{Status: CaseStatusListed}
	if err := listedCase.CanPostUpdate(); err != nil {
		t.Errorf("expected listed case to allow updates, got: %v", err)
	}

	hearingCase := &CourtCase{Status: CaseStatusHearing}
	if err := hearingCase.CanPostUpdate(); err != nil {
		t.Errorf("expected hearing case to allow updates, got: %v", err)
	}

	decidedCase := &CourtCase{Status: CaseStatusDecided}
	if err := decidedCase.CanPostUpdate(); err == nil {
		t.Errorf("expected decided case to reject updates")
	}

	disposedCase := &CourtCase{Status: CaseStatusDisposed}
	if err := disposedCase.CanPostUpdate(); err == nil {
		t.Errorf("expected disposed case to reject updates")
	}
}
