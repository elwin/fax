package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/beefsack/go-rate"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/knq/escpos"
	"github.com/urfave/cli/v2"
)

func main() {
	var (
		telegramToken string
		devicePath    string
		app           = &cli.App{
			Name:  "printer",
			Usage: "Start printer server",
			Action: func(c *cli.Context) error {
				return run(telegramToken, devicePath)
			},
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:        "telegram_token",
					Required:    true,
					Destination: &telegramToken,
				},
				&cli.StringFlag{
					Name:        "device_path",
					Required:    true,
					Destination: &devicePath,
				},
			},
		}
	)

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

type printer struct {
	path string
}

func newPrinter(devicePath string) *printer {
	return &printer{path: devicePath}
}

func (p *printer) print(printF func(e *escpos.Escpos) error) error {
	f, err := os.OpenFile("/dev/usb/lp0", os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer f.Close()

	return printF(escpos.New(f))
}

func run(telegramToken, devicePath string) error {
	printer := newPrinter(devicePath)

	// Just a quick connection check
	if err := printer.print(func(e *escpos.Escpos) error {
		return nil
	}); err != nil {
		return err
	}

	bot, err := tgbotapi.NewBotAPI(telegramToken)
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = true

	log.Printf("Authorized on account %s\n", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 30

	userRateLimit := map[int64]*rate.RateLimiter{}
	globalRateLimit := rate.New(100, time.Hour)

	for update := range bot.GetUpdatesChan(u) {
		if update.Message == nil {
			continue
		}

		log.Printf("[%s] %s", update.Message.From.UserName, update.Message.Text)

		if _, ok := userRateLimit[update.Message.From.ID]; !ok {
			userRateLimit[update.Message.From.ID] = rate.New(5, time.Hour)
		}

		var msg string
		if len(update.Message.Text) > 1000 {
			msg = "Sorry, your message is a bit too long. Please try to limit yourself a bit."
		} else if ok, _ := userRateLimit[update.Message.From.ID].Try(); !ok {
			msg = fmt.Sprintf(
				"Sorry, you've been sending a bit too many messages lately. " +
					"Please wait a bit before sending the next one.",
			)
		} else if ok, _ := globalRateLimit.Try(); !ok {
			msg = fmt.Sprintf(
				"Sorry, there have been too many messages lately. " +
					"Please wait a bit before sending the next one.",
			)
		} else if err := printer.print(func(e *escpos.Escpos) error {
			e.Init()
			e.FormfeedN(2)
			e.Write(fmt.Sprintf("Message from: %s\n", update.Message.From.UserName))
			e.Write(fmt.Sprintf("Received at: %s\n", update.Message.Time().Format("2. January 2006, 15:04")))
			e.FormfeedN(2)
			e.SetAlign("center")
			e.Write(update.Message.Text)
			e.FormfeedN(3)
			e.Cut()
			e.End()

			return nil
		}); err != nil {
			log.Println(err.Error())
			msg = "Huh, something went wrong. Try pinging Elwin, maybe he knows what's up?"
		} else {
			msg = "Cool, message has been printed!"
		}

		reply := tgbotapi.NewMessage(update.Message.Chat.ID, msg)
		if _, err := bot.Send(reply); err != nil {
			log.Println(err.Error())
		}
	}

	return nil
}
