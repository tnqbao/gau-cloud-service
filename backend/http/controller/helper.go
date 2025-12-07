package controller

func BuildPolicyJSON(role string) []byte {
	switch role {
	case "admin":
		return []byte(`{
	"Version": "2012-10-17",
	"Statement": [
		{
			"Effect": "Allow",
			"Action": [
				"s3:CreateBucket",
				"s3:DeleteBucket",
				"s3:ListAllMyBuckets",
				"s3:GetBucketLocation",
				"s3:ListBucket"
			],
			"Resource": ["arn:aws:s3:::*"]
		},
		{
			"Effect": "Allow",
			"Action": ["s3:*"],
			"Resource": ["arn:aws:s3:::*/*"]
		}
	]
}`)
	case "user":
		return []byte(`{
	"Version": "2012-10-17",
	"Statement": [
		{
			"Effect": "Allow",
			"Action": ["s3:CreateBucket"],
			"Resource": ["arn:aws:s3:::*"]
		},
		{
			"Effect": "Allow",
			"Action": [
				"s3:ListAllMyBuckets",
				"s3:ListBucket",
				"s3:GetBucketLocation",
				"s3:DeleteBucket"
			],
			"Resource": ["arn:aws:s3:::dummy-bucket"]
		},
		{
			"Effect": "Allow",
			"Action": [
				"s3:GetObject",
				"s3:PutObject",
				"s3:DeleteObject"
			],
			"Resource": ["arn:aws:s3:::dummy-bucket/*"]
		}
	]
}`)
	case "viewer":
		return []byte(`{
	"Version": "2012-10-17",
	"Statement": [
		{
			"Effect": "Allow",
			"Action": [
				"s3:ListAllMyBuckets",
				"s3:ListBucket"
			],
			"Resource": ["arn:aws:s3:::dummy-bucket"]
		},
		{
			"Effect": "Allow",
			"Action": ["s3:GetObject"],
			"Resource": ["arn:aws:s3:::dummy-bucket/*"]
		}
	]
}`)
	default:
		return []byte(`{
	"Version": "2012-10-17",
	"Statement": [
		{
			"Effect": "Allow",
			"Action": [
				"s3:CreateBucket",
				"s3:DeleteBucket",
				"s3:ListAllMyBuckets",
				"s3:GetBucketLocation",
				"s3:ListBucket"
			],
			"Resource": ["arn:aws:s3:::*"]
		},
		{
			"Effect": "Allow",
			"Action": ["s3:*"],
			"Resource": ["arn:aws:s3:::*/*"]
		}
	]
}`)
	}
}
