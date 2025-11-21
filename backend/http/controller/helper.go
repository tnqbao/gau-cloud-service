package controller

func BuildPolicyJSON(role string) []byte {
	switch role {
	case "admin":
		return []byte(`{
	"Version": "2012-10-17",
	"Statement": [
		{
			"Effect": "Allow",
			"Action": ["s3:*"],
			"Resource": ["arn:aws:s3:::dummy-bucket"]
		}
	]
}`)
	case "user":
		return []byte(`{
	"Version": "2012-10-17",
	"Statement": [
		{
			"Effect": "Allow",
			"Action": [
				"s3:GetObject",
				"s3:PutObject",
				"s3:DeleteObject",
				"s3:ListBucket"
			],
			"Resource": ["arn:aws:s3:::dummy-bucket"]
		}
	]
}`)
	case "viewer":
		return []byte(`{
	"Version": "2012-10-17",
	"Statement": [
		{
			"Effect": "Allow",
			"Action": ["s3:GetObject", "s3:ListBucket"],
			"Resource": ["arn:aws:s3:::dummy-bucket"]
		}
	]
}`)
	default:
		return []byte(`{
	"Version": "2012-10-17",
	"Statement": [
		{
			"Effect": "Allow",
			"Action": ["s3:*"],
			"Resource": ["arn:aws:s3:::dummy-bucket"]
		}
	]
}`)
	}
}
