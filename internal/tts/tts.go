package tts

// VoiceForSpeaker returns the appropriate voice from the map for a given speaker name.
func VoiceForSpeaker(speaker string, voices VoiceMap) Voice {
	switch speaker {
	case "Alex":
		return voices.Alex
	case "Sam":
		return voices.Sam
	default:
		return voices.Alex
	}
}
