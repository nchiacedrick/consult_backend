package mailer

import (
	"bytes"
	"fmt"
	"html/template"
	"os"

	"github.com/resend/resend-go/v2"
)

func NewResend(recipient, templateFile string, data interface{}) error {
	apiKey := os.Getenv("RESEND_API_KEY")
	client := resend.NewClient(apiKey)

	tmp, err := template.New("email").ParseFS(templateFS, "templates/"+templateFile)
	if err != nil {
		return err
	}

	subject := new(bytes.Buffer)
	if err := tmp.ExecuteTemplate(subject, "subject", data); err != nil {
		return err
	}

	plainBody := new(bytes.Buffer)
	if err := tmp.ExecuteTemplate(plainBody, "plainBody", data); err != nil {
		return err
	}

	// And likewise with the "htmlBody" template.
	htmlBody := new(bytes.Buffer)
	err = tmp.ExecuteTemplate(htmlBody, "htmlBody", data)
	if err != nil {
		return err
	}

	params := &resend.SendEmailRequest{
		From:    "Consult-Out <onboarding@consult-out.com>",
		To:      []string{recipient},
		Subject: "Welcome to Consult-Out!",
		Html:    htmlBody.String(),
		Text:    plainBody.String(),
		ReplyTo: "cedrickewi@gmail.com",
	}

	sent, err := client.Emails.Send(params)
	if err != nil {
		fmt.Println(err.Error())
		return err
	}
	fmt.Printf("Email sent! ID: %s\n", sent.Id)

	return nil
}
