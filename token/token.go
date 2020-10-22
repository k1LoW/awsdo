package token

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/Songmu/prompter"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/sts"
)

type Credentials struct {
	AccessKeyId     string
	SecretAccessKey string
	SessionToken    string
}

// GetCredentials
func GetCredentials(ctx context.Context, profile string) (*Credentials, error) {
	if profile == "" {
		profile = "default"
	}
	cache, err := getSessionTokenFromCache(profile)
	if err == nil {
		return &Credentials{
			AccessKeyId:     *cache.Credentials.AccessKeyId,
			SecretAccessKey: *cache.Credentials.SecretAccessKey,
			SessionToken:    *cache.Credentials.SessionToken,
		}, nil
	}
	sess := session.Must(session.NewSessionWithOptions(session.Options{Profile: profile}))
	var creds *Credentials
	var sNum *string
	iamSvc := iam.New(sess)
	devs, err := iamSvc.ListMFADevicesWithContext(ctx, &iam.ListMFADevicesInput{})
	if err != nil {
		return creds, err
	}

	switch {
	case len(devs.MFADevices) > 1:
		l := []string{}
		for _, d := range devs.MFADevices {
			l = append(l, *d.SerialNumber)
		}
		selected := prompter.Choose("Which MFA devices do you use?", l, l[0])
		sNum = &selected
	case len(devs.MFADevices) == 1:
		sNum = devs.MFADevices[0].SerialNumber
	}

	opt := &sts.GetSessionTokenInput{}
	if sNum != nil {
		tokenCode := prompter.Prompt("Enter MFA token code", "")
		opt.SerialNumber = sNum
		opt.TokenCode = &tokenCode
	}
	stsSvc := sts.New(sess)
	sessToken, err := stsSvc.GetSessionTokenWithContext(ctx, opt)
	if err != nil {
		return creds, err
	}
	if err := saveSessionTokenAsCache(profile, sessToken); err != nil {
		return creds, err
	}
	creds = &Credentials{
		AccessKeyId:     *sessToken.Credentials.AccessKeyId,
		SecretAccessKey: *sessToken.Credentials.SecretAccessKey,
		SessionToken:    *sessToken.Credentials.SessionToken,
	}
	return creds, nil
}

func saveSessionTokenAsCache(profile string, sessToken *sts.GetSessionTokenOutput) error {
	if _, err := os.Stat(dataPath()); err != nil {
		if err := os.MkdirAll(dataPath(), 0700); err != nil {
			return err
		}
	}
	out, err := json.Marshal(sessToken)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(cachePath(profile), out, 0600)
}

func getSessionTokenFromCache(profile string) (*sts.GetSessionTokenOutput, error) {
	var sessToken sts.GetSessionTokenOutput
	cache, err := ioutil.ReadFile(cachePath(profile))
	if err != nil {
		return &sessToken, err
	}
	if err := json.Unmarshal(cache, &sessToken); err != nil {
		return &sessToken, err
	}
	if time.Now().After(*sessToken.Credentials.Expiration) {
		return &sessToken, errors.New("session token expired")
	}
	return &sessToken, nil
}

func cachePath(profile string) string {
	return filepath.Join(dataPath(), fmt.Sprintf("%s.json", profile))
}

func dataPath() string {
	p := os.Getenv("XDG_DATA_HOME")
	if p == "" {
		home := os.Getenv("HOME")
		p = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(p, "awsgo")
}
