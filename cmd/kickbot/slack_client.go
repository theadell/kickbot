package main

import (
	"context"

	"github.com/slack-go/slack"
)

// SlackClient is an interface representing a subset of operations from the slack.Client.
// It's designed to abstract Slack operations for easier testing.
// If more functionality is needed, simply add the required methods from the slack.Client to this interface.
// Refer to the slack-go package for detailed documentation: https://pkg.go.dev/github.com/slack-go/slack#Client
type SlackClient interface {

	// PostEphemeral sends a temporary message visible only to a specific user in a channel.
	// Returns a timestamp of the posted message or an error.
	PostEphemeral(channelID, userID string, options ...slack.MsgOption) (string, error)

	// PostMessage sends a message to a Slack channel.
	// Returns the channel ID and timestamp of the posted message, or an error.
	PostMessage(channelID string, options ...slack.MsgOption) (string, string, error)

	// ScheduleMessage schedules a message to be sent to a Slack channel at a specified time.
	// Returns the channel ID and scheduled message's timestamp, or an error.
	ScheduleMessage(channelID, postAt string, options ...slack.MsgOption) (string, string, error)

	// UpdateMessage updates an existing message in a Slack channel.
	// Returns the channel ID, the message timestamp, and the text of the updated message, or an error if the update fails.
	UpdateMessage(channelID, timestamp string, options ...slack.MsgOption) (string, string, string, error)

	// DeleteMessage removes a message from a Slack channel.
	// Returns the channel and timestamp of the deleted message or an error.
	DeleteMessage(channel, messageTimestamp string) (string, string, error)

	// DeleteMessage removes a message from a Slack channel with a custom context.
	// Returns the channel and timestamp of the deleted message or an error.
	DeleteMessageContext(ctx context.Context, channel, messageTimestamp string) (string, string, error)
}

// compile-time assertion to ensure that `slack.Client` implements `SlackClient`
var _ SlackClient = (*slack.Client)(nil)
