# awsdo

## Usage

``` console
$ AWS_PROFILE=myaws awsdo -- aws s3 ls
Enter MFA token code: 123456
2019-12-15 11:00:19 bucket-foo
2020-10-22 12:29:19 bucket-bar
[...]
```

## Required IAM permission

- `iam:ListVirtualMFADevices`
- `sts:GetSessionToken`
