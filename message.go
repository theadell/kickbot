package main

import (
	"fmt"

	"github.com/slack-go/slack"
)

var joinBtn = slack.ButtonBlockElement{
	Type:     "button",
	Text:     slack.NewTextBlockObject("plain_text", "Bin dabei!", false, false),
	ActionID: ACTION_JOIN_ROUND,
	Value:    ACTION_JOIN_ROUND,
	Style:    "primary",
}

var leaveBtn = slack.ButtonBlockElement{
	Type:     "button",
	Text:     slack.NewTextBlockObject("plain_text", "Bin raus!", false, false),
	ActionID: ACTION_LEAVE_ROUND,
	Value:    ACTION_LEAVE_ROUND,
	Confirm: &slack.ConfirmationBlockObject{
		Title:   slack.NewTextBlockObject("plain_text", "Bist du sicher?", false, false),
		Text:    slack.NewTextBlockObject("plain_text", "Möchtest du wirklich das Spiel verlassen?", false, false),
		Confirm: slack.NewTextBlockObject("plain_text", "Nu", false, false),
		Deny:    slack.NewTextBlockObject("plain_text", "Nä", false, false),
	},
	Style: "danger",
}

var actionBlock = slack.NewActionBlock("ACT_GP_01", joinBtn, leaveBtn)

func NewGameInitiationMsg(playerId string, duel bool) slack.MsgOption {
	var text string

	if duel {
		text = fmt.Sprintf("<!here>, <@%s> sucht einen Herausforderer für ein 1v1 Kicker-Duell. Wer traut sich", playerId)
	} else {
		text = fmt.Sprintf("<!here>, <@%s> hat Bock auf Kicker! Wer macht mit? Noch 3 Leute gesucht!", playerId)
	}
	textBlock := slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", text, false, false), nil, nil)
	return slack.MsgOptionBlocks(textBlock, actionBlock)

}

func NewGameUpdateMsg(playerIds []string, quorum int) slack.MsgOption {
	needed := quorum - len(playerIds)
	var playerMentions string
	for _, id := range playerIds {
		playerMentions += fmt.Sprintf("<@%s> ", id)
	}

	var text string
	var blocks []slack.Block

	if needed > 0 {
		text = fmt.Sprintf("%s sind dabei. Noch %d Spieler gesucht!", playerMentions, needed)
		blocks = []slack.Block{
			slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", text, false, false), nil, nil),
			actionBlock,
		}
	} else {
		text = fmt.Sprintf("%s sind bereit. Los geht's!", playerMentions)
		blocks = []slack.Block{
			slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", text, false, false), nil, nil),
		}
	}

	return slack.MsgOptionBlocks(blocks...)
}
