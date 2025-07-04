package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/socketmode"
	"github.com/traPtitech/go-traq"
	traqwsbot "github.com/traPtitech/traq-ws-bot"
	"github.com/traPtitech/traq-ws-bot/payload"
)

var bot *traqwsbot.Bot

type Form struct {
	PrivacyData string `json:"privacyData"`
	Content     string `json:"content"`
}

func main() {
	godotenv.Load()
	TARGET_CHANNEL := os.Getenv("TARGET_CHANNEL")
	TRAQ_ACCESS_TOKEN := os.Getenv("TRAQ_BOT_ACCESS_TOKEN")
	bot, err := traqwsbot.NewBot(&traqwsbot.Options{
		AccessToken: TRAQ_ACCESS_TOKEN,
	})
	if err != nil {
		log.Fatal(err)
	}
	SLACK_ACCESS_TOKEN := os.Getenv("SLACK_TOKEN")
	SLACK_APP_TOKEN := os.Getenv("SLACK_WEBSOCKET_TOKEN")
	api := slack.New(
		SLACK_ACCESS_TOKEN,
		slack.OptionAppLevelToken(SLACK_APP_TOKEN),
		slack.OptionDebug(false),
		slack.OptionLog(log.New(os.Stdout, "slack-bot: ", log.Lshortfile|log.LstdFlags)),
	)
	socket := socketmode.New(
		api,
		socketmode.OptionDebug(false),
		socketmode.OptionLog(log.New(os.Stdout, "socket-mode: ", log.Lshortfile|log.LstdFlags)),
	)
	_, authTestErr := api.AuthTest()
	if authTestErr != nil {
		fmt.Fprintf(os.Stderr, "SLACK_BOT_TOKEN is invalid: %v\n", authTestErr)
		os.Exit(1)
	}

	go func() {
		for envelope := range socket.Events {
			// Slackソケットイベント処理
			switch envelope.Type {
			case socketmode.EventTypeInteractive:
				interaction, ok := envelope.Data.(slack.InteractionCallback)
				if !ok {
					continue
				}
				socket.Ack(*envelope.Request)
				if interaction.Type == slack.InteractionTypeMessageAction {
					err = api.OpenDialog(interaction.TriggerID, slack.Dialog{
						TriggerID:   interaction.TriggerID,
						CallbackID:  "dialog",
						Title:       "Dialog",
						SubmitLabel: "Submit",
						Elements: []slack.DialogElement{
							slack.TextInputElement{
								DialogInput: slack.DialogInput{
									Label:       "Text",
									Name:        "text",
									Type:        slack.InputTypeTextArea,
									Placeholder: "Text",
								},
								Value: interaction.Message.Text,
							},
						},
					})
					if err != nil {
						log.Printf("failed opening dialog: %v", err)
					}

				}
				if interaction.Type == slack.InteractionTypeDialogSubmission {
					_, _, err = bot.API().MessageApi.PostMessage(context.Background(), TARGET_CHANNEL).PostMessageRequest(traq.PostMessageRequest{
						Content: fmt.Sprintf("渉外slack[%vチャンネル]から転送です: \n%v", interaction.Channel.GroupConversation.Name, interaction.Submission["text"]),
					}).Execute()
					if err != nil {
						log.Printf("failed posting message: %v", err)
					}
				}
				if interaction.Type == slack.InteractionTypeBlockActions {
					// log.Printf("Block actions: %#v", interaction)
					sectionBlock := interaction.Message.Msg.Blocks.BlockSet[1].(*slack.SectionBlock)
					err = api.OpenDialog(interaction.TriggerID, slack.Dialog{
						TriggerID:   interaction.TriggerID,
						CallbackID:  "dialog",
						Title:       "Dialog",
						SubmitLabel: "Submit",
						Elements: []slack.DialogElement{
							slack.TextInputElement{
								DialogInput: slack.DialogInput{
									Label:       "Text",
									Name:        "text",
									Type:        slack.InputTypeTextArea,
									Placeholder: "Text",
								},
								Value: sectionBlock.Text.Text,
							},
						},
					})
					if err != nil {
						log.Printf("failed opening dialog: %v", err)
					}

				}
			}
		}
	}()

	bot.OnBotMessageStampsUpdated(func(e *payload.BotMessageStampsUpdated) {
		// e.Stamps[i].StampIDにyokunasasouがある場合、traqのメッセージを削除する
		for _, stamp := range e.Stamps {
			if stamp.StampID == "4e7e3747-168a-4249-b485-91fabe390043" {
				_, err = bot.API().MessageApi.DeleteMessage(context.Background(), e.MessageID).Execute()
				if err != nil {
					log.Printf("failed deleting message: %v", err)
				}
				break
			}
		}
	})

	go func() {
		if err := bot.Start(); err != nil {
			log.Fatal(err)
		}
	}()

	go func() {
		socket.Run()
	}()

	e := echo.New()

	e.GET("/ping", func(c echo.Context) error {
		return c.String(http.StatusOK, "pong")
	})

	e.POST("/inbox", func(c echo.Context) error {
		header := c.Request().Header
		token := header.Get("X-Form-Token")
		FORM_TOKEN := os.Getenv("FORM_TOKEN")
		if token != FORM_TOKEN {
			return c.String(http.StatusUnauthorized, "unauthorized")
		}
		var form Form
		if err := json.NewDecoder(c.Request().Body).Decode(&form); err != nil {
			return c.String(http.StatusBadRequest, "invalid request body")
		}

		_, _, err = api.PostMessage("C07T00ZK4KD", slack.MsgOptionBlocks(
			slack.NewSectionBlock(
				slack.NewTextBlockObject("mrkdwn", string(form.PrivacyData), false, true),
				nil,
				nil,
			),
			slack.NewSectionBlock(
				slack.NewTextBlockObject("mrkdwn", string(form.Content), false, true),
				nil,
				nil,
			),
			slack.NewActionBlock("button",
				slack.NewButtonBlockElement("test", "Click Me", slack.NewTextBlockObject("plain_text", "traQへ転送", false, false)),
			),
		))
		if err != nil {
			log.Printf("failed posting message: %v", err)
		}
		return c.String(http.StatusOK, "ok")
	})

	e.POST("/accounts", func(c echo.Context) error {
		header := c.Request().Header
		token := header.Get("X-Form-Token")
		FORM_TOKEN := os.Getenv("FORM_TOKEN")
		if token != FORM_TOKEN {
			return c.String(http.StatusUnauthorized, "unauthorized")
		}
		var form Form
		if err := json.NewDecoder(c.Request().Body).Decode(&form); err != nil {
			return c.String(http.StatusBadRequest, "invalid request body")
		}

		_, _, err = api.PostMessage("C07T2KD5MJQ", slack.MsgOptionBlocks(
			slack.NewSectionBlock(
				slack.NewTextBlockObject("mrkdwn", string(form.PrivacyData), false, true),
				nil,
				nil,
			),
			slack.NewSectionBlock(
				slack.NewTextBlockObject("mrkdwn", string(form.Content), false, true),
				nil,
				nil,
			),
			slack.NewActionBlock("button",
				slack.NewButtonBlockElement("test", "Click Me", slack.NewTextBlockObject("plain_text", "traQへ転送", false, false)),
			),
		))
		if err != nil {
			log.Printf("failed posting message: %v", err)
		}
		return c.String(http.StatusOK, "ok")
	})

	e.Logger.Fatal(e.Start(":8080"))
}
