# awsdo

`awsdo` is a tool to do anything using AWS temporary credentials.

## Usage

`awsdo` does anything with temporary credentials generated using `aws sts get-session-token` and `aws sts assume-role`.

### As command wrapper

``` console
$ AWS_PROFILE=myaws awsdo -- terraform apply
Enter MFA code for arn:aws:iam::111111111111:mfa/k1low: 123456
[...]
```

### As env exporter

When `awsdo` is executed with no arguments, `awsdo` outputs shell script to export AWS credentials environment variables like [`aswrap`](https://github.com/fujiwara/aswrap).

``` console
$ AWS_PROFILE=myaws awsdo
Enter MFA code for arn:aws:iam::111111111111:mfa/k1low: 123456
export AWS_REGION=ap-northeast-1
export AWS_ACCESS_KEY_ID=XXXXXXXXXXXXXXXX
export AWS_SECRET_ACCESS_KEY=vl/Zv5hGxdy1DPh7IfpYwP/YKU8J6645...
export AWS_SESSION_TOKEN=FwoGZXIYXdGUaFij9VStcW9fcbuKCKGAWjLxF/3hXgGSoemniFV...
```

If you want to set credentials in a current shell by `eval`, you can use `--token-code` to set the MFA token code.

``` console
$ eval "$(awsdo --profile myaws --token-code 123456)"
```

### As AWS management console login supporter

Login to the AWS management console from a terminal using generaged login link by `awsdo`.

``` console
$ AWS_PROFILE=myaws awsdo --login
```

## Required IAM permissions

- `iam:ListMFADevices`
- `sts:AssumeRole`
- `sts:GetSessionToken`

## How `awsdo` works

- Load `~/.aws/credentials` and `~/.aws/config`.
- Get temporary credentials.
    1. If the section has `aws_session_token`, `awsdo` use that.
        - Find profile ( section of `AWS_PROFILE` or `--profile` ).
        - **Get temporary credentials :key:**.
    2. If `--role-arn` is set, `awsdo` tries to assume role ( `sts:AssumeRole` ).
        - Find profile ( section of `AWS_PROFILE` or `--profile` ).
        - `awsdo` tries to get the MFA device serial number ( `iam:ListMFADevices` ).
        - If `awsdo` get MFA device serial number, it uses multi-factor authentication.
        - **Get temporary credentials :key:**.
    3. If the section has `role_arn`, `awsdo` tries to assume role ( `sts:AssumeRole` ).
        - Find profile ( section of `AWS_PROFILE` or `--profile` ).
        - If the section does not have `mfa_serial`, `awsdo` tries to get the MFA device serial number ( `iam:ListMFADevices` ).
        - If `awsdo` get MFA device serial number, it uses multi-factor authentication.
        - **Get temporary credentials :key:**.
    4. If the section has `sso_session`, `awsdo` tries to SSO login.
        - Find profile ( section of `AWS_PROFILE` or `--profile` ).
        - `awsdo` tries to SSO login like `aws sso login`.
        - **Get temporary credentials :key:**.
    5. Else, `awsdo` try to get session token ( `sts:getSessionToken` ).
        - Find profile ( section of `AWS_PROFILE` or `--profile` ).
        - If the section does not have `mfa_serial`, `awsdo` tries to get the MFA device serial number ( `iam:ListMFADevices` ).
        - If `awsdo` get MFA device serial number, it uses multi-factor authentication.
        - **Get temporary credentials :key:**.
- Set the temporary credentials to environment variables and execute command or export environment variables.
    - `AWS_ACCESS_KEY_ID`
    - `AWS_SECRET_ACCESS_KEY`
    - `AWS_SESSION_TOKEN`
    - `AWS_REGION`

## Example

### Assume Role on CI

``` yaml
name: AWS example workflow
on:
  push
permissions:
  id-token: write
  contents: read
jobs:
  assumeRole:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      - uses: aws-actions/configure-aws-credentials@v1
        with:
          role-to-assume: arn:aws:iam::${{ secrets.AWS_ACCOUNT }}:role/example-role
          aws-region: ${{ secrets.AWS_REGION }}
      - name: Run as ${{ secrets.AWS_ACCOUNT }}
        run: |
          aws sts get-caller-identity
      - name: Setup awsdo
        run: |
          export AWSDO_VERSION=X.X.X
          curl -L https://git.io/dpkg-i-from-url | bash -s -- https://github.com/k1LoW/awsdo/releases/download/v$AWSDO_VERSION/awsdo_$AWSDO_VERSION-1_amd64.deb
      - name: Run as ${{ secrets.AWS_ANOTHER_ACCOUNT }} using awsdo
        run: |
          awsdo --role-arn=arn:aws:iam::${{ secrets.AWS_ANOTHER_ACCOUNT }}:role/another-example-role -- aws sts get-caller-identity
```

## Install

**deb:**

``` console
$ export AWSDO_VERSION=X.X.X
$ curl -o awsdo.deb -L https://github.com/k1LoW/awsdo/releases/download/v$AWSDO_VERSION/awsdo_$AWSDO_VERSION-1_amd64.deb
$ dpkg -i awsdo.deb
```

**RPM:**

``` console
$ export AWSDO_VERSION=X.X.X
$ yum install https://github.com/k1LoW/awsdo/releases/download/v$AWSDO_VERSION/awsdo_$AWSDO_VERSION-1_amd64.rpm
```

**homebrew tap:**

```console
$ brew install k1LoW/tap/awsdo
```

**[aqua](https://aquaproj.github.io/):**

```console
$ aqua g -i k1LoW/awsdo
```

**manually:**

Download binary from [releases page](https://github.com/k1LoW/awsdo/releases)

**go install:**

```console
$ go install github.com/k1LoW/awsdo@latest
```

## References

- [aswrap](https://github.com/fujiwara/aswrap) - AWS assume role credential wrapper.
- [aws-vault](https://github.com/99designs/aws-vault) - A vault for securely storing and accessing AWS credentials in development environments.
- [aws-sso-go](https://github.com/mrtc0/aws-sso-go) - A utility tool that allows credentials to be saved in 1Password even in an AWS SSO environment.
