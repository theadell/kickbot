package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/slack-go/slack"
	gomock "go.uber.org/mock/gomock"
)

func TestSlashCommandHandlerWithValidCommands(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSlackClient := NewMockSlackClient(ctrl)

	gameMgr := NewGameManager(mockSlackClient)

	testCases := []struct {
		channelID      string
		command        string
		expectedCalls  func()
		expectedStatus int
	}{
		{
			channelID: "test-channel-regular",
			command:   CMD_START_ROUND,
			expectedCalls: func() {
				mockSlackClient.EXPECT().
					PostMessage("test-channel-regular", gomock.Any()).
					Times(1)
			},
			expectedStatus: http.StatusOK,
		},
		{
			channelID: "test-channel-duel",
			command:   CMD_START_ROUND,
			expectedCalls: func() {
				mockSlackClient.EXPECT().
					PostMessage("test-channel-duel", gomock.Any()).
					Times(1)
			},
			expectedStatus: http.StatusOK,
		},
		{
			channelID: "test-channel-cancel",
			command:   CMD_CANCEL_ROUND,
			expectedCalls: func() {
				mockSlackClient.EXPECT().
					PostEphemeral("test-channel-cancel", "test-user", gomock.Any()).
					Times(1)
			},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("channelID: %s, command: %s", tc.channelID, tc.command), func(t *testing.T) {

			tc.expectedCalls()

			var parameters string
			switch tc.channelID {
			case "test-channel-regular":
				parameters = ""
			case "test-channel-duel":
				parameters = "-d"
			}

			formData := url.Values{
				"token":           {"mock-token"},
				"team_id":         {"test-team-id"},
				"team_domain":     {"team-domain"},
				"enterprise_id":   {"enterpriseID"},
				"enterprise_name": {"enterprise-name"},
				"channel_id":      {tc.channelID},
				"channel_name":    {"test-channel"},
				"user_id":         {"test-user"},
				"user_name":       {"test"},
				"command":         {tc.command},
				"text":            {parameters},
				"response_url":    {""},
				"trigger_id":      {""},
				"api_app_id":      {"app-id"},
			}

			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/commands", strings.NewReader(formData.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			handleSlackCommand(gameMgr)(rr, req)

			if rr.Result().StatusCode != tc.expectedStatus {
				t.Errorf("Status code returned, %d, did not match expected code %d", rr.Result().StatusCode, http.StatusOK)
			}
		})
	}
}

func TestSlashCommandHandlerWithInvalidCommands(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSlackClient := NewMockSlackClient(ctrl)

	testCases := []struct {
		channelID string
		command   string
	}{
		{
			channelID: "test-channel-1",
			command:   "test",
		},
		{
			channelID: "test-channel-2",
			command:   "play",
		},
		{
			channelID: "test-channel-3",
			command:   "meet",
		},
		{
			channelID: "test-channel-4",
			command:   "ping",
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("channelID: %s, command: %s", tc.channelID, tc.command), func(t *testing.T) {

			// Expect No PostMessage call
			mockSlackClient.EXPECT().
				PostMessage(tc.channelID, gomock.Any()).
				Times(0)

			gameMgr := NewGameManager(mockSlackClient)

			formData := url.Values{
				"token":           {"mock-token"},
				"team_id":         {"test-team-id"},
				"team_domain":     {"team-domain"},
				"enterprise_id":   {"enterpriseID"},
				"enterprise_name": {"enterprise-name"},
				"channel_id":      {tc.channelID},
				"channel_name":    {"test-channel"},
				"user_id":         {"test-user"},
				"user_name":       {"test"},
				"command":         {tc.command},
				"text":            {"test-text"},
				"response_url":    {""},
				"trigger_id":      {""},
				"api_app_id":      {"app-id"},
			}

			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/commands", strings.NewReader(formData.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			handleSlackCommand(gameMgr)(rr, req)

			if rr.Result().StatusCode != http.StatusBadRequest {
				t.Errorf("Status code returned, %d, did not match expected code %d", rr.Result().StatusCode, http.StatusBadRequest)
			}
		})
	}
}

func TestSlackInteractionCallbackHandler(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSlackClient := NewMockSlackClient(ctrl)

	gameMgr := NewGameManager(mockSlackClient)

	channelID := "test-channel"
	channel := SlackChannel(channelID)
	player := "test-p2"

	// Expect a post for the game creation
	mockSlackClient.EXPECT().
		PostMessage(channelID, gomock.Any()).
		Times(1)

	// Expect 2 UpdateMessage for 1 join and 1 leave
	mockSlackClient.EXPECT().
		UpdateMessage(channelID, gomock.Any(), gomock.Any()).
		Return("channelID", "ts", "text", nil).MaxTimes(2)

	var gameOptions = GameOpts{
		timeout:  time.Minute * 30,
		gameType: GameTypeTwoVsTwo,
	}
	gameMgr.CreateGame(channel, "unique-test-p1", gameOptions)

	tests := []struct {
		name           string
		actionID       string
		expectHTTPCode int
		hasRequestBody bool
		hasActions     bool
	}{
		{
			name:           "Valid Join Round Action",
			actionID:       ACTION_JOIN_ROUND,
			expectHTTPCode: http.StatusOK,
			hasRequestBody: true,
			hasActions:     true,
		},
		{
			name:           "Valid Leave Round Action",
			actionID:       ACTION_LEAVE_ROUND,
			expectHTTPCode: http.StatusOK,
			hasRequestBody: true,
			hasActions:     true,
		},
		{
			name:           "Invalid Action 1",
			actionID:       "INVALID_ACTION_1",
			expectHTTPCode: http.StatusBadRequest,
			hasRequestBody: true,
			hasActions:     true,
		},
		{
			name:           "Invalid Action 2",
			actionID:       "INVALID_ACTION_2",
			expectHTTPCode: http.StatusBadRequest,
			hasRequestBody: true,
			hasActions:     true,
		},
		{
			name:           "No Request Body",
			actionID:       "",
			expectHTTPCode: http.StatusBadRequest,
			hasRequestBody: false,
			hasActions:     false,
		},
		{
			name:           "Empty Block Actions",
			actionID:       "",
			expectHTTPCode: http.StatusBadRequest,
			hasRequestBody: true,
			hasActions:     false,
		},
	}

	// Loop through each test case
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Set up the request body if necessary
			var requestBody = strings.NewReader("")
			if tc.hasRequestBody {
				callback := slack.InteractionCallback{}
				callback.Channel.ID = channelID
				callback.User.ID = player
				if tc.hasActions {
					callback.ActionCallback.BlockActions = []*slack.BlockAction{{ActionID: tc.actionID}}
				}
				payload, _ := json.Marshal(callback)
				form := url.Values{}
				form.Set("payload", string(payload))
				requestBody = strings.NewReader(form.Encode())
			}
			req := httptest.NewRequest(http.MethodPost, "/events", requestBody)
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			rr := httptest.NewRecorder()

			handleSlackEvent(gameMgr)(rr, req)

			if rr.Result().StatusCode != tc.expectHTTPCode {
				t.Errorf("Status code returned, %d, did not match expected code %d", rr.Result().StatusCode, tc.expectHTTPCode)
			}
		})
	}

}

func TestParsingGameOptionFlags(t *testing.T) {

	tests := []struct {
		name        string
		inputParams string
		gameType    GameType
		timeout     time.Duration
	}{
		{
			name:        "no params regular game 30 minute timeout",
			inputParams: "",
			gameType:    GameTypeTwoVsTwo,
			timeout:     time.Minute * 30,
		},
		{
			name:        "Regular 60 minute timeout",
			inputParams: "-timeout 60m",
			gameType:    GameTypeTwoVsTwo,
			timeout:     time.Minute * 60,
		},
		{
			name:        "Duel 60 minute timeout",
			inputParams: "-duel -t 60m",
			gameType:    GameTypeOneVsOne,
			timeout:     time.Minute * 60,
		},
		{
			name:        "Duel 30 minute timeout",
			inputParams: "-d",
			gameType:    GameTypeOneVsOne,
			timeout:     time.Minute * 30,
		},
	}

	// Loop through each test case
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var gameOptions = parseFlags(tc.inputParams)

			if gameOptions.gameType != tc.gameType {
				t.Errorf("Parameters' and GameOptions' GameType doesn't match")
			}
			if gameOptions.timeout != tc.timeout {
				t.Errorf("Parameters' and GameOptions' Timeout doesn't match")
			}
		})
	}
}
