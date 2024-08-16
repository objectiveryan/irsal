package db

import (
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/objectiveryan/irsal/internal/common"
)

type StorageFactory func() common.Storage

func DoTestSetMessageID(newStorage StorageFactory, t *testing.T) {
	t.Run("AnnotID-ChatID duplicates prohibited", func(t *testing.T) {
		s := newStorage()
		err := s.SetMessageID("a", common.AnnotationMetadata{HypGroup: "g"}, 1, 2)
		if err != nil {
			t.Fatalf("SetMessageID() returned err=%v", err)
		}
		err = s.SetMessageID("a", common.AnnotationMetadata{HypGroup: "g"}, 1, 3)
		if err == nil {
			t.Fatalf("SetMessageID() successfully added duplicate")
		}
	})
	t.Run("ChatID-MessageID duplicates prohibited", func(t *testing.T) {
		s := newStorage()
		err := s.SetMessageID("a", common.AnnotationMetadata{HypGroup: "g"}, 1, 2)
		if err != nil {
			t.Fatalf("SetMessageID() returned err=%v", err)
		}
		err = s.SetMessageID("b", common.AnnotationMetadata{HypGroup: "g"}, 1, 2)
		if err == nil {
			t.Fatalf("SetMessageID() successfully added duplicate")
		}
	})
}

func DoTestMessageID(newStorage StorageFactory, t *testing.T) {
	t.Run("Not found", func(t *testing.T) {
		s := newStorage()
		_, err := s.MessageID("a", 1)
		if err != common.ErrNotFound {
			t.Fatalf("MessageID() returned err=%v; want ErrNotFound", err)
		}
	})

	t.Run("Successful lookup", func(t *testing.T) {
		s := newStorage()
		err := s.SetMessageID("a", common.AnnotationMetadata{HypGroup: "g"}, 1, 2)
		if err != nil {
			t.Fatalf("SetMessageID() returned err=%v", err)
		}
		messageID, err := s.MessageID("a", 1)
		if err != nil {
			t.Fatalf("MessageID() returned err=%v", err)
		}
		if messageID != 2 {
			t.Fatalf("MessageID() returned %v; want 2", messageID)
		}
	})

	t.Run("Mismatch", func(t *testing.T) {
		s := newStorage()
		err := s.SetMessageID("a", common.AnnotationMetadata{HypGroup: "g"}, 1, 2)
		if err != nil {
			t.Fatalf("SetMessageID() returned err=%v", err)
		}
		_, err = s.MessageID("b", 1)
		if err != common.ErrNotFound {
			t.Fatalf("MessageID() returned err=%v; want ErrNotFound", err)
		}
		_, err = s.MessageID("a", 2)
		if err != common.ErrNotFound {
			t.Fatalf("MessageID() returned err=%v; want ErrNotFound", err)
		}
	})
}

func DoTestAnnotationID(newStorage StorageFactory, t *testing.T) {
	t.Run("Not found", func(t *testing.T) {
		s := newStorage()
		_, _, err := s.AnnotationID(1, 2)
		if err != common.ErrNotFound {
			t.Fatalf("AnnotationID() returned err=%v; want ErrNotFound", err)
		}
	})

	t.Run("Successful lookup", func(t *testing.T) {
		s := newStorage()
		err := s.SetMessageID("a", common.AnnotationMetadata{References: []string{"p"}, HypGroup: "g"}, 1, 2)
		if err != nil {
			t.Fatalf("SetMessageID() returned err=%v", err)
		}
		annotID, meta, err := s.AnnotationID(1, 2)
		if err != nil {
			t.Fatalf("AnnotationID() returned err=%v", err)
		}
		if annotID != "a" {
			t.Fatalf("AnnotationID() returned annotID=%q; want \"a\"", annotID)
		}
		if !reflect.DeepEqual(meta.References, []string{"p"}) {
			t.Fatalf("AnnotationID() returned refs=%q; want [\"p\"]", meta.References)
		}
		if meta.HypGroup != "g" {
			t.Fatalf("AnnotationID() returned group=%q; want \"g\"", meta.HypGroup)
		}
	})

	t.Run("Mismatch", func(t *testing.T) {
		s := newStorage()
		err := s.SetMessageID("a", common.AnnotationMetadata{HypGroup: "g"}, 1, 2)
		if err != nil {
			t.Fatalf("SetMessageID() returned err=%v", err)
		}
		_, _, err = s.AnnotationID(2, 1)
		if err != common.ErrNotFound {
			t.Fatalf("AnnotationID() returned err=%v; want ErrNotFound", err)
		}
		_, _, err = s.AnnotationID(1, 3)
		if err != common.ErrNotFound {
			t.Fatalf("AnnotationID() returned err=%v; want ErrNotFound", err)
		}
		_, _, err = s.AnnotationID(4, 2)
		if err != common.ErrNotFound {
			t.Fatalf("AnnotationID() returned err=%v; want ErrNotFound", err)
		}
	})
}

func DoTestSubscription(newStorage StorageFactory, t *testing.T) {
	t.Run("Not found", func(t *testing.T) {
		s := newStorage()
		_, err := s.Subscription(1, "g")
		if err != common.ErrNotFound {
			t.Fatalf("Subscription() returned err=%v; want ErrNotFound", err)
		}
	})

	t.Run("Successful lookup", func(t *testing.T) {
		s := newStorage()
		now := time.Now()
		err := s.AddSubscription(&common.Subscription{"token", "group", now, 42})
		if err != nil {
			t.Fatalf("AddSubscription() returned err=%v", err)
		}

		sub, err := s.Subscription(42, "g")
		if err != nil {
			t.Fatalf("Subscription() returned err=%v", err)
		}

		if sub.HypToken != "token" {
			t.Errorf("HypToken=%q; want \"token\"", sub.HypToken)
		}
		if sub.HypGroup != "group" {
			t.Errorf("HypGroup=%q; want \"group\"", sub.HypGroup)
		}
		if sub.SearchAfter != now {
			t.Errorf("SearchAfter=%+v; want %+v", sub.SearchAfter, now)
		}
		if sub.ChatID != 42 {
			t.Errorf("ChatID=%d; want 42", sub.ChatID)
		}
	})

	t.Run("Returns copy", func(t *testing.T) {
		s := newStorage()
		err := s.AddSubscription(&common.Subscription{"token", "group", time.Now(), 42})
		if err != nil {
			t.Fatalf("AddSubscription() returned err=%v", err)
		}
		sub1, err := s.Subscription(42, "g")
		if err != nil {
			t.Fatalf("Subscription() returned err=%v", err)
		}

		// Modify sub1
		sub1.HypToken = "CHANGED"

		sub2, err := s.Subscription(42, "g")
		if err != nil {
			t.Fatalf("Subscription() returned err=%v", err)
		}
		// Database copy unmodified
		if sub2.HypToken != "token" {
			t.Fatalf("Subscription was modified: HypToken=%q; want \"token\"", sub2.HypToken)
		}
	})
}

func DoTestSubscriptions(newStorage StorageFactory, t *testing.T) {
	t.Run("Default empty", func(t *testing.T) {
		s := newStorage()
		subs, err := s.Subscriptions()
		if err != nil {
			t.Fatalf("Subscriptions() returned err=%v", err)
		}
		if len(subs) != 0 {
			t.Fatalf("Subscriptions() returned %+v; want []", subs)
		}
	})

	t.Run("Return copies", func(t *testing.T) {
		s := newStorage()
		err := s.AddSubscription(&common.Subscription{"token", "group", time.Now(), 42})
		if err != nil {
			t.Fatalf("AddSubscription() returned err=%v", err)
		}
		subs, err := s.Subscriptions()
		if err != nil {
			t.Fatalf("Subscriptions() returned err=%v", err)
		}
		if len(subs) != 1 {
			t.Fatalf("Subscriptions() returned %d subs; want 1", len(subs))
		}

		// Modify sub
		subs[0].HypToken = "CHANGED"

		subs, err = s.Subscriptions()
		if err != nil {
			t.Fatalf("Subscriptions() returned err=%v", err)
		}
		if len(subs) != 1 {
			t.Fatalf("Subscriptions() returned %d subs; want 1", len(subs))
		}
		// Database copy unmodified
		if subs[0].HypToken != "token" {
			t.Fatalf("Subscription was modified: HypToken=%q; want \"token\"", subs[0].HypToken)
		}
	})
}

func DoTestAddSubscription(newStorage StorageFactory, t *testing.T) {
	t.Run("Duplicates prohibited", func(t *testing.T) {
		sub := &common.Subscription{"token", "group", time.Now(), 42}
		s := newStorage()

		err := s.AddSubscription(sub)
		if err != nil {
			t.Fatalf("AddSubscription() returned err=%v", err)
		}
		err = s.AddSubscription(sub)
		if err == nil {
			t.Fatalf("AddSubscription() successfully added duplicate")
		}

	})
	t.Run("Save copy", func(t *testing.T) {
		sub := &common.Subscription{"token", "group", time.Now(), 42}
		s := newStorage()

		err := s.AddSubscription(sub)
		if err != nil {
			t.Fatalf("AddSubscription() returned err=%v", err)
		}
		sub.HypToken = "CHANGED"

		subs, err := s.Subscriptions()
		if err != nil {
			t.Fatalf("Subscriptions() returned err=%v", err)
		}
		if len(subs) != 1 {
			t.Fatalf("Subscriptions() returned %d subs; want 1", len(subs))
		}
		// Database copy unmodified
		if subs[0].HypToken != "token" {
			t.Fatalf("Subscription was modified: HypToken=%q; want \"token\"", subs[0].HypToken)
		}
	})
}

func DoTestUpdateSubscription(newStorage StorageFactory, t *testing.T) {
	t.Run("Save copy", func(t *testing.T) {
		s := newStorage()
		err := s.AddSubscription(&common.Subscription{"token", "group", time.Now(), 42})
		if err != nil {
			t.Fatalf("AddSubscription() returned err=%v", err)
		}
		subs, err := s.Subscriptions()
		if err != nil {
			t.Fatalf("Subscriptions() returned err=%v", err)
		}
		if len(subs) != 1 {
			t.Fatalf("Subscriptions() returned %d subs; want 1", len(subs))
		}

		sub := subs[0]
		sub.HypToken = "CHANGED1"
		s.UpdateSubscription(sub)
		sub.HypToken = "CHANGED2"

		subs, err = s.Subscriptions()
		if err != nil {
			t.Fatalf("Subscriptions() returned err=%v", err)
		}
		if len(subs) != 1 {
			t.Fatalf("Subscriptions() returned %d subs; want 1", len(subs))
		}
		if subs[0].HypToken != "CHANGED1" {
			t.Fatalf("Subscription was not modified: HypToken=%q; want \"CHANGED1\"", subs[0].HypToken)
		}
	})

	t.Run("Returns ErrNotFound if no match", func(t *testing.T) {
		s := newStorage()
		err := s.AddSubscription(&common.Subscription{"token", "group", time.Now(), 42})
		if err != nil {
			t.Fatalf("AddSubscription() returned err=%v", err)
		}
		err = s.UpdateSubscription(&common.Subscription{"token", "group2", time.Now(), 42})
		if err != common.ErrNotFound {
			t.Fatalf("err=%v; want ErrNotFound", err)
		}
		err = s.UpdateSubscription(&common.Subscription{"token", "group", time.Now(), 99})
		if err != common.ErrNotFound {
			t.Fatalf("err=%v; want ErrNotFound", err)
		}
	})
}

func DoTestLock(newStorage StorageFactory, t *testing.T) {
	s := newStorage()
	var i int
	var wg sync.WaitGroup
	const n = 1000
	wg.Add(n)
	f := func() {
		s.Lock()
		i += 1
		s.Unlock()
		wg.Done()
	}
	for j := 0; j < n; j++ {
		go f()
	}
	wg.Wait()
	if i != n {
		t.Errorf("i=%d; want %d", i, n)
	}
}

func DoTests(newStorage StorageFactory, t *testing.T) {
	t.Run("SetMessageID", func(t *testing.T) { DoTestSetMessageID(newStorage, t) })
	t.Run("MessageID", func(t *testing.T) { DoTestMessageID(newStorage, t) })
	t.Run("AnnotationID", func(t *testing.T) { DoTestAnnotationID(newStorage, t) })
	t.Run("Subscriptions", func(t *testing.T) { DoTestSubscriptions(newStorage, t) })
	t.Run("AddSubscription", func(t *testing.T) { DoTestAddSubscription(newStorage, t) })
	t.Run("UpdateSubscription", func(t *testing.T) { DoTestUpdateSubscription(newStorage, t) })
	t.Run("Lock", func(t *testing.T) { DoTestLock(newStorage, t) })
}

func TestDbStorage(t *testing.T) {
	DoTests(NewInMemoryStorage, t)
}
