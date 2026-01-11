package controller

func MaskSensitiveString(s string) string {
	if len(s) <= 8 {
		return "***********"
	}
	return s[:4] + "***********" + s[len(s)-4:]
}

func MaskAccessKey(accessKey string) string {
	return MaskSensitiveString(accessKey)
}

func MaskSecretKey(secretKey string) string {
	return "***********"
}
