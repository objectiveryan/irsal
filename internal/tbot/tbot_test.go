package tbot

import (
	"context"
	"log"
	"reflect"
	"testing"
	"time"

	"github.com/objectiveryan/irsal/internal/check"
	"github.com/objectiveryan/irsal/internal/common"
	"github.com/objectiveryan/irsal/internal/db"
	"github.com/objectiveryan/irsal/internal/fake"
	"github.com/objectiveryan/irsal/internal/hyp"
	"github.com/objectiveryan/irsal/internal/poller"
	tele "gopkg.in/telebot.v3"
)

func TestOnText_NonReplyIsIgnored(t *testing.T) {
	s := db.NewInMemoryStorage()
	h := &fake.HypFactory{}
	tb := &Bot{"token", s, h}

	err := tb.onText(&tele.Message{
		ID:   2,
		Chat: &tele.Chat{ID: 1},
	})
	if err != nil {
		t.Fatalf("Failed to handle message: %v", err)
	}

	if len(h.Annots) != 0 {
		t.Fatalf("onText() created %d annotations; expected 0", len(h.Annots))
	}
	check.NoAnnotationForMessage(t, s, 1, 2)
}

func TestOnText_ReplyNotToBotIsIgnored(t *testing.T) {
	s := db.NewInMemoryStorage()
	h := &fake.HypFactory{}
	tb := &Bot{"token", s, h}

	chat := &tele.Chat{ID: 1}
	err := tb.onText(&tele.Message{
		ID:   3,
		Chat: chat,
		ReplyTo: &tele.Message{
			ID:   2,
			Chat: chat,
		},
	})
	if err != nil {
		t.Fatalf("Failed to handle message: %v", err)
	}

	if len(h.Annots) != 0 {
		t.Fatalf("onText() created %d annotations; expected 0", len(h.Annots))
	}
	check.NoAnnotationForMessage(t, s, 1, 2)
	check.NoAnnotationForMessage(t, s, 1, 3)
}

func TestOnText_ReplyToBot(t *testing.T) {
	s := db.NewInMemoryStorage()
	s.AddSubscription(&common.Subscription{"ht", "g", time.Now(), 1})
	h := &fake.HypFactory{}
	tb := &Bot{"token", s, h}
	// Record a past annotation a0 posted as message 1:2
	err := s.SetMessageID("a0", common.AnnotationMetadata{HypGroup: "g"}, 1, 2)
	if err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	// Handle a new message 1:3 which is a reply to 1:2
	chat := &tele.Chat{ID: 1}
	err = tb.onText(&tele.Message{
		ID:   3,
		Chat: chat,
		ReplyTo: &tele.Message{
			ID:   2,
			Chat: chat,
		},
	})
	if err != nil {
		t.Fatalf("Failed to handle message: %v", err)
	}

	// Confirm a new annotation was created as a reply to a0
	if len(h.Annots) != 1 {
		t.Fatalf("onText() created %d annotations; expected 1", len(h.Annots))
	}
	annot := h.Annots[0]
	expectedRefs := []string{"a0"}
	if !reflect.DeepEqual(annot.References, expectedRefs) {
		t.Errorf("annot.References=%q; expected %q", annot.References, expectedRefs)
	}
	check.AnnotationMessage(t, s, annot.ID, common.AnnotationMetadata{References: expectedRefs, HypGroup: "g"}, 1, 3)
}

func TestOnText_ReplyToReply(t *testing.T) {
	s := db.NewInMemoryStorage()
	s.AddSubscription(&common.Subscription{"ht", "g", time.Now(), 1})
	h := &fake.HypFactory{}
	tb := &Bot{"token", s, h}
	// Record a past annotation a0, which has several ancestors, posted as message 1:2
	err := s.SetMessageID("a0", common.AnnotationMetadata{References: []string{"x", "y", "z"}, HypGroup: "g"}, 1, 2)
	if err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	// Handle a new message 1:3 which is a reply to 1:2
	chat := &tele.Chat{ID: 1}
	err = tb.onText(&tele.Message{
		ID:   3,
		Chat: chat,
		ReplyTo: &tele.Message{
			ID:   2,
			Chat: chat,
		},
	})
	if err != nil {
		t.Fatalf("Failed to handle message: %v", err)
	}

	// Confirm a new annotation was created as a reply to a0
	if len(h.Annots) != 1 {
		t.Fatalf("onText() created %d annotations; expected 1", len(h.Annots))
	}
	annot := h.Annots[0]
	expectedRefs := []string{"x", "y", "z", "a0"}
	if !reflect.DeepEqual(annot.References, expectedRefs) {
		t.Errorf("annot.References=%q; expected %q", annot.References, expectedRefs)
	}
	check.AnnotationMessage(t, s, annot.ID, common.AnnotationMetadata{References: expectedRefs, HypGroup: "g"}, 1, 3)
}

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

func TestPollerAfterBot(t *testing.T) {
	SEARCH_AFTER := time.Now()
	LAST_UPDATED := SEARCH_AFTER.Add(time.Minute)
	sub0 := &common.Subscription{"hyptoken", "g", SEARCH_AFTER, 1}
	h := fake.NewHypFactory([]*hyp.Annotation{
		{ID: "a2", Group: "g", Updated: hyp.ToTimestamp(LAST_UPDATED), Text: "Parent"},
	})
	s := db.NewInMemoryStorage()
	tg := &FakeTg{}
	p := &poller.Poller{h, s, tg}
	err := s.AddSubscription(sub0)
	if err != nil {
		t.Fatal(err)
	}

	onNewAnnotation := make(chan bool)
	h.Observe(func() {
		onNewAnnotation <- true
	})
	afterPoll := make(chan bool)
	go func() {
		<-onNewAnnotation
		defer func() { close(afterPoll) }()

		err := p.RunOnce(context.Background())
		if err != nil {
			t.Errorf("RunOnce() returned err=%+v", err)
			return
		}

		// Only 1 message was sent, for the new root annotation.
		// No message was sent for the reply annotation.
		if len(tg.SentMessages) != 1 {
			t.Errorf("Poller sent %d messages; want 1", len(tg.SentMessages))
			return
		}
		if tg.SentMessages[0].ChatID != 1 {
			t.Errorf("ChatID=%d; want 1", tg.SentMessages[0].ChatID)
		}
		// Message sent for new root annotation is not a reply.
		if tg.SentMessages[0].ParentMessageID != 0 {
			t.Errorf("ParentMessageID=%d; want 0", tg.SentMessages[0].ParentMessageID)
		}
	}()

	tb := &Bot{"tgtoken", s, h}
	// Record a past annotation a1, posted as message 1:2
	err = s.SetMessageID("a0", common.AnnotationMetadata{HypGroup: "g"}, 1, 2)
	if err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	// Handle a new message 1:3 which is a reply to 1:2
	chat := &tele.Chat{ID: 1}
	err = tb.onText(&tele.Message{
		ID:   3,
		Chat: chat,
		ReplyTo: &tele.Message{
			ID:   2,
			Chat: chat,
		},
	})
	if err != nil {
		t.Fatalf("Failed to handle message: %v", err)
	}

	// Annotation was created for reply
	_, _, err = s.AnnotationID(1, 3)
	if err != nil {
		t.Fatalf("Failed to find annotation for handled message: %v", err)
	}

	// Wait for poller to finish
	<-afterPoll
}
