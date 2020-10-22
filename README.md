# awsdo

AWS temporary credential (aka session token) wrapper.

## Usage

``` console
$ AWS_PROFILE=myaws awsdo -- aws s3 ls
Enter MFA token code: 123456
2019-12-15 11:00:19 bucket-foo
2020-10-22 12:29:19 bucket-bar
[...]
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
