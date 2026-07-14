package sender

import (
	"fmt"
	"log"
	"strings"
)

type EmailProvider interface {
	Send(to string, subject string, body string) error
}

type LogProvider struct{}

func NewLogProvider() *LogProvider {
	return &LogProvider{}
}

func (p *LogProvider) Send(to string, subject string, body string) error {
	log.Printf("[EMAIL] To: %s | Subject: %s", to, subject)
	log.Printf("[EMAIL] Body:\n%s", body)
	log.Printf("[EMAIL] --- End of email ---")
	return nil
}

func SendConfirmationEmail(provider EmailProvider, recipientEmail, bookingRef, eventName, eventDate, venue string, quantity int) error {
	subject := fmt.Sprintf("Your Ticket Confirmation - %s", eventName)
	body := buildConfirmationBody(bookingRef, eventName, eventDate, venue, quantity)
	return provider.Send(recipientEmail, subject, body)
}

func buildConfirmationBody(bookingRef, eventName, eventDate, venue string, quantity int) string {
	lines := []string{
		fmt.Sprintf("Dear Customer,"),
		"",
		fmt.Sprintf("Your ticket purchase for \"%s\" has been confirmed!", eventName),
		"",
		fmt.Sprintf("Booking Reference: %s", bookingRef),
		fmt.Sprintf("Event: %s", eventName),
		fmt.Sprintf("Date: %s", eventDate),
		fmt.Sprintf("Venue: %s", venue),
		fmt.Sprintf("Quantity: %d", quantity),
		"",
		"Thank you for your purchase. Please present this booking reference at the venue.",
		"",
		"- Event Ticket Platform",
	}
	return strings.Join(lines, "\n")
}
