package services

import (
	"context"
	"math"
	"sync"

	"github.com/pkg/errors"
	logger "github.com/sirupsen/logrus"

	"firebase.google.com/go/v4/messaging"
	"github.com/muselive-cloud-functions/utils/constants"
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
)

func batchMessageSend(ctx context.Context, message *messaging.MulticastMessage, client *messaging.Client, ch chan<- *messaging.BatchResponse, errorCh chan<- error, wg *sync.WaitGroup) {
	defer wg.Done()

	batchResponse, err := client.SendMulticast(ctx, message)
	if err != nil {
		errorCh <- err
		return
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
	sendChan := make(chan *messaging.BatchResponse)
	errorChan := make(chan error)

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
		go batchMessageSend(ctx, message, client, sendChan, errorChan, &wg)
	}

	wg.Wait()
	numSuccess, numFail := 0, 0
	for i := 0; i < batchSize; i++ {
		select {
		case resp := <-sendChan:
			numSuccess += resp.SuccessCount
			numFail += resp.FailureCount
		case err := <-errorChan:
			logger.Error(errors.WithStack(err))
		}
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
