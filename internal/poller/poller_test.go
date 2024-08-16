package poller

import (
	"context"
	"log"
	"testing"
	"time"

	"github.com/objectiveryan/irsal/internal/check"
	"github.com/objectiveryan/irsal/internal/common"
	"github.com/objectiveryan/irsal/internal/db"
	"github.com/objectiveryan/irsal/internal/fake"
	"github.com/objectiveryan/irsal/internal/hyp"
)

type SentMessage struct {
	ChatID          int64
	MessageID       int
	ParentMessageID int
	Text            string
}

type FakeTg struct {
	NextMessageID int
	SentMessages  []*SentMessage
}

func (tg *FakeTg) Send(chatID int64, parentMessageID int, text string) (int, error) {
	tg.NextMessageID++
	log.Printf("Sending messageID=%d in chatID=%d with parent %d: %q", tg.NextMessageID, chatID, parentMessageID, text)
	msg := &SentMessage{chatID, tg.NextMessageID, parentMessageID, text}
	tg.SentMessages = append(tg.SentMessages, msg)
	return msg.MessageID, nil
}

// func getSub(s common.Storage, sub *common.Subscription) *common.Subscription {
// 	subs, err := s.Subscriptions()
// 	if err != nil {
// 		panic(err)
// 	}
// 	for _, ssub := range subs {
// 		if ssub.HypGroup == sub.HypGroup && ssub.ChatID == sub.ChatID {
// 			return ssub
// 		}
// 	}
// 	panic("sub not found")
// }

func TestHandleSub_NewAnnotation(t *testing.T) {
	SEARCH_AFTER := time.Unix(1, 0)
	LAST_UPDATED := time.Unix(2, 0)
	const CHAT_ID = 42
	subTemplate := &common.Subscription{"ht", "grp", SEARCH_AFTER, CHAT_ID}
	h := fake.NewHypFactory([]*hyp.Annotation{{ID: "a1", Group: "grp", Updated: hyp.ToTimestamp(LAST_UPDATED)}})
	s := db.NewInMemoryStorage()
	tg := &FakeTg{}
	p := &Poller{h, s, tg}
	s.AddSubscription(subTemplate)
	subs, err := s.Subscriptions()
	if err != nil {
		t.Fatalf("Subscriptions() returned err=%v", err)
	}

	err = p.handleSub(context.TODO(), subs[0])
	if err != nil {
		t.Fatalf("handleSub() returned err=%v", err)
	}

	// Subscription.SearchAfter was updated in storage
	sub, err := s.Subscription(CHAT_ID, "grp")
	if err != nil {
		t.Fatalf("Failed to look up subscription: %v", err)
	}
	if sub.SearchAfter != LAST_UPDATED {
		t.Errorf("sub.SearchAfter=%v; expected %v", sub.SearchAfter, LAST_UPDATED)
	}
	// One Telegram message was sent
	if len(tg.SentMessages) != 1 {
		t.Fatalf("len(SentMessages)=%d; expected 1", len(tg.SentMessages))
	}
	// Annotation<-->Message was recorded
	check.AnnotationMessage(t, s, "a1", common.AnnotationMetadata{HypGroup: "grp"}, CHAT_ID, tg.SentMessages[0].MessageID)
}

func TestHandleSub_Reply(t *testing.T) {
	SEARCH_AFTER := time.Unix(1, 0)
	LAST_UPDATED1 := time.Unix(2, 0)
	LAST_UPDATED2 := time.Unix(3, 0)
	const CHAT_ID = 42
	subTemplate := &common.Subscription{"ht", "grp", SEARCH_AFTER, CHAT_ID}
	h := fake.NewHypFactory([]*hyp.Annotation{
		{ID: "a1", Group: "grp", Updated: hyp.ToTimestamp(LAST_UPDATED1), Text: "Parent"},
		{ID: "a2", Group: "grp", Updated: hyp.ToTimestamp(LAST_UPDATED2), Text: "Child", References: []string{"a1"}},
	})
	s := db.NewInMemoryStorage()
	tg := &FakeTg{}
	p := &Poller{h, s, tg}
	s.AddSubscription(subTemplate)
	subs, err := s.Subscriptions()
	if err != nil {
		t.Fatalf("Subscriptions() returned err=%v", err)
	}

	err = p.handleSub(context.TODO(), subs[0])
	if err != nil {
		t.Fatalf("handleSub() returned err=%v", err)
	}

	// Subscription.SearchAfter was updated in storage
	sub, err := s.Subscription(CHAT_ID, subTemplate.HypGroup)
	if err != nil {
		t.Fatalf("Failed to look up subscription: %v", err)
	}
	if sub.SearchAfter != LAST_UPDATED2 {
		t.Errorf("sub.SearchAfter=%v; expected %v", sub.SearchAfter, LAST_UPDATED2)
	}
	// Two Telegram messages were sent
	if len(tg.SentMessages) != 2 {
		t.Errorf("len(SentMessages)=%d; expected 2", len(tg.SentMessages))
	}
	// Annotation<-->Message was recorded
	check.AnnotationMessage(t, s, "a1", common.AnnotationMetadata{HypGroup: "grp"}, CHAT_ID, tg.SentMessages[0].MessageID)
	check.AnnotationMessage(t, s, "a2", common.AnnotationMetadata{References: []string{"a1"}, HypGroup: "grp"}, CHAT_ID, tg.SentMessages[1].MessageID)
	// Second Telegram message was reply to first
	if tg.SentMessages[1].ParentMessageID != tg.SentMessages[0].MessageID {
		t.Errorf("SentMessages[1].ParentMessageID=%v; expected %v", tg.SentMessages[1].ParentMessageID, tg.SentMessages[0].MessageID)
	}
}

func TestHandleSub_ReplyToUnposted(t *testing.T) {
	LAST_UPDATED1 := time.Unix(1, 0)
	SEARCH_AFTER := time.Unix(2, 0)
	LAST_UPDATED2 := time.Unix(3, 0)
	const CHAT_ID = 42
	subTemplate := &common.Subscription{"ht", "grp", SEARCH_AFTER, CHAT_ID}
	h := fake.NewHypFactory([]*hyp.Annotation{
		{ID: "a1", Group: "grp", Updated: hyp.ToTimestamp(LAST_UPDATED1), Text: "Parent"},
		{ID: "a2", Group: "grp", Updated: hyp.ToTimestamp(LAST_UPDATED2), Text: "Child", References: []string{"a1"}},
	})
	s := db.NewInMemoryStorage()
	tg := &FakeTg{}
	p := &Poller{h, s, tg}
	s.AddSubscription(subTemplate)
	subs, err := s.Subscriptions()
	if err != nil {
		t.Fatalf("Subscriptions() returned err=%v", err)
	}

	// First confirm that we really will only get a2 with the initial query:
	annots, err := h.NewClient(subs[0].HypToken, subs[0].HypGroup).AnnotationsAfter(context.Background(), SEARCH_AFTER)
	if err != nil {
		t.Fatalf("AnnotationsAfter() returned err=%v", err)
	}
	if len(annots) != 1 {
		t.Fatalf("AnnotationsAfter() returned %d annotations, expected 1", len(annots))
	}
	if annots[0].ID != "a2" {
		t.Fatalf("AnnotationsAfter() returned annotation %q, expected \"a2\"", annots[0].ID)
	}

	err = p.handleSub(context.TODO(), subs[0])
	if err != nil {
		t.Fatalf("handleSub() returned err=%v", err)
	}

	// Subscription.SearchAfter was updated in storage
	sub, err := s.Subscription(CHAT_ID, subTemplate.HypGroup)
	if err != nil {
		t.Fatalf("Failed to look up subscription: %v", err)
	}
	if sub.SearchAfter != LAST_UPDATED2 {
		t.Errorf("sub.SearchAfter=%v; expected %v", sub.SearchAfter, LAST_UPDATED2)
	}
	// Two Telegram messages were sent
	if len(tg.SentMessages) != 2 {
		t.Errorf("len(SentMessages)=%d; expected 2", len(tg.SentMessages))
	}
	// Annotation<-->Message was recorded
	check.AnnotationMessage(t, s, "a1", common.AnnotationMetadata{HypGroup: "grp"}, CHAT_ID, tg.SentMessages[0].MessageID)
	check.AnnotationMessage(t, s, "a2", common.AnnotationMetadata{References: []string{"a1"}, HypGroup: "grp"}, CHAT_ID, tg.SentMessages[1].MessageID)
	// Second Telegram message was reply to first
	if tg.SentMessages[1].ParentMessageID != tg.SentMessages[0].MessageID {
		t.Errorf("SentMessages[1].ParentMessageID=%v; expected %v", tg.SentMessages[1].ParentMessageID, tg.SentMessages[0].MessageID)
	}
}
