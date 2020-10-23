# awsdo

AWS temporary credential (aka session token) wrapper.

## Usage

``` ini
# ~/.aws/credentials

[myaws]
aws_access_key_id=XXXXXXXxxxXxXXXXXXXXXXxxxx
aws_secret_access_key=YYyyyYyyYyYYYYYYyyYyYYY
```

### As command wrapper

``` console
$ AWS_PROFILE=myaws awsdo -- aws s3 ls
Enter MFA token code: 123456
2019-12-15 11:00:19 bucket-foo
2020-10-22 12:29:19 bucket-bar
[...]
```

### As env exporter

When awsgo is executed with no arguments, awsgo outputs shell script to export AWS credentials environment variables like [aswrap](https://github.com/fujiwara/aswrap).

``` console
$ export AWS_PROFILE=myaws awsdo
Enter MFA token code: 123456
export AWS_ACCESS_KEY_ID=XXXXXXXXXXXXXXXX
export AWS_SECRET_ACCESS_KEY=vl/Zv5hGxdy1DPh7IfpYwP/YKU8J6645...
export AWS_SESSION_TOKEN=FwoGZXIYXdGUaFij9VStcW9fcbuKCKGAWjLxF/3hXgGSoemniFV...
```

If you want to set credentials in a current shell by `eval`, you can use `--token-code` to set the MFA token code.

``` console
$ eval "$(awsdo --profile myaws --token-code 123456)"
```

## Required IAM permission

- `iam:ListMFADevices`
- `sts:GetSessionToken`

## Install

**deb:**

Use [dpkg-i-from-url](https://github.com/k1LoW/dpkg-i-from-url)

``` console
$ export AWSDO_VERSION=X.X.X
$ curl -L https://git.io/dpkg-i-from-url | bash -s -- https://github.com/k1LoW/awsdo/releases/download/v$AWSDO_VERSION/awsdo_$AWSDO_VERSION-1_amd64.deb
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

**manually:**

Download binary from [releases page](https://github.com/k1LoW/awsdo/releases)

**go get:**

```console
$ go get github.com/k1LoW/awsdo
```

## Reference

- [aswrap](https://github.com/fujiwara/aswrap) - AWS assume role credential wrapper.
