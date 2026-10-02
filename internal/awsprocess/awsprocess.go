// Package awsprocess emits credentials in the JSON format expected by the
// AWS CLI and SDKs "credential_process" setting.
package awsprocess

import (
	"encoding/json"
	"fmt"
	"io"
)

// credentials is the output shape documented at
// https://docs.aws.amazon.com/sdkref/latest/guide/feature-process-credentials.html
//
// SessionToken is omitted for long-term credentials, and Expiration is
// omitted entirely: these are static keys, so AWS SDKs treat them as
// non-refreshable and simply re-invoke this process when needed.
type credentials struct {
	Version         int    `json:"Version"`
	AccessKeyId     string `json:"AccessKeyId"`
	SecretAccessKey string `json:"SecretAccessKey"`
	SessionToken    string `json:"SessionToken,omitempty"`
}

// Write emits the credential_process JSON document for the given credentials.
func Write(w io.Writer, accessKeyID, secretAccessKey, sessionToken string) error {
	b, err := json.Marshal(credentials{
		Version:         1,
		AccessKeyId:     accessKeyID,
		SecretAccessKey: secretAccessKey,
		SessionToken:    sessionToken,
	})
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(b))
	return err
}
