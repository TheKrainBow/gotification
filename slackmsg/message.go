package slackmsg

// Block is one arbitrary Slack Block Kit JSON object.
type Block map[string]any

// Message is a Slack chat.postMessage payload subset supported by gotification.
type Message struct {
	Text           string       `json:"text,omitempty"`
	Blocks         []Block      `json:"blocks,omitempty"`
	ThreadTS       string       `json:"thread_ts,omitempty"`
	ReplyBroadcast bool         `json:"reply_broadcast,omitempty"`
	Attachments    []Attachment `json:"attachments,omitempty"`
}

// Attachment describes one Slack attachment payload.
type Attachment struct {
	Color      string            `json:"color,omitempty"`
	Pretext    string            `json:"pretext,omitempty"`
	AuthorName string            `json:"author_name,omitempty"`
	AuthorLink string            `json:"author_link,omitempty"`
	AuthorIcon string            `json:"author_icon,omitempty"`
	Title      string            `json:"title,omitempty"`
	TitleLink  string            `json:"title_link,omitempty"`
	Text       string            `json:"text,omitempty"`
	Fields     []AttachmentField `json:"fields,omitempty"`
	Footer     string            `json:"footer,omitempty"`
	FooterIcon string            `json:"footer_icon,omitempty"`
	Timestamp  int64             `json:"ts,omitempty"`
	MarkdownIn []string          `json:"mrkdwn_in,omitempty"`
	Blocks     []Block           `json:"blocks,omitempty"`
}

// AttachmentField is one name/value pair rendered inside a Slack attachment.
type AttachmentField struct {
	Title string `json:"title,omitempty"`
	Value string `json:"value,omitempty"`
	Short bool   `json:"short,omitempty"`
}
