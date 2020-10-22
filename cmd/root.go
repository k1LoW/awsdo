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

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/k1LoW/awsdo/token"
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

		creds, err := token.GetCredentials(ctx, sess)
		if err != nil {
			cmd.PrintErrln(err)
			os.Exit(1)
		}
		if creds != nil {
			envs = append(envs, fmt.Sprintf("AWS_ACCESS_KEY_ID=%s", creds.AccessKeyId))
			envs = append(envs, fmt.Sprintf("AWS_SECRET_ACCESS_KEY=%s", creds.SecretAccessKey))
			envs = append(envs, fmt.Sprintf("AWS_SESSION_TOKEN=%s", creds.SessionToken))
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
