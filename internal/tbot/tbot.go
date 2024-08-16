package tbot

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	tele "gopkg.in/telebot.v3"
	"gopkg.in/telebot.v3/middleware"

	"github.com/objectiveryan/irsal/internal/common"
	"github.com/objectiveryan/irsal/internal/hyp"
)

type Bot struct {
	Token   string
	Storage common.Storage
	Hyp     hyp.ClientFactory
}

func formatUser(user *tele.User) string {
	var nameParts []string
	if user.FirstName != "" {
		nameParts = append(nameParts, user.FirstName)
	}
	if user.LastName != "" {
		nameParts = append(nameParts, user.LastName)
	}
	var name string
	if len(nameParts) > 0 {
		name = strings.Join(nameParts, " ")
	}
	if user.Username != "" {
		if name != "" {
			return fmt.Sprintf("%s (%s)", name, user.Username)
		}
		return user.Username
	}
	if name != "" {
		return name
	}
	return "Someone"
}

func MessageText(msg *tele.Message) string {
	return fmt.Sprintf("%s wrote \"%s\"", formatUser(msg.Sender), msg.Text)
}

func (tb *Bot) onText(msg *tele.Message) error {
	if msg == nil {
		log.Println("Ignoring OnText with no message")
		return nil
	}
	log.Printf("onText: ChatID=%d MessageID=%d Text=%q", msg.Chat.ID, msg.ID, msg.Text)
	if msg.ReplyTo == nil {
		log.Println("Ignoring OnText which is not a reply")
		return nil
	}
	if msg.ReplyTo.Chat.ID != msg.Chat.ID {
		log.Println("Ignoring OnText reply in different chat from parent")
		return nil
	}
	parentAnnotID, parentMeta, err := tb.Storage.AnnotationID(msg.Chat.ID, msg.ReplyTo.ID)
	if err == common.ErrNotFound {
		log.Println("Ignoring OnText reply to non-annotation message")
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to look up annotation for message: %v", err)
	}

	sub, err := tb.Storage.Subscription(msg.Chat.ID, parentMeta.HypGroup)
	if err == common.ErrNotFound {
		// This seems unlikely given that there's an annotation associated with the parent message.
		// But it can happen if we delete a subscription.
		log.Println("Ignoring OnText reply to chat without subscription")
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to look up subscription for chat: %v", err)
	}

	refs := append(parentMeta.References, parentAnnotID)
	// Lock the storage so the poller can't try to look up the message ID for the annotation before we record it.
	tb.Storage.Lock()
	defer tb.Storage.Unlock()
	annotID, err := tb.Hyp.NewClient(sub.HypToken, sub.HypGroup).Reply(context.TODO(), MessageText(msg), refs, parentMeta.URI)
	if err != nil {
		log.Printf("Failed to post annotation reply to %v: %v", parentAnnotID, err)
		return err
	}
	err = tb.Storage.SetMessageID(annotID, common.AnnotationMetadata{refs, sub.HypGroup, parentMeta.URI}, msg.Chat.ID, msg.ID)
	if err != nil {
		log.Printf("Failed to record annotation for chat reply: %v", err)
	} else {
		log.Printf("Successfully recorded annotation for chat reply")
	}
	return err
}

type BotRunner struct {
	b       *Bot
	tb      *tele.Bot
	tbReady chan struct{}
}

func NewBotRunner(b *Bot) *BotRunner {
	return &BotRunner{b, nil, make(chan struct{})}
}

// for poller.MessageSender
func (r *BotRunner) Send(chatID int64, parentMessageID int, text string) (messageID int, err error) {
	<-r.tbReady
	var msg *tele.Message
	if parentMessageID == 0 {
		msg, err = r.tb.Send(&tele.Chat{ID: chatID}, text)
	} else {
		msg, err = r.tb.Reply(&tele.Message{ID: parentMessageID, Chat: &tele.Chat{ID: chatID}}, text)
	}
	if err != nil {
		return -1, err
	}
	return msg.ID, nil
}

func (r *BotRunner) Run(ctxt context.Context) error {
	pref := tele.Settings{
		Token:  r.b.Token,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

	tb, err := tele.NewBot(pref)
	if err != nil {
		return err
	}
	r.tb = tb
	close(r.tbReady)
	tb.Use(middleware.Logger())

	tb.Handle(tele.OnText, func(c tele.Context) error {
		log.Println("tele.OnText")
		return r.b.onText(c.Message())
	})

	go func() {
		<-ctxt.Done()
		log.Println("BotRunner.Run context done. Stopping bot")
		tb.Stop()
	}()
	log.Println("Starting bot")
	tb.Start()
	return ctxt.Err()
}
