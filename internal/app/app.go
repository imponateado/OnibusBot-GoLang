package app

import (
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/leoteodoro/onibus-bot-go/internal/api"
	"github.com/leoteodoro/onibus-bot-go/internal/bot"
	"github.com/leoteodoro/onibus-bot-go/internal/bot/handlers"
	"github.com/leoteodoro/onibus-bot-go/internal/repository"
	"github.com/leoteodoro/onibus-bot-go/internal/service"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func Run() {
	_ = godotenv.Load()

	token := os.Getenv("BOT_TOKEN")
	if token == "" {
		log.Fatal("BOT_TOKEN não configurado")
	}

	version := "1.1.0-go-layered"

	// 1. Repositories
	subsRepo := repository.NewJSONSubscriptionRepository("subscriptions.json")
	groupsRepo := repository.NewCSVGroupRepository("groups.csv")
	userRepo := repository.NewJSONUserRepository("users.json")
	prefsRepo := repository.NewJSONUserPrefsRepository("user_prefs.json")

	// 2. API Client
	apiClient := api.NewAPIClient()

	// 3. Services
	busService := service.NewBusService(version, apiClient, subsRepo, groupsRepo, prefsRepo)
	userService := service.NewUserService(userRepo)
	broadcastService := service.NewBroadcastService("broadcast.md", userService, prefsRepo)

	// 4. Bot API
	tgBot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Fatal(err)
	}

	// 5. Router & Handlers
	router := bot.NewRouter(tgBot, busService, userService)
	router.Register("/start", &handlers.StartHandler{Service: busService})
	router.Register("oi", &handlers.StartHandler{Service: busService})
	router.Register("/info", &handlers.InfoHandler{Version: version})
	router.Register("/grupos", &handlers.GroupsHandler{Service: busService})
	router.Register("/lowmode", &handlers.LowModeHandler{Service: busService})
	router.Register("/silenciar", &handlers.SilenciarHandler{BroadcastService: broadcastService})
	
	// Prefix-based handlers
	callbackHandler := &handlers.CallbackHandler{Service: busService}
	broadcastOptOutHandler := &handlers.BroadcastOptOutHandler{BroadcastService: broadcastService}
	router.Register("stop_", callbackHandler)
	router.Register("sentido_", callbackHandler)
	router.Register("gsentido_", callbackHandler)
	router.Register("select_group_", callbackHandler)
	router.Register("broadcast_optout", broadcastOptOutHandler)
	router.Register("callback_default", callbackHandler)
	
	// Default search handler
	router.Register("search", &handlers.SearchHandler{Service: busService})

	// 6. Start Bot
	telegramBot, _ := bot.NewTelegramBot(token, busService, router)
	broadcastService.SetNotifier(telegramBot)
	broadcastService.StartLoop()
	telegramBot.Start()
}
