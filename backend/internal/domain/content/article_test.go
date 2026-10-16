package content

import "testing"

func TestArticleWorkflow(t *testing.T) {
	draft := &Article{Status: StatusDraft}
	if err := draft.CanSubmit(); err != nil {
		t.Errorf("expected draft article to be submittable, got: %v", err)
	}

	published := &Article{Status: StatusPublished}
	if err := published.CanSubmit(); err == nil {
		t.Errorf("expected published article submission to fail")
	}

	inReview := &Article{Status: StatusInReview}
	if err := inReview.CanPublish(); err != nil {
		t.Errorf("expected in_review article to be publishable, got: %v", err)
	}

	archived := &Article{Status: StatusArchived}
	if err := archived.CanPublish(); err == nil {
		t.Errorf("expected archived article publishing to fail")
	}
}
