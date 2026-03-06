package slack

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/TheKrainBow/gotification/slackmsg"
)

type recordingHTTPClient struct {
	reqs   []*http.Request
	bodies []string
}

func (c *recordingHTTPClient) Do(req *http.Request) (*http.Response, error) {
	c.reqs = append(c.reqs, req)
	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	c.bodies = append(c.bodies, string(bodyBytes))
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
		Header:     make(http.Header),
	}, nil
}

func TestSendToChannelMessageIncludesAttachments(t *testing.T) {
	client := &recordingHTTPClient{}
	p, err := NewProvider(Config{BotToken: "xoxb-test", HTTPClient: client})
	if err != nil {
		t.Fatalf("NewProvider failed: %v", err)
	}

	err = p.SendToChannelMessage(context.Background(), "C123", slackmsg.Message{
		Text: "USB receiver unplugged",
		Attachments: []slackmsg.Attachment{{
			Color: "#7B2CBF",
			Fields: []slackmsg.AttachmentField{
				{Title: "Host", Value: "maagosti", Short: true},
			},
		}},
	})
	if err != nil {
		t.Fatalf("SendToChannelMessage failed: %v", err)
	}

	if got, want := client.reqs[0].URL.Path, "/api/chat.postMessage"; got != want {
		t.Fatalf("unexpected path: got %q want %q", got, want)
	}
	if !strings.Contains(client.bodies[0], `"attachments":[`) {
		t.Fatalf("request body is missing attachments: %s", client.bodies[0])
	}
	if !strings.Contains(client.bodies[0], `"color":"#7B2CBF"`) {
		t.Fatalf("request body is missing attachment color: %s", client.bodies[0])
	}
	if !strings.Contains(client.bodies[0], `"title":"Host"`) {
		t.Fatalf("request body is missing attachment fields: %s", client.bodies[0])
	}
}

func TestSendToChannelMessageIncludesThreadFields(t *testing.T) {
	client := &recordingHTTPClient{}
	p, err := NewProvider(Config{BotToken: "xoxb-test", HTTPClient: client})
	if err != nil {
		t.Fatalf("NewProvider failed: %v", err)
	}

	err = p.SendToChannelMessage(context.Background(), "C123", slackmsg.Message{
		Text:           "thread reply",
		ThreadTS:       "1741256640.123456",
		ReplyBroadcast: true,
	})
	if err != nil {
		t.Fatalf("SendToChannelMessage failed: %v", err)
	}

	if !strings.Contains(client.bodies[0], `"thread_ts":"1741256640.123456"`) {
		t.Fatalf("request body is missing thread timestamp: %s", client.bodies[0])
	}
	if !strings.Contains(client.bodies[0], `"reply_broadcast":true`) {
		t.Fatalf("request body is missing reply_broadcast: %s", client.bodies[0])
	}
}

func TestAddReactionUsesReactionsAdd(t *testing.T) {
	client := &recordingHTTPClient{}
	p, err := NewProvider(Config{BotToken: "xoxb-test", HTTPClient: client})
	if err != nil {
		t.Fatalf("NewProvider failed: %v", err)
	}

	err = p.AddReaction(context.Background(), "C123", "1741256640.123456", ":eyes:")
	if err != nil {
		t.Fatalf("AddReaction failed: %v", err)
	}

	if got, want := client.reqs[0].URL.Path, "/api/reactions.add"; got != want {
		t.Fatalf("unexpected path: got %q want %q", got, want)
	}
	if !strings.Contains(client.bodies[0], `"timestamp":"1741256640.123456"`) {
		t.Fatalf("request body is missing timestamp: %s", client.bodies[0])
	}
	if !strings.Contains(client.bodies[0], `"name":"eyes"`) {
		t.Fatalf("request body is missing normalized emoji name: %s", client.bodies[0])
	}
}
