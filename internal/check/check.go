package check

import (
	"reflect"
	"testing"

	"github.com/objectiveryan/irsal/internal/common"
)

func AnnotationMessage(t *testing.T, s common.Storage, annotID string, meta common.AnnotationMetadata, chatID int64, messageID int) bool {
	t.Helper()
	aID, aMeta, err := s.AnnotationID(chatID, messageID)
	if err != nil {
		t.Errorf("AnnotationID() returned err=%v", err)
		return false
	}
	if aID != annotID {
		t.Errorf("AnnotationID() returned annotID=%q; expected %q", aID, annotID)
		return false
	}
	if !reflect.DeepEqual(aMeta.References, meta.References) {
		t.Errorf("AnnotationID() returned references=%q; expected %q", aMeta.References, meta.References)
		return false
	}
	if aMeta.HypGroup != meta.HypGroup {
		t.Errorf("AnnotationID() returned group=%q; expected %q", aMeta.HypGroup, meta.HypGroup)
		return false
	}
	mID, err := s.MessageID(annotID, chatID)
	if err != nil {
		t.Errorf("MessageID() returned err=%v", err)
		return false
	}
	if mID != messageID {
		t.Errorf("MessageID() returned %d; expected %d", mID, messageID)
		return false
	}
	return true
}

func NoAnnotationForMessage(t *testing.T, s common.Storage, chatID int64, messageID int) bool {
	t.Helper()
	annotID, _, err := s.AnnotationID(chatID, messageID)
	if err == common.ErrNotFound {
		return true
	} else if err != nil {
		t.Errorf("AnnotationID() returned err=%v", err)
		return false
	}
	t.Errorf("AnnotationID() returned annotID=%q; expected err=ErrNotFound", annotID)
	return false
}
