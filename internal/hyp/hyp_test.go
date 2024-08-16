package hyp

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

const sampleAnnotationJSON = `{"created":"2021-07-24T23:46:38.502955+00:00","document":{"title":["Document Title"]},"flagged":false,"group":"fakegroup","hidden":false,"id":"fake-id","links":{"html":"https://hypothes.is/a/fake-id","incontext":"https://hyp.is/fake-id/example.test/fakeurl/","json":"https://hypothes.is/api/annotations/fake-id"},"permissions":{"admin":["acct:fakeuser@hypothes.is"],"delete":["acct:fakeuser@hypothes.is"],"read":["group:fakegroup"],"update":["acct:fakeuser@hypothes.is"]},"tags":[],"target":[{"selector":[{"endContainer":"/div[2]/div[1]/div[1]/div[1]/main[1]/article[1]/div[1]/div[1]/div[1]/div[1]/section[2]/div[1]/div[1]/div[1]/div[1]/div[1]/section[1]/div[1]/div[1]/div[2]/div[1]/div[1]/div[1]/div[1]/div[1]/p[1]","endOffset":6,"startContainer":"/div[2]/div[1]/div[1]/div[1]/main[1]/article[1]/div[1]/div[1]/div[1]/div[1]/section[2]/div[1]/div[1]/div[1]/div[1]/div[1]/section[1]/div[1]/div[1]/div[2]/div[1]/div[1]/div[1]/div[1]/div[1]/p[1]","startOffset":0,"type":"RangeSelector"},{"end":1933,"start":1927,"type":"TextPositionSelector"},{"exact":"During","prefix":"\t\t\t\t\t\n\t\t\t\t\t\t\n\t\t\t\t\n\t\t\t\t\t\t\t\t\n\t\t\t\t\t","suffix":" the first year, three core text","type":"TextQuoteSelector"}],"source":"https://example.test/fakeurl/"}],"text":"Asdf","updated":"2021-07-24T23:46:38.502955+00:00","uri":"https://example.test/fakeurl/","user":"acct:fakeuser@hypothes.is","user_info":{"display_name":null}}`

func TestParseAnnotation(t *testing.T) {
	var annot Annotation
	if err := json.Unmarshal([]byte(sampleAnnotationJSON), &annot); err != nil {
		t.Fatal("Failed to unmarshal json:", err)
	}
	if annot.ID != "fake-id" {
		t.Errorf("ID=%q; want \"fake-id\"", annot.ID)
	}
	if annot.Updated == nil {
		t.Errorf("Updated=nil")
	} else {
		// "2021-07-24T23:46:38.502955+00:00"
		updated := time.Time(*annot.Updated)
		if want := time.Date(2021, 7, 24, 23, 46, 38, 502955000, time.UTC); !updated.Equal(want) {
			t.Errorf("Updated=%+v; want %+v", updated, want)
		}
	}
	if annot.User != "acct:fakeuser@hypothes.is" {
		t.Errorf("User=%q; want \"acct:fakeuser@hypothes.is\"", annot.User)
	}
	if annot.URI != "https://example.test/fakeurl/" {
		t.Errorf("URI=%q; want \"https://example.test/fakeurl/\"", annot.URI)
	}
	if annot.Text != "Asdf" {
		t.Errorf("Text=%q; want \"Asdf\"", annot.Text)
	}
	if annot.Group != "fakegroup" {
		t.Errorf("Group=%q; want \"fakegroup\"", annot.Group)
	}
	if annot.Permissions == nil {
		t.Error("Permissions=nil")
	} else {
		want := []string{"acct:fakeuser@hypothes.is"}
		if !reflect.DeepEqual(annot.Permissions.Admin, want) {
			t.Errorf("Permissions.Admin=%q; want %q", annot.Permissions.Admin, want)
		}
		if !reflect.DeepEqual(annot.Permissions.Delete, want) {
			t.Errorf("Permissions.Delete=%q; want %q", annot.Permissions.Delete, want)
		}
		if !reflect.DeepEqual(annot.Permissions.Update, want) {
			t.Errorf("Permissions.Update=%q; want %q", annot.Permissions.Update, want)
		}
		want = []string{"group:fakegroup"}
		if !reflect.DeepEqual(annot.Permissions.Read, want) {
			t.Errorf("Permissions.Read=%q; want %q", annot.Permissions.Read, want)
		}
	}
	if len(annot.Targets) != 1 {
		t.Errorf("%d targets; want 1", len(annot.Targets))
	} else {
		target := annot.Targets[0]
		if target == nil {
			t.Error("target=nil")
		} else {
			if want := "https://example.test/fakeurl/"; target.Source != want {
				t.Errorf("target.Source=%q; want %q", target.Source, want)
			}
			if target.Selectors.TextQuote == nil {
				t.Error("target.Selectors.TextQuote=nil")
			} else if want := "During"; *target.Selectors.TextQuote != want {
				t.Errorf("target.Selectors.TextQuote=%q; want %q", *target.Selectors.TextQuote, want)
			}
		}
	}
}

func TestSerializeAnnotation(t *testing.T) {
	data, err := json.Marshal(NewAnnotationTemplate("content", "grp", []string{"ref1", "ref2"}, "http://example.test/foo"))
	if err != nil {
		t.Fatal("Failed to marshal annotation:", err)
	}
	s := string(data)
	// unwanted fields
	for _, field := range []string{"id", "updated", "user", "target"} {
		substr := "\"" + field + "\""
		if strings.Contains(s, substr) {
			t.Errorf("%q contains %q", s, substr)
		}
	}
	// wanted fields
	for _, field := range []string{"uri", "text", "group", "permissions", "references"} {
		substr := "\"" + field + "\""
		if !strings.Contains(s, substr) {
			t.Errorf("%q doesn't contain %q", s, substr)
		}
	}
}
