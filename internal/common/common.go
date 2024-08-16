package common

import (
	"errors"
	"fmt"
	"time"
)

var ErrNotFound = errors.New("not found")

type Subscription struct {
	HypToken    string
	HypGroup    string
	SearchAfter time.Time
	ChatID      int64
}

type SubKey struct {
	HypGroup string
	ChatID   int64
}

func (k SubKey) String() string {
	return fmt.Sprintf("SK{%s:%d}", k.HypGroup, k.ChatID)
}

func (sub *Subscription) Key() SubKey {
	return SubKey{sub.HypGroup, sub.ChatID}
}

type AnnotationMetadata struct {
	References []string
	HypGroup   string
	URI        string
}

type Storage interface {
	Close() error

	MessageID(annotID string, chatID int64) (int, error)
	SetMessageID(annotID string, meta AnnotationMetadata, chatID int64, messageID int) error
	AnnotationID(chatID int64, messageID int) (string, AnnotationMetadata, error)

	AddSubscription(*Subscription) error
	Subscriptions() ([]*Subscription, error)
	UpdateSubscription(sub *Subscription) error
	Subscription(chatID int64, hypGroup string) (*Subscription, error)

	Lock()
	Unlock()
}
