package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
	"github.com/traPtitech/go-traq"
	traqwsbot "github.com/traPtitech/traq-ws-bot"
)

var bot *traqwsbot.Bot

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("This is production environment. Please set environment variables.")
	}
	TARGET_CHANNEL := os.Getenv("TARGET_CHANNEL")
	TRAQ_ACCESS_TOKEN := os.Getenv("TRAQ_BOT_ACCESS_TOKEN")
	bot, err = traqwsbot.NewBot(&traqwsbot.Options{
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
			switch envelope.Type {
			case socketmode.EventTypeEventsAPI:
				socket.Ack(*envelope.Request)
				eventPayload, _ := envelope.Data.(slackevents.EventsAPIEvent)
				if eventPayload.Type == slackevents.CallbackEvent {
					switch ev := eventPayload.InnerEvent.Data.(type) {
					case *slackevents.ReactionAddedEvent:
						// リアクションが追加されたとき
						if ev.Reaction == "traq" {
							_, err := api.GetConversationHistory(&slack.GetConversationHistoryParameters{
								ChannelID: ev.Item.Channel,
								Limit:     1,
								Latest:    ev.Item.Timestamp,
								Inclusive: true,
							})
							if err != nil {
								log.Printf("failed getting conversation history: %v", err)
							}
							_, _, err = api.PostMessage(ev.Item.Channel, slack.MsgOptionBlocks(
								slack.NewSectionBlock(
									slack.NewTextBlockObject("plain_text", "Hello, world!", false, false),
									nil,
									nil,
								),
								slack.NewActionBlock("button",
									slack.NewButtonBlockElement(ev.Item.Timestamp, "Click Me", slack.NewTextBlockObject("plain_text", "Click Me", false, false)),
								),
							))
							if err != nil {
								log.Printf("failed posting message: %v", err)
							}
						}
					}
				}
			case socketmode.EventTypeInteractive:
				interaction, ok := envelope.Data.(slack.InteractionCallback)
				if !ok {
					continue
				}
				socket.Ack(*envelope.Request)
				if interaction.Type == slack.InteractionTypeMessageAction {
					sectionBlock := interaction.Message.Msg.Blocks.BlockSet[0].(*slack.SectionBlock)
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
				if interaction.Type == slack.InteractionTypeDialogSubmission {
					_, _, err = bot.API().MessageApi.PostMessage(context.Background(), TARGET_CHANNEL).PostMessageRequest(traq.PostMessageRequest{
						Content: fmt.Sprintf("渉外slackから転送です: \n%v", interaction.Submission["text"]),
					}).Execute()
					if err != nil {
						log.Printf("failed posting message: %v", err)
					}
				}
				if interaction.Type == slack.InteractionTypeBlockActions {
					log.Printf("Block actions: %#v", interaction)
					sectionBlock := interaction.Message.Msg.Blocks.BlockSet[0].(*slack.SectionBlock)
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
	socket.Run()

	if err := bot.Start(); err != nil {
		log.Fatal(err)
	}

}
