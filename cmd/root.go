/*
Copyright © 2020 Ken'ichiro Oyama <k1lowxb@gmail.com>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/
package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/Songmu/prompter"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/k1LoW/awsdo/version"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:     "awsdo",
	Short:   "awsdo",
	Long:    `awsdo.`,
	Args:    cobra.MinimumNArgs(1),
	Version: version.Version,
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()
		sess := session.Must(session.NewSession())
		envs := os.Environ()

		var sNum *string
		iamSvc := iam.New(sess)
		devs, err := iamSvc.ListMFADevicesWithContext(ctx, &iam.ListMFADevicesInput{})
		if err != nil {
			cmd.PrintErrln(err)
			os.Exit(1)
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
				cmd.PrintErrln(err)
				os.Exit(1)
			}
			envs = append(envs, fmt.Sprintf("AWS_ACCESS_KEY_ID=%s", *sessToken.Credentials.AccessKeyId))
			envs = append(envs, fmt.Sprintf("AWS_SECRET_ACCESS_KEY=%s", *sessToken.Credentials.SecretAccessKey))
			envs = append(envs, fmt.Sprintf("AWS_SESSION_TOKEN=%s", *sessToken.Credentials.SessionToken))
		}

		command := args[0]
		c := exec.Command(command, args[1:]...)
		c.Stdout = os.Stderr
		c.Stderr = os.Stderr
		c.Env = envs
		if err := c.Run(); err != nil {
			cmd.PrintErrln(err)
			os.Exit(1)
		}
	},
}

func Execute() {
	rootCmd.SetOut(os.Stdout)
	rootCmd.SetErr(os.Stderr)
	if err := rootCmd.Execute(); err != nil {
		rootCmd.PrintErrln(err)
		os.Exit(1)
	}
}

func init() {}
