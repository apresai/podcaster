package tts

// VoiceForSpeaker returns the appropriate voice from the map for a given speaker name.
func VoiceForSpeaker(speaker string, voices VoiceMap) Voice {
	switch speaker {
	case "Alex":
		return voices.Host1
	case "Sam":
		return voices.Host2
	case "Jordan":
		return voices.Host3
	default:
		return voices.Host1
	}
}
