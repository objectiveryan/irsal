package fake

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/objectiveryan/irsal/internal/common"
	"github.com/objectiveryan/irsal/internal/hyp"
)

type HypFactory struct {
	Annots    []*hyp.Annotation
	observers []func()
}

func NewHypFactory(annots []*hyp.Annotation) *HypFactory {
	return &HypFactory{annots, nil /*observers*/}
}

func (f *HypFactory) Observe(fn func()) {
	f.observers = append(f.observers, fn)
}

func (f *HypFactory) notify() {
	for _, o := range f.observers {
		o()
	}
}

func (f *HypFactory) NewClient(token, group string) hyp.Client {
	return &Hyp{0, token, group, f}
}

type Hyp struct {
	nextID int
	token  string
	group  string
	parent *HypFactory
}

func (h *Hyp) Annotation(ctxt context.Context, id string) (*hyp.Annotation, error) {
	for _, a := range h.parent.Annots {
		if a.ID == id {
			return a, nil
		}
	}
	return nil, common.ErrNotFound
}

func (h *Hyp) AnnotationsAfter(ctxt context.Context, t time.Time) ([]*hyp.Annotation, error) {
	var res []*hyp.Annotation
	for _, a := range h.parent.Annots {
		if a.Group == h.group && time.Time(*a.Updated).After(t) {
			copy := *a
			res = append(res, &copy)
		}
	}
	return res, nil
}

func (h *Hyp) Reply(ctxt context.Context, text string, references []string, uri string) (annotID string, err error) {
	h.nextID++
	annot := hyp.NewAnnotationTemplate(text, h.group, references, uri)
	annot.ID = fmt.Sprintf("a%d", h.nextID)
	annot.Updated = hyp.ToTimestamp(time.Now())
	h.parent.Annots = append(h.parent.Annots, annot)
	log.Printf("FakeHyp: Posted new annotation %q", annot.ID)
	h.parent.notify()
	return annot.ID, nil
}
