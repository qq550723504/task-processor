package preview

func ResolvePlatforms(resultPlatforms, requestPlatforms []string) []string {
	if len(resultPlatforms) > 0 {
		return append([]string(nil), resultPlatforms...)
	}
	if len(requestPlatforms) > 0 {
		return append([]string(nil), requestPlatforms...)
	}
	return nil
}
