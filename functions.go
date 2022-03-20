package functions

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"sync"

	firebase "firebase.google.com/go/v4"
	"github.com/pkg/errors"
	logger "github.com/sirupsen/logrus"

	"firebase.google.com/go/v4/messaging"
	"github.com/muselive-cloud-functions/tools/constants"
)

type (
	Follower struct {
		ID       int64  `json:"id"`
		Alias    string `json:"alias"`
		FCMToken string `json:"fcmToken"`
	}

	ScheduledShowTaskMessage struct {
		PerformerID string      `json:"performerId"`
		Title       string      `json:"title"`
		Body        string      `json:"body"`
		Category    string      `json:"category"`
		Followers   []*Follower `json:"followers"`
		HallID      string      `json:"hallId"`
		ShowID      string      `json:"showId"`
	}

	MyShowTaskMessage struct {
		PerformerID string `json:"performerId"`
		Title       string `json:"title"`
		Body        string `json:"body"`
		Category    string `json:"category"`
		FCMToken    string `json:"fcmToken"`
		HallID      string `json:"hallId"`
		ShowID      string `json:"showId"`
	}
)

const (
	FCM_TOKENS_THRESHOLD = constants.FCM_TOKENS_THRESHOLD
	NOTI_CATEGORY_HALL   = constants.NOTI_CATEGORY_HALL
	WEBHOOK_URL          = constants.WEBHOOK_URL
)

var client *messaging.Client

// One-time initialization: logger and FCM client
func init() {
	logger.SetLevel(logger.DebugLevel)
	logger.SetFormatter(&logger.JSONFormatter{})
	logger.SetOutput(os.Stdout)

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

func batchMessageSend(ctx context.Context, message *messaging.MulticastMessage, client *messaging.Client, ch chan<- *messaging.BatchResponse, wg *sync.WaitGroup) {
	defer wg.Done()

	batchResponse, err := client.SendMulticast(ctx, message)
	if err != nil {
		logger.Error(errors.WithStack(err))
	}
	ch <- batchResponse
}

func (s *ScheduledShowTaskMessage) Send(ctx context.Context, client *messaging.Client) error {
	numFollowers := len(s.Followers)
	registrationTokens := make([]string, numFollowers)
	for i := 0; i < numFollowers; i++ {
		registrationTokens[i] = s.Followers[i].FCMToken
	}

	// Define message
	var data = map[string]string{
		"performerId": s.PerformerID,
		"hallId":      s.HallID,
		"showId":      s.ShowID,
	}

	var wg sync.WaitGroup
	batchSize := int(math.Ceil(float64(numFollowers / FCM_TOKENS_THRESHOLD)))
	sendChan := make(chan *messaging.BatchResponse, batchSize)

	wg.Add(batchSize)
	for i := 0; i < numFollowers; i += FCM_TOKENS_THRESHOLD {
		nextIdx := i + FCM_TOKENS_THRESHOLD
		if nextIdx > numFollowers {
			nextIdx = numFollowers
		}
		batchTokens := registrationTokens[i:nextIdx]
		message := &messaging.MulticastMessage{
			Notification: &messaging.Notification{
				Title: s.Title,
				Body:  s.Body,
			},
			Data: data,
			APNS: &messaging.APNSConfig{
				Payload: &messaging.APNSPayload{
					Aps: &messaging.Aps{
						Category: NOTI_CATEGORY_HALL,
					},
				},
			},
			Tokens: batchTokens,
		}
		go batchMessageSend(ctx, message, client, sendChan, &wg)
	}

	wg.Wait()
	numSuccess, numFail := 0, 0
	for i := 0; i < batchSize; i++ {
		resp := (<-sendChan)
		numSuccess += resp.SuccessCount
		numFail += resp.FailureCount
	}
	logger.Info("Message sending complete!")
	logger.Infof("Success: %d, failure: %d, total %d\n", numSuccess, numFail, numFollowers)

	return nil
}

func (s *MyShowTaskMessage) Send(ctx context.Context, client *messaging.Client) error {
	var data = map[string]string{
		"performerId": s.PerformerID,
		"hallId":      s.HallID,
		"showId":      s.ShowID,
	}
	message := &messaging.Message{
		Notification: &messaging.Notification{
			Title: s.Title,
			Body:  s.Body,
		},
		Data: data,
		APNS: &messaging.APNSConfig{
			Payload: &messaging.APNSPayload{
				Aps: &messaging.Aps{
					Category: NOTI_CATEGORY_HALL,
				},
			},
		},
		Token: s.FCMToken,
	}

	_, err := client.Send(ctx, message)
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func SendToFollowers(w http.ResponseWriter, r *http.Request) {
	var msg ScheduledShowTaskMessage

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

	webhookUrl := fmt.Sprintf("%s/noti/scheduled-show/send-timestamp/%s", WEBHOOK_URL, msg.ShowID)
	// Create custom request w/ nil body to exclude Content-Type field and set Content-Length to 0
	req, err := http.NewRequest("POST", webhookUrl, nil)
	if err != nil {
		logger.Error(errors.WithStack(err))
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		logger.Error(errors.WithStack(err))
	}

	defer res.Body.Close()
	logger.Infof("Got response %d from timestamp update webhook", res.StatusCode)
	w.WriteHeader(http.StatusOK)
}

func SendToMe(w http.ResponseWriter, r *http.Request) {
	var msg MyShowTaskMessage

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

	webhookUrl := fmt.Sprintf("%s/api/webhooks/noti/my-show/send-timestamp/%s", WEBHOOK_URL, msg.ShowID)
	// Create custom request to exclude Content-Type and set Content-Length to 0
	req, err := http.NewRequest("POST", webhookUrl, nil)
	if err != nil {
		logger.Error(errors.WithStack(err))
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		logger.Error(errors.WithStack(err))
	}

	defer res.Body.Close()
	logger.Infof("Got response %d from timestamp update webhook", res.StatusCode)
	w.WriteHeader(http.StatusOK)
}
