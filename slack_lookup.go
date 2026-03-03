package gotification

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// FindSlackUsersByName resolves Slack user IDs matching query for one provider.
//
// Provider selection follows the same rules as Send:
// if provider is empty, the default Slack provider is used.
func (d *Dispatcher) FindSlackUsersByName(ctx context.Context, provider string, query string) ([]string, error) {
	providerKey, p, err := d.slackProviderFor(provider)
	if err != nil {
		return nil, &NotifyError{Kind: ErrInvalidInput, Channel: ChannelSlack, Provider: provider, Cause: err}
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, &NotifyError{Kind: ErrInvalidInput, Channel: ChannelSlack, Provider: providerKey, Cause: errors.New("query is required")}
	}

	lookup, ok := p.(SlackUserLookupProvider)
	if !ok {
		return nil, &NotifyError{Kind: ErrInvalidInput, Channel: ChannelSlack, Provider: providerKey, Cause: fmt.Errorf("slack provider %q does not support user lookup", providerKey)}
	}

	ids, err := lookup.FindUsersByName(ctx, query)
	if err != nil {
		dest := Destination{Channel: ChannelSlack, Kind: DestinationSlackUser, ID: query, Provider: providerKey}
		return nil, wrapProviderError(err, ChannelSlack, providerKey, dest)
	}
	return ids, nil
}
