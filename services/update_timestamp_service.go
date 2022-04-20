package services

import (
	"bytes"
	"encoding/json"
	"os"
	"net/http"
	"github.com/pkg/errors"
	"fmt"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

type (
	SendTimestampMessage struct {
		NotiType string `json:"notiType"`
		ShowId   string `json:"showId"`
	}
)

func computeHmac256(method, body, contentType, secret string) string {
	// Plaintext for HMAC signature is created as below:
	// HTTP Method + hex(SHA256 hash of request body) + content-type
	bodyHash := sha256.New()
	bodyHash.Write([]byte(body))
	bodyHashStr := hex.EncodeToString(bodyHash.Sum(nil))
	message := method + bodyHashStr + contentType
	key := []byte(secret)
	h := hmac.New(sha256.New, key)
	h.Write([]byte(message))
	return hex.EncodeToString(h.Sum(nil))
}

func UpdateSendTimestamp(message *SendTimestampMessage) error {
	url := os.Getenv("WEBHOOK_URL")
	secret := os.Getenv("GCP_CF_WEBHOOK_KEY")
	body, err := json.Marshal(message)
	if err != nil {
		return errors.WithStack(err)
	}

	hmacSignature := computeHmac256("POST", string(body), "application/json", secret)
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	req.Header.Add("X-GCP-CF-HMAC-SHA256", hmacSignature)
	req.Header.Add("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return errors.WithStack(err)
	} else if resp.StatusCode != http.StatusOK {
		errMsg := fmt.Sprintf("Received response status code %d", resp.StatusCode)
		return errors.New(errMsg)
	}
	defer resp.Body.Close()
	return nil
}
