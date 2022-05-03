package functions

import (
	"context"
	"encoding/json"
	"net/http"
	"os"

	firebase "firebase.google.com/go/v4"
	"github.com/pkg/errors"
	logger "github.com/sirupsen/logrus"

	"firebase.google.com/go/v4/messaging"
	"github.com/joho/godotenv"
	"github.com/muselive-cloud-functions/services"
)

var client *messaging.Client

// One-time initialization: logger and FCM client
func init() {
	logger.SetLevel(logger.DebugLevel)
	logger.SetFormatter(&logger.JSONFormatter{})
	logger.SetOutput(os.Stdout)

	var GO_ENV string
	if GO_ENV = os.Getenv("GO_ENV"); GO_ENV == "development" || GO_ENV == "test" {
		if err := godotenv.Load(".env.dev"); err != nil {
			logger.Fatal(err)
		}
	} else if GO_ENV == "production" {
		if err := godotenv.Load(".env"); err != nil {
			logger.Fatal(err)
		}
	}

	ctx := context.Background()
	conf := &firebase.Config{
		ProjectID: "muselive",
	}
	app, err := firebase.NewApp(ctx, conf)
	if err != nil {
		logger.Fatalf("firebase.Newapp creation failed: %v", err)
	}
	client, err = app.Messaging(ctx)
	logger.Infof("Successfully initialized cloud function")
}

func SendScheduledShowNoti(w http.ResponseWriter, r *http.Request) {
	var msg services.ScheduledShowTaskMessage

	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		logger.Error(errors.WithStack(err))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err := msg.Send(r.Context(), client); err != nil {
		logger.Error(errors.WithStack(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	message := &services.SendTimestampMessage{
		NotiType: "scheduled-show",
		ShowId: msg.ShowID,
	}
	if err := services.UpdateSendTimestamp(message); err != nil {
		logger.Error("Failure to update sendTimestamp for scheduled-show: %v", err)
	}
	w.WriteHeader(http.StatusOK)
}

func SendMyShowNoti(w http.ResponseWriter, r *http.Request) {
	var msg services.MyShowTaskMessage

	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		logger.Error(errors.WithStack(err))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err := msg.Send(r.Context(), client); err != nil {
		logger.Error(errors.WithStack(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	message := &services.SendTimestampMessage{
		NotiType: "my-show",
		ShowId: msg.ShowID,
	}
	if err := services.UpdateSendTimestamp(message); err != nil {
		logger.Error("Failure to update sendTimestamp for my-show: %v", err)
	}
	w.WriteHeader(http.StatusOK)
}
