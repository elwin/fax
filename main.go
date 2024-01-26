package main

import (
	"context"
	"crypto/subtle"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/beefsack/go-rate"
	syncMap "github.com/elwin/x/sync"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/knq/escpos"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/urfave/cli/v2"
)

var authorizedUsers = syncMap.NewMap[string, string]()

func main() {
	var (
		telegramToken string
		devicePath    string
		app           = &cli.App{
			Name:  "printer",
			Usage: "Start printer server",
			Action: func(c *cli.Context) error {
				return run(c.Context, telegramToken, devicePath)
			},
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:        "telegram_token",
					Required:    true,
					EnvVars:     []string{"TELEGRAM_TOKEN"},
					Destination: &telegramToken,
				},
				&cli.StringFlag{
					Name:        "device_path",
					Required:    true,
					Destination: &devicePath,
					EnvVars:     []string{"DEVICE_PATH"},
					DefaultText: "/dev/usb/lp0",
				},
			},
		}
	)

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}

}

type printer struct {
	path  string
	mutex sync.Mutex
}

func newPrinter(devicePath string) *printer {
	return &printer{path: devicePath}
}

func (p *printer) print(printF func(e *escpos.Escpos) error) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	f, err := os.OpenFile(p.path, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer f.Close()

	return printF(escpos.New(f))
}

func (p *printer) printText(t time.Time, username, body string) error {
	fmt.Println(t.String(), username, body)

	return p.print(func(e *escpos.Escpos) error {
		e.Init()
		e.FormfeedN(2)
		e.Write(fmt.Sprintf("Message from: %s\n", username))
		e.Write(fmt.Sprintf("Received at: %s\n", t.Format("2. January 2006, 15:04")))
		e.FormfeedN(2)
		// e.SetAlign("center")
		e.Write(body)
		e.FormfeedN(3)
		e.Cut()
		e.End()

		return nil
	})
}

func run(ctx context.Context, telegramToken, devicePath string) error {
	p := newPrinter(devicePath)

	// Just a quick connection check
	if err := p.print(func(e *escpos.Escpos) error {
		return nil
	}); err != nil {
		return err
	}

	authorizedUsers.Set("elwin", "1243")
	authorizedUsers.Set("pablo", "1234")

	shutdownWebserver := runWebserver(p)

	shutdownTelegram := runTelegram(telegramToken, p)

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGTERM)
	<-c

	fmt.Println("Shutting down...")
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	if err := shutdownWebserver(ctx); err != nil {
		return err
	}

	if err := shutdownTelegram(ctx); err != nil {
		log.Fatal(err)
	}

	return nil
}

func runTelegram(telegramToken string, p *printer) func(context.Context) error {
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

		body := strings.Split(update.Message.Text, " ")
		if update.Message.From.ID == 283013051 && len(body) == 3 && body[0] == "/register" {
			username, password := body[1], body[2]
			authorizedUsers.Set(username, password)
			msg = fmt.Sprintf("Cool, registered %s", username)
		} else if len(update.Message.Text) > 1000 {
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
		} else if err := p.printText(update.Message.Time(), update.Message.From.UserName, update.Message.Text); err != nil {
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

	return func(ctx context.Context) error {
		bot.StopReceivingUpdates()

		return nil
	}
}

func runWebserver(p *printer) func(context.Context) error {
	e := echo.New()
	e.Use(middleware.BasicAuth(func(username string, password string, c echo.Context) (bool, error) {
		return true, nil

		expectedPassword := authorizedUsers.Get(username)
		if !expectedPassword.Exists() {
			return false, nil
		}

		return subtle.ConstantTimeCompare([]byte(expectedPassword.Get()), []byte(password)) == 1, nil
	}))

	e.POST("/print/text", func(c echo.Context) error {
		username, _, _ := c.Request().BasicAuth()

		params := struct {
			Description string `query:"description" header:"description" form:"description" json:"description" xml:"description"`
		}{}

		if err := c.Bind(&params); err != nil {
			return err
		}

		if params.Description == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "Please provide the description value.")
		}

		if err := p.printText(time.Now(), username, params.Description); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err)
		}

		return c.String(http.StatusOK, "Message successfully printed.")
	})

	go func() {
		if err := e.Start(":3030"); err != nil {
			log.Fatal(err)
		}
	}()

	return func(ctx context.Context) error {
		return e.Shutdown(ctx)
	}
}
