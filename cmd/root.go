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

	"github.com/k1LoW/awsdo/token"
	"github.com/k1LoW/awsdo/version"
	"github.com/mattn/go-isatty"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
)

var (
	profile       string
	roleArn       string
	sourceProfile string
	duration      string
	sNum          string
	tokenCode     string
	disableCache  bool
	login         bool
)

var rootCmd = &cobra.Command{
	Use:          "awsdo",
	Short:        "awsdo is a tool to do anything using AWS temporary credentials",
	Long:         `awsdo is a tool to do anything using AWS temporary credentials.`,
	Version:      version.Version,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		envs := os.Environ()

		t, err := token.Get(ctx,
			token.Profile(profile),
			token.RoleArn(roleArn),
			token.SourceProfile(sourceProfile),
			token.Duration(duration),
			token.SerialNumber(sNum),
			token.TokenCode(tokenCode),
			token.DisableCache(disableCache))
		if err != nil {
			return err
		}

		// no arguments
		if len(args) == 0 {
			if login {
				link, err := t.GenerageLoginLink()
				if err != nil {
					return err
				}
				if isatty.IsTerminal(os.Stdout.Fd()) {
					if err := browser.OpenURL(link); err != nil {
						return err
					}
				} else {
					cmd.Println(link)
				}
				return nil
			}
			if t.Region != "" {
				cmd.Printf("export AWS_REGION=%s\n", t.Region)
			}
			cmd.Printf("export AWS_ACCESS_KEY_ID=%s\n", t.AccessKeyId)
			cmd.Printf("export AWS_SECRET_ACCESS_KEY=%s\n", t.SecretAccessKey)
			cmd.Printf("export AWS_SESSION_TOKEN=%s\n", t.SessionToken)
			return nil
		}

		if t.Region != "" {
			envs = append(envs, fmt.Sprintf("AWS_REGION=%s", t.Region))
		}
		envs = append(envs, fmt.Sprintf("AWS_ACCESS_KEY_ID=%s", t.AccessKeyId))
		envs = append(envs, fmt.Sprintf("AWS_SECRET_ACCESS_KEY=%s", t.SecretAccessKey))
		envs = append(envs, fmt.Sprintf("AWS_SESSION_TOKEN=%s", t.SessionToken))
		command := args[0]
		c := exec.Command(command, args[1:]...)
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		c.Env = envs
		if err := c.Run(); err != nil {
			return err
		}
		return nil
	},
}

func Execute() {
	rootCmd.SetOut(os.Stdout)
	rootCmd.SetErr(os.Stderr)
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().StringVarP(&profile, "profile", "p", "", "AWS named profile")
	rootCmd.Flags().StringVarP(&roleArn, "role-arn", "r", "", "IAM Role ARN to be assumed")
	rootCmd.Flags().StringVarP(&sourceProfile, "source-profile", "s", "", "AWS named profile that assume role")
	rootCmd.Flags().StringVarP(&duration, "duration", "d", "1hour", "the duration that the credentials should remain valid")
	rootCmd.Flags().StringVarP(&sNum, "serial-number", "n", "", "the identification number of the MFA device")
	rootCmd.Flags().StringVarP(&tokenCode, "token-code", "c", "", "the value provided by the MFA device")
	rootCmd.Flags().BoolVarP(&disableCache, "disable-cache", "", false, "disable the credentials cache")
	rootCmd.Flags().BoolVarP(&login, "login", "", false, "generate login link for the AWS management console")
}
