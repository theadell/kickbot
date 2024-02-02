package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	gomock "go.uber.org/mock/gomock"
)

func TestSlashCommandHandlerWithValidCommands(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSlackClient := NewMockSlackClient(ctrl)

	gameMgr := NewGameManager(mockSlackClient, DEFAULT_GAMEREQ_TIMEOUT)
	defer gameMgr.Shutdown()

	testCases := []struct {
		channelID string
		command   string
	}{
		{
			channelID: "test-channel-1",
			command:   CMD_START_ROUND,
		},
		{
			channelID: "test-channel-2",
			command:   CMD_START_1V1_ROUND,
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("channelID: %s, command: %s", tc.channelID, tc.command), func(t *testing.T) {

			// Expect a PostMessage call for each new game
			mockSlackClient.EXPECT().
				PostMessage(tc.channelID, gomock.Any()).
				Times(1)

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

			if rr.Result().StatusCode != http.StatusOK {
				t.Errorf("Status code returned, %d, did not match expected code %d", rr.Result().StatusCode, http.StatusOK)
			}
		})
	}
}

func TestSlashCommandHandlerWithInvalidCommands(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSlackClient := NewMockSlackClient(ctrl)

	gameMgr := NewGameManager(mockSlackClient, DEFAULT_GAMEREQ_TIMEOUT)
	defer gameMgr.Shutdown()

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

			gameMgr := NewGameManager(mockSlackClient, DEFAULT_GAMEREQ_TIMEOUT)
			defer gameMgr.Shutdown()

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
