package slack

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/TheKrainBow/gotification/slackmsg"
)

type recordingHTTPClient struct {
	reqs   []*http.Request
	bodies []string
	resps  []string
}

func (c *recordingHTTPClient) Do(req *http.Request) (*http.Response, error) {
	c.reqs = append(c.reqs, req)
	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	c.bodies = append(c.bodies, string(bodyBytes))
	respBody := `{"ok":true}`
	if len(c.resps) > 0 {
		respBody = c.resps[0]
		c.resps = c.resps[1:]
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(respBody)),
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

func TestSendToChannelMessageIncludesBlocks(t *testing.T) {
	client := &recordingHTTPClient{}
	p, err := NewProvider(Config{BotToken: "xoxb-test", HTTPClient: client})
	if err != nil {
		t.Fatalf("NewProvider failed: %v", err)
	}

	err = p.SendToChannelMessage(context.Background(), "C123", slackmsg.Message{
		Text: "Un dossier est en attente de validation",
		Attachments: []slackmsg.Attachment{{
			Color: "#ffcc00",
			Blocks: []slackmsg.Block{
				{
					"type": "header",
					"text": map[string]any{
						"type": "plain_text",
						"text": "📁 Un dossier est en attente de validation",
					},
				},
				{
					"type": "actions",
					"elements": []any{
						map[string]any{
							"type": "button",
							"text": map[string]any{
								"type": "plain_text",
								"text": "Voir le dossier",
							},
							"url":   "https://adm.example.com/admin/dossiers/42",
							"style": "primary",
						},
					},
				},
			},
		}},
	})
	if err != nil {
		t.Fatalf("SendToChannelMessage failed: %v", err)
	}

	if !strings.Contains(client.bodies[0], `"blocks":[`) {
		t.Fatalf("request body is missing blocks: %s", client.bodies[0])
	}
	if !strings.Contains(client.bodies[0], `"type":"header"`) {
		t.Fatalf("request body is missing header block: %s", client.bodies[0])
	}
	if !strings.Contains(client.bodies[0], `"Voir le dossier"`) {
		t.Fatalf("request body is missing button block: %s", client.bodies[0])
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

func TestSendToChannelRawMessageInjectsChannel(t *testing.T) {
	client := &recordingHTTPClient{}
	p, err := NewProvider(Config{BotToken: "xoxb-test", HTTPClient: client})
	if err != nil {
		t.Fatalf("NewProvider failed: %v", err)
	}

	err = p.SendToChannelRawMessage(context.Background(), "C123", json.RawMessage(`{"text":"hello"}`))
	if err != nil {
		t.Fatalf("SendToChannelRawMessage failed: %v", err)
	}

	if !strings.Contains(client.bodies[0], `"channel":"C123"`) {
		t.Fatalf("request body is missing injected channel: %s", client.bodies[0])
	}
	if !strings.Contains(client.bodies[0], `"text":"hello"`) {
		t.Fatalf("request body is missing original text: %s", client.bodies[0])
	}
}

func TestSendToChannelRawMessageRejectsConflictingChannel(t *testing.T) {
	client := &recordingHTTPClient{}
	p, err := NewProvider(Config{BotToken: "xoxb-test", HTTPClient: client})
	if err != nil {
		t.Fatalf("NewProvider failed: %v", err)
	}

	err = p.SendToChannelRawMessage(context.Background(), "C123", json.RawMessage(`{"channel":"C999","text":"hello"}`))
	if err == nil {
		t.Fatal("expected conflicting channel error")
	}
	if !strings.Contains(err.Error(), "different channel") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSendToUserRawMessageInjectsResolvedDMChannel(t *testing.T) {
	client := &recordingHTTPClient{resps: []string{
		`{"ok":true,"channel":{"id":"D123"}}`,
		`{"ok":true}`,
	}}
	p, err := NewProvider(Config{BotToken: "xoxb-test", HTTPClient: client})
	if err != nil {
		t.Fatalf("NewProvider failed: %v", err)
	}

	err = p.SendToUserRawMessage(context.Background(), "U123", json.RawMessage(`{"text":"hello"}`))
	if err != nil {
		t.Fatalf("SendToUserRawMessage failed: %v", err)
	}

	if got, want := client.reqs[0].URL.Path, "/api/conversations.open"; got != want {
		t.Fatalf("unexpected openConversation path: got %q want %q", got, want)
	}
	if got, want := client.reqs[1].URL.Path, "/api/chat.postMessage"; got != want {
		t.Fatalf("unexpected postMessage path: got %q want %q", got, want)
	}
	if !strings.Contains(client.bodies[1], `"channel":"D123"`) {
		t.Fatalf("request body is missing resolved dm channel: %s", client.bodies[1])
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
