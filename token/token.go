package token

import (
	"context"

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

	if sNum != nil {
		stsSvc := sts.New(sess)
		tokenCode := prompter.Prompt("Enter MFA token code", "")
		sessToken, err := stsSvc.GetSessionTokenWithContext(ctx, &sts.GetSessionTokenInput{
			SerialNumber: sNum,
			TokenCode:    &tokenCode,
		})
		if err != nil {
			return creds, err
		}
		creds = &Credentials{
			AccessKeyId:     *sessToken.Credentials.AccessKeyId,
			SecretAccessKey: *sessToken.Credentials.SecretAccessKey,
			SessionToken:    *sessToken.Credentials.SessionToken,
		}
	}
	return creds, nil
}
