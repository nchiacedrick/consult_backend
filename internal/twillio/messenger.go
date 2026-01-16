// Package twillio provides helpers for sending SMS messages via the Twilio API.
package twillio

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/twilio/twilio-go"
	twilioApi "github.com/twilio/twilio-go/rest/api/v2010"
)

type TwillioConfig struct {
	AccountSID  string
	AuthToken   string
	PhoneNumber string
}

type TwillioSendMessagePayload struct {
	To      string
	Message string
}

type TwillioMessageResponse struct {
	Sid         string `json:"sid"`
	DateCreated string `json:"date_created"`
}

func SendMessage(payload TwillioSendMessagePayload) (*TwillioMessageResponse, error) {
	if err := godotenv.Load(); err != nil {
		return nil, err
	}

	AccountSID := os.Getenv("TWILIO_ACCOUNT_SID")
	AuthToken := os.Getenv("TWILIO_AUTH_TOKEN")
	PhoneNumber := os.Getenv("TWILIO_PHONE_NUMBER")

	client := twilio.NewRestClientWithParams(twilio.ClientParams{
		Username: AccountSID,
		Password: AuthToken,
	})

	params := &twilioApi.CreateMessageParams{}
	params.SetTo(payload.To)
	params.SetFrom(PhoneNumber)
	params.SetBody(payload.Message)

	resp, err := client.Api.CreateMessage(params)
	if err != nil {
		fmt.Println("Error sending message via Twillio:", err.Error())	
		return nil, err
	} else {
		reponse, _ := json.Marshal(resp)
		fmt.Println("Twillio message response:", string(reponse))
		return &TwillioMessageResponse{
			Sid:         *resp.Sid,
			DateCreated: *resp.DateCreated,
		}, nil
	}
}