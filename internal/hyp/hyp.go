package hyp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"
)

const timestampFormat = "2006-01-02T15:04:05.999999-07:00" // "2022-09-01T00:00:00.000000+00:00"

type searchResponse struct {
	Rows  []*Annotation `json:"rows"`
	Total int           `json:"total"`
}

type Annotation struct {
	ID          string       `json:"id,omitempty"`
	Updated     *Timestamp   `json:"updated,omitempty"`
	User        string       `json:"user,omitempty"`
	URI         string       `json:"uri"`
	Text        string       `json:"text"`
	Group       string       `json:"group"`
	Permissions *Permissions `json:"permissions"`
	Targets     []*Target    `json:"target,omitempty"`
	References  []string     `json:"references,omitempty"`
}

type Permissions struct {
	Read   []string `json:"read,omitempty"`
	Admin  []string `json:"admin,omitempty"`
	Update []string `json:"update,omitempty"`
	Delete []string `json:"delete,omitempty"`
}

type Timestamp time.Time

func ToTimestamp(t time.Time) *Timestamp {
	ts := Timestamp(t)
	return &ts
}

type Target struct {
	Source    string    `json:"source"`
	Selectors Selectors `json:"selector"`
}

type Selectors struct {
	TextQuote *string
}

type rawSelector struct {
	Type  string
	Exact *string
}

func (s *Selectors) UnmarshalJSON(data []byte) error {
	log.Println("UnmarshalJSON[Selectors]: " + string(data))
	var raw []rawSelector
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("failed to unmarshal Selectors: %v", err)
	}
	for _, r := range raw {
		if r.Type == "TextQuoteSelector" {
			s.TextQuote = r.Exact
			break
		}
	}
	return nil
}

func (ts *Timestamp) MarshalJSON() ([]byte, error) {
	t := time.Time(*ts)
	if !t.IsZero() {
		panic("Marshaling non-zero Timestamp")
	}
	return nil, nil
}

func (ts *Timestamp) UnmarshalJSON(data []byte) error {
	log.Println("UnmarshalJSON[Timestamp]: " + string(data))
	var raw string
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("failed to unmarshal Timestamp: %v", err)
	}
	t, err := time.Parse(timestampFormat, raw)
	if err != nil {
		return fmt.Errorf("failed to parse Timestamp: %v", err)
	}
	*ts = Timestamp(t)
	return nil
}

type ClientFactory interface {
	NewClient(token, group string) Client
}

type clientFactory struct {
}

func NewClientFactory() ClientFactory {
	return &clientFactory{}
}

func (f *clientFactory) NewClient(token, group string) Client {
	return &client{token, group}
}

type Client interface {
	Annotation(ctxt context.Context, ID string) (*Annotation, error)
	AnnotationsAfter(ctxt context.Context, t time.Time) ([]*Annotation, error)
	Reply(ctxt context.Context, text string, references []string, uri string) (annotID string, err error)
}

type client struct {
	Token string
	Group string
}

func (c *client) Annotation(ctxt context.Context, ID string) (*Annotation, error) {
	client := &http.Client{}
	req, err := http.NewRequestWithContext(ctxt, "GET", "https://api.hypothes.is/api/annotations/"+ID, nil)
	if err != nil {
		panic(fmt.Sprintf("Failed to create http request for fetch: %v", err))
	}
	req.Header = map[string][]string{
		"Authorization": {"Bearer " + c.Token},
	}
	httpResp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to perform fetch: %v", err)
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to perform fetch: status=%v; want 200", httpResp.StatusCode)
	}
	decoder := json.NewDecoder(httpResp.Body)
	var annot Annotation
	if err := decoder.Decode(&annot); err != nil {
		return nil, fmt.Errorf("failed to decode fetch response: %v", err)
	}
	return &annot, nil
}

func (c *client) AnnotationsAfter(ctxt context.Context, t time.Time) ([]*Annotation, error) {
	searchAfter := t.Format(timestampFormat)
	log.Println("searchAfter=", searchAfter)
	query := url.Values{
		"sort":         {"updated"},
		"order":        {"asc"},
		"group":        {c.Group},
		"search_after": {searchAfter},
	}.Encode()
	log.Println("query=", query)
	req, err := http.NewRequestWithContext(ctxt, "GET", "https://api.hypothes.is/api/search?"+query, nil)
	if err != nil {
		panic(fmt.Sprintf("Failed to create http request for search: %v", err))
	}
	req.Header = map[string][]string{
		"Authorization": {"Bearer " + c.Token},
	}
	httpResp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to perform search: %v", err)
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to perform search: status=%v; want 200", httpResp.StatusCode)
	}
	decoder := json.NewDecoder(httpResp.Body)
	var resp searchResponse
	if err := decoder.Decode(&resp); err != nil {
		return nil, fmt.Errorf("failed to decode search response: %v", err)
	}
	return resp.Rows, nil
}

func (c *client) Reply(ctxt context.Context, text string, references []string, uri string) (annotID string, err error) {
	if len(references) == 0 {
		panic("hyp.client.Reply: no references")
	}
	annot := NewAnnotationTemplate(text, c.Group, references, uri)
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	err = enc.Encode(annot)
	if err != nil {
		panic(fmt.Sprintf("Failed to encode annotation: %v", err))
	}
	req, err := http.NewRequestWithContext(ctxt, "POST", "https://api.hypothes.is/api/annotations", &buf)
	if err != nil {
		panic(fmt.Sprintf("Failed to create http request for create: %v", err))
	}
	req.Header = map[string][]string{
		"Authorization": {"Bearer " + c.Token},
	}
	httpResp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to perform create: %v", err)
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode != 200 {
		return "", fmt.Errorf("failed to perform create: status=%v; want 200", httpResp.StatusCode)
	}
	decoder := json.NewDecoder(httpResp.Body)
	var newAnnot Annotation
	if err := decoder.Decode(&newAnnot); err != nil {
		return "", fmt.Errorf("failed to decode create response: %v", err)
	}
	return newAnnot.ID, nil
}

// The fields required to create an Annotation
func NewAnnotationTemplate(text, group string, references []string, uri string) *Annotation {
	return &Annotation{
		URI:         uri,
		Text:        text,
		Group:       group,
		Permissions: &Permissions{Read: []string{"group:" + group}},
		References:  references,
	}
}
