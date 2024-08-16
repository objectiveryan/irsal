package poller

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/objectiveryan/irsal/internal/common"
	"github.com/objectiveryan/irsal/internal/hyp"
)

type MessageSender interface {
	Send(chatID int64, parentMessageID int, text string) (int, error)
}

type Poller struct {
	Hyp     hyp.ClientFactory
	Storage common.Storage
	Tg      MessageSender
}

func isDone(ctxt context.Context) bool {
	select {
	case <-ctxt.Done():
		return true
	default:
		return false
	}
}

func (p *Poller) handleSub(ctxt context.Context, sub *common.Subscription) error {
	// loop until all annotations are handled
	h := p.Hyp.NewClient(sub.HypToken, sub.HypGroup)
	log.Printf("handleSub(%v)", sub.Key())
	for {
		if isDone(ctxt) {
			return ctxt.Err()
		}
		annots, err := h.AnnotationsAfter(ctxt, sub.SearchAfter)
		if err != nil {
			log.Printf("Failed to get annotations: %v", err)
			return nil
		}
		log.Printf("Got %d annotations", len(annots))
		if len(annots) == 0 {
			return nil
		}
		for i, annot := range annots {
			log.Printf("Annotation [%d/%d] %q", (i + 1), len(annots), annot.ID)
			if annot.Updated == nil {
				log.Println("No 'updated' field in annotation")
				return nil
			}
			_, err := p.handleAnnot(ctxt, annot, sub.ChatID, h)
			if err != nil {
				log.Println(err)
				// Move on to the next subscription; next time try this annotation again
				return nil
			}
			sub.SearchAfter = time.Time(*annot.Updated)
			p.Storage.UpdateSubscription(sub)
		}
	}
}

func (p *Poller) handleAncestor(ctxt context.Context, annotID string, chatID int64, h hyp.Client) (int, error) {
	annot, err := h.Annotation(ctxt, annotID)
	if err != nil {
		return -1, fmt.Errorf("failed to look up annotation %q: %v", annotID, err)
	}
	return p.handleAnnot(ctxt, annot, chatID, h)
}

func RootMessageText(user, text, selection, url string) string {
	return fmt.Sprintf("%s selected \"%s\" and wrote \"%s\"\n%s", user, selection, text, url)
}

func ReplyMessageText(user, text, url string) string {
	return fmt.Sprintf("%s wrote \"%s\"\n%s", user, text, url)
}

func (p *Poller) handleAnnot(ctxt context.Context, annot *hyp.Annotation, chatID int64, h hyp.Client) (int, error) {
	mID, err := p.Storage.MessageID(annot.ID, chatID)
	if err == nil {
		log.Printf("Ignoring annotation %q which already has a chat message %d/%d\n", annot.ID, chatID, mID)
		return mID, nil
	} else if err != common.ErrNotFound {
		return -1, fmt.Errorf("failed to look up existing message for annotation %q: %v", annot.ID, err)
	}

	var parentMessageID int
	if len(annot.References) > 0 {
		// Annotation is a reply.
		// Find Tg message to reply to:
		parentAnnotID := annot.References[len(annot.References)-1]
		log.Printf("Annotation %q is reply to %q", annot.ID, parentAnnotID)
		var err error
		parentMessageID, err = p.Storage.MessageID(parentAnnotID, chatID)
		if err == common.ErrNotFound {
			parentMessageID, err = p.handleAncestor(ctxt, parentAnnotID, chatID, h)
			if err != nil {
				return -1, fmt.Errorf("failed to post ancestors of %q starting from %q: %v", annot.ID, parentAnnotID, err)
			}
		} else if err != nil {
			return -1, fmt.Errorf("failed to look up existing message for annotation %q: %v", parentAnnotID, err)
		}
		// fall through since parentMessageID was set
	}

	if isDone(ctxt) {
		return -1, ctxt.Err()
	}
	var text string
	if parentMessageID == 0 {
		var selection string
		if annot.Targets != nil && len(annot.Targets) > 0 && annot.Targets[0].Selectors.TextQuote != nil {
			selection = *annot.Targets[0].Selectors.TextQuote
		} else {
			log.Println("Warning: no TextQuote selector")
		}
		text = RootMessageText(annot.User, annot.Text, selection, "https://hypothes.is/a/"+annot.ID)
	} else {
		text = ReplyMessageText(annot.User, annot.Text, "https://hypothes.is/a/"+annot.ID)
	}
	messageID, err := p.Tg.Send(chatID, parentMessageID, text)
	if err != nil {
		return -1, fmt.Errorf("failed to send message for annotation: %v", err)
	}
	err = p.Storage.SetMessageID(annot.ID, common.AnnotationMetadata{annot.References, annot.Group, annot.URI}, chatID, messageID)
	if err != nil {
		return -1, err
	}
	return messageID, nil
}

func (p *Poller) RunOnce(ctxt context.Context) error {
	subs, err := p.Storage.Subscriptions()
	if err != nil {
		log.Printf("Failed to get subscriptions: %v", err)
		return err
	} else {
		log.Printf("%d subscriptions", len(subs))
		var lastErr error
		for i, sub := range subs {
			log.Printf("[%d/%d] ChatID=%d Group=%s", i+1, len(subs), sub.ChatID, sub.HypGroup)
			err = p.handleSub(ctxt, sub)
			if err != nil {
				// Ignore but log error
				log.Println(err)
				lastErr = err
			}
		}
		return lastErr
	}
}

// Only returns once the context is closed.
func (p *Poller) Run(ctxt context.Context) error {
	for {
		if isDone(ctxt) {
			return ctxt.Err()
		}

		_ = p.RunOnce(ctxt)

		log.Println("Sleeping to poll Hypothesis again")
		time.Sleep(1 * time.Minute)
	}
}
