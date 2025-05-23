package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Songmu/prompter"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/ssocreds"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/sso"
	"github.com/aws/aws-sdk-go-v2/service/ssooidc"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/aws-sdk-go-v2/service/sts/types"
	"github.com/k1LoW/awsdo/ini"
	"github.com/k1LoW/duration"
	"github.com/pkg/browser"
)

const federationURL = "https://signin.aws.amazon.com/federation"
const destinationURL = "https://console.aws.amazon.com/"
const clientName = "awsdo"
const clientType = "public"
const grantType = "urn:ietf:params:oauth:grant-type:device_code"

type token struct {
	Region          string `json:"-"`
	AccessKeyID     string `json:"sessionId"`
	SecretAccessKey string `json:"sessionKey"`
	SessionToken    string `json:"sessionToken"`
}

// ref: https://github.com/99designs/aws-vault/blob/39a34315c76ac14143326737fe65def9de2d71ab/cli/login.go#L82
func (t *token) GenerateLoginLink() (string, error) {
	ses, err := json.Marshal(t)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest("GET", federationURL, nil)
	if err != nil {
		return "", err
	}
	q := req.URL.Query()
	q.Add("Action", "getSigninToken")
	q.Add("Session", string(ses))
	req.URL.RawQuery = q.Encode()
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	if res.StatusCode != http.StatusOK {
		_, _ = fmt.Fprintf(os.Stderr, "%v\n", body)
		return "", fmt.Errorf("getSigninToken error: %v", res.Status)
	}

	var resp map[string]string

	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return "", err
	}

	signinToken, ok := resp["SigninToken"]
	if !ok {
		_, _ = fmt.Fprintf(os.Stderr, "%v\n", resp)
		return "", errors.New("parse error")
	}

	return fmt.Sprintf("%s?Action=login&Issuer=aws-vault&Destination=%s&SigninToken=%s", federationURL, url.QueryEscape(destinationURL), url.QueryEscape(signinToken)), nil
}

type Config struct {
	profile         string
	roleArn         string
	sourceProfile   string
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

func RoleArn(roleArn string) Option {
	return func(c *Config) error {
		c.roleArn = roleArn
		return nil
	}
}

func SourceProfile(sourceProfile string) Option {
	return func(c *Config) error {
		c.sourceProfile = sourceProfile
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

// Token returns temporary credentials.
// nolint:revive // unexported-return is acceptable for this API
func Token(ctx context.Context, options ...Option) (*token, error) {
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

	// aws sts assume-role
	roleArn := c.roleArn
	if roleArn == "" {
		roleArn = i.GetKey(c.profile, "role_arn")
	}
	sourceProfile := c.sourceProfile
	if sourceProfile == "" {
		sourceProfile = i.GetKey(c.profile, "source_profile")
	}
	if c.sNum == "" {
		c.sNum = i.GetKey(c.profile, "mfa_serial")
	}
	key := fmt.Sprintf("%s-%s-%s", c.profile, roleArn, sourceProfile)

	if !c.disableCache {
		cache, err := getSessionTokenFromCache(key)
		if err == nil {
			return &token{
				Region:          i.GetKey(c.profile, "region"),
				AccessKeyID:     *cache.AccessKeyId,
				SecretAccessKey: *cache.SecretAccessKey,
				SessionToken:    *cache.SessionToken,
			}, nil
		}
	}
	var t *token

	// Use the temporary credentials listed in ~/.aws
	if i.GetKey(c.profile, "aws_session_token") != "" && i.GetKey(c.profile, "aws_access_key_id") != "" && i.GetKey(c.profile, "aws_secret_access_key") != "" {
		t = &token{
			Region:          i.GetKey(c.profile, "region"),
			AccessKeyID:     i.GetKey(c.profile, "aws_access_key_id"),
			SecretAccessKey: i.GetKey(c.profile, "aws_secret_access_key"),
			SessionToken:    i.GetKey(c.profile, "aws_session_token"),
		}
		return t, nil
	}

	// aws sts assume-role
	if roleArn != "" {
		cfg, err := config.LoadDefaultConfig(ctx, config.WithSharedConfigProfile(sourceProfile))
		if err != nil {
			return t, err
		}

		if c.sNum == "" {
			iamSvc := iam.NewFromConfig(cfg)
			devs, _ := iamSvc.ListMFADevices(ctx, &iam.ListMFADevicesInput{})
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
		stsSvc := sts.NewFromConfig(cfg)
		if c.sNum != "" {
			if c.tokenCode == "" {
				c.tokenCode = prompter.Prompt(fmt.Sprintf("Enter MFA code for %s", c.sNum), "")
			}
		}
		sessName := fmt.Sprintf("awsdo-session-%d", time.Now().Unix())
		// Ensure durationSeconds is within int32 range to prevent overflow
		// #nosec G115 - We've already checked for overflow above
		durationSeconds := int32(c.durationSeconds)
		opt := &sts.AssumeRoleInput{
			RoleSessionName: &sessName,
			DurationSeconds: &durationSeconds,
			RoleArn:         &roleArn,
		}
		externalID := i.GetKey(c.profile, "external_id")
		if externalID != "" {
			opt.ExternalId = &externalID
		}
		if c.sNum != "" {
			opt.SerialNumber = &c.sNum
			opt.TokenCode = &c.tokenCode
		}
		assueRoleOut, err := stsSvc.AssumeRole(ctx, opt)
		if err != nil {
			return t, err
		}
		if !c.disableCache {
			if err := saveSessionTokenAsCache(key, assueRoleOut.Credentials); err != nil {
				return t, err
			}
		}
		t = &token{
			Region:          i.GetKey(c.profile, "region"),
			AccessKeyID:     *assueRoleOut.Credentials.AccessKeyId,
			SecretAccessKey: *assueRoleOut.Credentials.SecretAccessKey,
			SessionToken:    *assueRoleOut.Credentials.SessionToken,
		}
		return t, nil
	}

	cfg, err := config.LoadDefaultConfig(ctx, config.WithSharedConfigProfile(c.profile))
	if err != nil {
		return t, err
	}

	// sso login
	if i.Has(c.profile, "sso_session") {
		if !c.disableCache {
			// Try to get credentials from cache
			creds, err := cfg.Credentials.Retrieve(ctx)
			if err == nil {
				t = &token{
					Region:          i.GetKey(c.profile, "region"),
					AccessKeyID:     creds.AccessKeyID,
					SecretAccessKey: creds.SecretAccessKey,
					SessionToken:    creds.SessionToken,
				}
				return t, nil
			}
		}
		return ssoLogin(ctx, c.profile, i, c.disableCache)
	}

	stsSvc := sts.NewFromConfig(cfg)

	if c.sNum == "" {
		iamSvc := iam.NewFromConfig(cfg)
		devs, _ := iamSvc.ListMFADevices(ctx, &iam.ListMFADevicesInput{})
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
	// Ensure durationSeconds is within int32 range to prevent overflow
	// #nosec G115 - We've already checked for overflow above
	durationSeconds := int32(c.durationSeconds)
	opt := &sts.GetSessionTokenInput{
		DurationSeconds: &durationSeconds,
	}
	if c.sNum != "" {
		opt.SerialNumber = &c.sNum
		opt.TokenCode = &c.tokenCode
	}
	sessToken, err := stsSvc.GetSessionToken(ctx, opt)
	if err != nil {
		return t, err
	}
	if err := saveSessionTokenAsCache(key, sessToken.Credentials); err != nil {
		return t, err
	}
	t = &token{
		Region:          i.GetKey(c.profile, "region"),
		AccessKeyID:     *sessToken.Credentials.AccessKeyId,
		SecretAccessKey: *sessToken.Credentials.SecretAccessKey,
		SessionToken:    *sessToken.Credentials.SessionToken,
	}
	return t, nil
}

type Credentials struct {
	AccessKeyID     *string    `json:"AccessKeyId"`
	SecretAccessKey *string    `json:"SecretAccessKey"`
	SessionToken    *string    `json:"SessionToken"`
	Expiration      *time.Time `json:"Expiration"`
}

type cacheData struct {
	StartURL              string    `json:"startUrl"`
	Region                string    `json:"region"`
	AccessToken           string    `json:"accessToken"`
	ExpiresAt             time.Time `json:"expiresAt"`
	ClientID              string    `json:"clientId"`
	ClientSecret          string    `json:"clientSecret"`
	RegistrationExpiresAt time.Time `json:"registrationExpiresAt"`
}

func ssoLogin(ctx context.Context, profile string, i *ini.Ini, disableCache bool) (*token, error) {
	if !i.Has(profile, "sso_session", "sso_account_id", "sso_role_name", "region") {
		return nil, fmt.Errorf("invalid profile: %s", profile)
	}
	var ssoStartURL, ssoRegion, ssoRegistrationScopes string

	ssoSession := i.GetKey(profile, "sso_session")
	if i.Has(profile, "sso_start_url") {
		ssoStartURL = i.GetKey(profile, "sso_start_url")
	} else {
		ssoStartURL = i.GetKey(fmt.Sprintf("sso-session %s", ssoSession), "sso_start_url")
	}
	if ssoStartURL == "" {
		return nil, fmt.Errorf("invalid profile: %s", profile)
	}

	if i.Has(profile, "sso_region") {
		ssoRegion = i.GetKey(profile, "sso_region")
	} else {
		ssoRegion = i.GetKey(fmt.Sprintf("sso-session %s", ssoSession), "sso_region")
	}
	if ssoRegion == "" {
		return nil, fmt.Errorf("invalid profile: %s", profile)
	}

	if i.Has(profile, "sso_registration_scopes") {
		ssoRegistrationScopes = i.GetKey(profile, "sso_registration_scopes")
	} else {
		ssoRegistrationScopes = i.GetKey(fmt.Sprintf("sso-session %s", ssoSession), "sso_registration_scopes")
	}
	if ssoRegistrationScopes == "" {
		return nil, fmt.Errorf("invalid profile: %s", profile)
	}

	cfg, err := config.LoadDefaultConfig(ctx, config.WithSharedConfigProfile(profile))
	if err != nil {
		return nil, err
	}
	cfg.Region = ssoRegion

	ssooidcClient := ssooidc.NewFromConfig(cfg)
	register, err := ssooidcClient.RegisterClient(ctx, &ssooidc.RegisterClientInput{
		ClientName: aws.String(clientName),
		ClientType: aws.String(clientType),
		Scopes:     []string{ssoRegistrationScopes},
	})
	if err != nil {
		return nil, err
	}

	deviceAuth, err := ssooidcClient.StartDeviceAuthorization(ctx, &ssooidc.StartDeviceAuthorizationInput{
		ClientId:     register.ClientId,
		ClientSecret: register.ClientSecret,
		StartUrl:     aws.String(ssoStartURL),
	})
	if err != nil {
		return nil, err
	}
	url := aws.ToString(deviceAuth.VerificationUriComplete)
	if err := browser.OpenURL(url); err != nil {
		return nil, err
	}
	_, _ = fmt.Fprintf(os.Stderr, "User Code: %s\n", aws.ToString(deviceAuth.UserCode))

	var ssotoken *ssooidc.CreateTokenOutput
	for {
		ssotoken, err = ssooidcClient.CreateToken(context.TODO(), &ssooidc.CreateTokenInput{
			ClientId:     register.ClientId,
			ClientSecret: register.ClientSecret,
			DeviceCode:   deviceAuth.DeviceCode,
			GrantType:    aws.String(grantType),
		})
		if err == nil {
			break
		}
		if !strings.Contains(err.Error(), "AuthorizationPendingException") {
			return nil, err
		}
		time.Sleep(2 * time.Second)
	}
	if ssotoken == nil {
		return nil, errors.New("login failed")
	}

	ssoClient := sso.NewFromConfig(cfg)

	creds, err := ssoClient.GetRoleCredentials(ctx, &sso.GetRoleCredentialsInput{
		AccessToken: ssotoken.AccessToken,
		AccountId:   aws.String(i.GetKey(profile, "sso_account_id")),
		RoleName:    aws.String(i.GetKey(profile, "sso_role_name")),
	})
	if err != nil {
		return nil, err
	}

	if !disableCache {
		cachePath, err := ssocreds.StandardCachedTokenFilepath(ssoSession)
		if err != nil {
			return nil, err
		}
		d := cacheData{
			StartURL:              ssoStartURL,
			Region:                i.GetKey(profile, "region"),
			AccessToken:           *ssotoken.AccessToken,
			ExpiresAt:             time.Unix(time.Now().Unix()+int64(ssotoken.ExpiresIn), 0).UTC(),
			ClientID:              *register.ClientId,
			ClientSecret:          *register.ClientSecret,
			RegistrationExpiresAt: time.Unix(register.ClientSecretExpiresAt, 0).UTC(),
		}
		b, err := json.Marshal(d)
		if err != nil {
			return nil, err
		}
		dir := filepath.Dir(cachePath)
		if err := os.MkdirAll(dir, 0700); err != nil {
			return nil, err
		}
		if err := os.WriteFile(cachePath, b, 0600); err != nil {
			return nil, err
		}
	}

	return &token{
		Region:          i.GetKey(profile, "region"),
		AccessKeyID:     *creds.RoleCredentials.AccessKeyId,
		SecretAccessKey: *creds.RoleCredentials.SecretAccessKey,
		SessionToken:    *creds.RoleCredentials.SessionToken,
	}, nil
}

func saveSessionTokenAsCache(key string, creds *types.Credentials) error {
	if _, err := os.Stat(dataPath()); err != nil {
		if err := os.MkdirAll(dataPath(), 0700); err != nil {
			return err
		}
	}
	// Convert to Credentials for caching
	ccreds := &Credentials{
		AccessKeyID:     creds.AccessKeyId,
		SecretAccessKey: creds.SecretAccessKey,
		SessionToken:    creds.SessionToken,
		Expiration:      creds.Expiration,
	}
	out, err := json.Marshal(ccreds)
	if err != nil {
		return err
	}
	return os.WriteFile(cachePath(key), out, 0600)
}

func getSessionTokenFromCache(key string) (*types.Credentials, error) {
	var ccreds Credentials
	cache, err := os.ReadFile(cachePath(key))
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(cache, &ccreds); err != nil {
		return nil, err
	}
	if time.Now().After(*ccreds.Expiration) {
		return nil, errors.New("session token expired")
	}
	// Convert back to types.Credentials
	return &types.Credentials{
		AccessKeyId:     ccreds.AccessKeyID,
		SecretAccessKey: ccreds.SecretAccessKey,
		SessionToken:    ccreds.SessionToken,
		Expiration:      ccreds.Expiration,
	}, nil
}

func cachePath(key string) string {
	r := strings.NewReplacer(":", "_", "/", "_")
	return filepath.Join(dataPath(), fmt.Sprintf("%s.json", r.Replace(key)))
}

func dataPath() string {
	p := os.Getenv("XDG_DATA_HOME")
	if p == "" {
		home, _ := os.UserHomeDir()
		p = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(p, "awsdo")
}
