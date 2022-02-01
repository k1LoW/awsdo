package token

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Songmu/prompter"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/k1LoW/awsdo/ini"
	"github.com/k1LoW/duration"
)

type Token struct {
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
	disableCache    bool
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

func DisableCache(disableCache bool) Option {
	return func(c *Config) error {
		c.disableCache = disableCache
		return nil
	}
}

func Get(ctx context.Context, options ...Option) (*Token, error) {
	c := &Config{}
	for _, option := range options {
		if err := option(c); err != nil {
			return nil, err
		}
	}

	i, err := ini.New()
	if err != nil {
		return nil, err
	}

	if !c.disableCache {
		cache, err := getSessionTokenFromCache(c.profile)
		if err == nil {
			return &Token{
				Region:          i.GetKey(c.profile, "region"),
				AccessKeyId:     *cache.AccessKeyId,
				SecretAccessKey: *cache.SecretAccessKey,
				SessionToken:    *cache.SessionToken,
			}, nil
		}
	}
	var t *Token

	if c.sNum == "" {
		c.sNum = i.GetKey(c.profile, "mfa_serial")
	}

	// aws sts assume-role
	roleArn := i.GetKey(c.profile, "role_arn")
	sourceProfile := i.GetKey(c.profile, "source_profile")
	if roleArn != "" {
		if sourceProfile == "" {
			sourceProfile = "default"
		}
		sess := session.Must(session.NewSessionWithOptions(session.Options{Profile: sourceProfile}))
		if c.sNum == "" {
			iamSvc := iam.New(sess)
			devs, _ := iamSvc.ListMFADevicesWithContext(ctx, &iam.ListMFADevicesInput{})
			switch {
			case devs == nil:
				break
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
		stsSvc := sts.New(sess)
		if c.sNum != "" {
			if c.tokenCode == "" {
				c.tokenCode = prompter.Prompt(fmt.Sprintf("Enter MFA code for %s", c.sNum), "")
			}
		}
		sessName := fmt.Sprintf("awsdo-session-%d", time.Now().Unix())
		opt := &sts.AssumeRoleInput{
			RoleSessionName: &sessName,
			DurationSeconds: &c.durationSeconds,
			RoleArn:         &roleArn,
		}
		externalId := i.GetKey(c.profile, "external_id")
		if externalId != "" {
			opt.ExternalId = &externalId
		}
		if c.sNum != "" {
			opt.SerialNumber = &c.sNum
			opt.TokenCode = &c.tokenCode
		}
		assueRoleOut, err := stsSvc.AssumeRoleWithContext(ctx, opt)
		if err != nil {
			return t, err
		}
		if !c.disableCache {
			if err := saveSessionTokenAsCache(c.profile, assueRoleOut.Credentials); err != nil {
				return t, err
			}
		}
		t = &Token{
			Region:          i.GetKey(c.profile, "region"),
			AccessKeyId:     *assueRoleOut.Credentials.AccessKeyId,
			SecretAccessKey: *assueRoleOut.Credentials.SecretAccessKey,
			SessionToken:    *assueRoleOut.Credentials.SessionToken,
		}
		return t, nil
	}

	sess := session.Must(session.NewSessionWithOptions(session.Options{Profile: c.profile}))
	stsSvc := sts.New(sess)

	if c.sNum == "" {
		iamSvc := iam.New(sess)
		devs, _ := iamSvc.ListMFADevicesWithContext(ctx, &iam.ListMFADevicesInput{})
		switch {
		case devs == nil:
			break
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

	if c.sNum != "" {
		if c.tokenCode == "" {
			c.tokenCode = prompter.Prompt(fmt.Sprintf("Enter MFA code for %s", c.sNum), "")
		}
	}

	// aws sts get-session-token
	opt := &sts.GetSessionTokenInput{
		DurationSeconds: &c.durationSeconds,
	}
	if c.sNum != "" {
		opt.SerialNumber = &c.sNum
		opt.TokenCode = &c.tokenCode
	}
	sessToken, err := stsSvc.GetSessionTokenWithContext(ctx, opt)
	if err != nil {
		return t, err
	}
	if err := saveSessionTokenAsCache(c.profile, sessToken.Credentials); err != nil {
		return t, err
	}
	t = &Token{
		Region:          i.GetKey(c.profile, "region"),
		AccessKeyId:     *sessToken.Credentials.AccessKeyId,
		SecretAccessKey: *sessToken.Credentials.SecretAccessKey,
		SessionToken:    *sessToken.Credentials.SessionToken,
	}
	return t, nil
}

func saveSessionTokenAsCache(profile string, creds *sts.Credentials) error {
	if _, err := os.Stat(dataPath()); err != nil {
		if err := os.MkdirAll(dataPath(), 0700); err != nil {
			return err
		}
	}
	out, err := json.Marshal(creds)
	if err != nil {
		return err
	}
	return os.WriteFile(cachePath(profile), out, 0600)
}

func getSessionTokenFromCache(profile string) (*sts.Credentials, error) {
	var creds sts.Credentials
	cache, err := os.ReadFile(cachePath(profile))
	if err != nil {
		return &creds, err
	}
	if err := json.Unmarshal(cache, &creds); err != nil {
		return &creds, err
	}
	if time.Now().After(*creds.Expiration) {
		return &creds, errors.New("session token expired")
	}
	return &creds, nil
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
