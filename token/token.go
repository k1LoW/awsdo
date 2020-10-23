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
	"github.com/k1LoW/duration"
)

type Credentials struct {
	Region          string
	AccessKeyId     string
	SecretAccessKey string
	SessionToken    string
}

type Config struct {
	profile         string
	durationSeconds int64
	sNum            string
	tokenCode       string
}

type Option func(*Config) error

func Profile(profile string) Option {
	return func(c *Config) error {
		if profile == "" {
			profile = os.Getenv("AWS_PROFILE")
		}
		if profile == "" {
			profile = "default"
		}
		c.profile = profile
		return nil
	}
}

func Duration(s string) Option {
	return func(c *Config) error {
		d, err := duration.Parse(s)
		if err != nil {
			return err
		}
		c.durationSeconds = int64(d.Seconds())
		return nil
	}
}

func SerialNumber(sNum string) Option {
	return func(c *Config) error {
		c.sNum = sNum
		return nil
	}
}

func TokenCode(tokenCode string) Option {
	return func(c *Config) error {
		c.tokenCode = tokenCode
		return nil
	}
}

// GetCredentials
func GetCredentials(ctx context.Context, options ...Option) (*Credentials, error) {
	c := &Config{}
	for _, option := range options {
		if err := option(c); err != nil {
			return nil, err
		}
	}

	inis, err := NewInis()
	if err != nil {
		return nil, err
	}

	cache, err := getSessionTokenFromCache(c.profile)
	if err == nil {
		return &Credentials{
			Region:          inis.GetKey(c.profile, "region"),
			AccessKeyId:     *cache.Credentials.AccessKeyId,
			SecretAccessKey: *cache.Credentials.SecretAccessKey,
			SessionToken:    *cache.Credentials.SessionToken,
		}, nil
	}
	sess := session.Must(session.NewSessionWithOptions(session.Options{Profile: c.profile}))
	var creds *Credentials

	if c.sNum == "" {
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
			c.sNum = prompter.Choose("Which MFA devices do you use?", l, l[0])
		case len(devs.MFADevices) == 1:
			c.sNum = *devs.MFADevices[0].SerialNumber
		}
	}

	opt := &sts.GetSessionTokenInput{
		DurationSeconds: &c.durationSeconds,
	}
	if c.sNum != "" {
		opt.SerialNumber = &c.sNum
		if c.tokenCode == "" {
			c.tokenCode = prompter.Prompt("Enter MFA token code", "")
		}
		opt.TokenCode = &c.tokenCode
	}
	stsSvc := sts.New(sess)
	sessToken, err := stsSvc.GetSessionTokenWithContext(ctx, opt)
	if err != nil {
		return creds, err
	}
	if err := saveSessionTokenAsCache(c.profile, sessToken); err != nil {
		return creds, err
	}
	creds = &Credentials{
		Region:          inis.GetKey(c.profile, "region"),
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
		home, _ := os.UserHomeDir()
		p = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(p, "awsgo")
}
