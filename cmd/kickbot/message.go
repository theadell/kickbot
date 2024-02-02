package main

import (
	"fmt"
	"github.com/slack-go/slack"
	"strings"
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

var actionBlock = slack.NewActionBlock("GAME_ACTIONS", joinBtn, leaveBtn)

func NewGameRequestMsg(playerId string, gameType GameType) slack.MsgOption {
	var text string

	if gameType == GameTypeOneVsOne {
		text = fmt.Sprintf("<!here>, <@%s> sucht einen Herausforderer für ein 1v1 Kicker-Duell. Wer traut sich", playerId)
	} else {
		text = fmt.Sprintf("<!here>, <@%s> hat Bock auf Kicker! Wer macht mit? Noch 3 Leute gesucht!", playerId)
	}
	textBlock := slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", text, false, false), nil, nil)
	return slack.MsgOptionBlocks(textBlock, slack.NewDividerBlock(), actionBlock)

}

func GameRequestUpdateMsg(playerIds []string, quorum int) slack.MsgOption {
	needed := quorum - len(playerIds)
	playerMentions := make([]string, len(playerIds))

	for i, id := range playerIds {
		playerMentions[i] = fmt.Sprintf("<@%s>", id)
	}

	playerMentionText := strings.Join(playerMentions, " ")
	var text string
	var blocks []slack.Block

	if needed > 0 {
		text = fmt.Sprintf("%s sind dabei. Noch %d Spieler gesucht!", playerMentionText, needed)
		blocks = []slack.Block{
			slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", text, false, false), nil, nil),
			actionBlock,
		}
	} else {
		text = fmt.Sprintf("%s sind bereit. Los geht's!", playerMentionText)
		blocks = []slack.Block{
			slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", text, false, false), nil, nil),
		}
	}

	return slack.MsgOptionBlocks(blocks...)
}
