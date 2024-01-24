package main

import (
	"log"
	"os"

	"github.com/slack-go/slack"
)

func main() {
	token := os.Getenv("KICKBOT_TOKEN")
	channelID := os.Getenv("KICKBOT_CHANNELID")

	api := slack.New(token)
	attachment := slack.Attachment{
		Text:       "New foosball game! Who's in?",
		CallbackID: "game_join",
		Color:      "#4af030",
		Actions: []slack.AttachmentAction{
			{
				Name:  "join",
				Text:  "Join",
				Type:  "button",
				Value: "join",
			},
		},
	}
	message := slack.MsgOptionAttachments(attachment)
	channel, timestamp, err := api.PostMessage(channelID, message)
	if err != nil {
		log.Fatalf("Failed to post message: %s", err)
	}

	log.Printf("Message successfully sent to channel %s at %s", channel, timestamp)

}
